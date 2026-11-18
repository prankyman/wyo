// Package ozon is the fake Ozon: a program in the test's own process that answers the methods of Ozon's Seller API that wyo
// uses (.agents/research/ozon-official-seller-api-check-2026-11-18.md, section 5, cited below as OZC), in the shapes of Ozon's
// own description file (api/ozon/seller-api.json), so that the code which talks to Ozon is tested against something that
// behaves like it and never calls the real one. Ozon has no test environment (OZC section 2): this is the only Ozon a test
// or a development run ever meets.
//
// What it keeps of Ozon's behaviour, each read from the description file or from Ozon's table of roles (see the notes in
// each file):
//   - the two headers every method needs, Client-Id and Api-Key (OZC section 2): a key that was never set up, or a call whose
//     Client-Id is not the key's, is refused as an invalid key;
//   - the key's end date, its roles and the methods of each role, read through /v1/roles (OZC sections 2 and 3), and the
//     states of a key that make Ozon refuse a call: deactivated, limited to some addresses, an offer not signed (OZC
//     sections 2 and 4);
//   - the limit of 50 requests a second for one Client ID, with Ratelimit-Remaining on every answer and Retry-After on the
//     429 (OZC section 4);
//   - the error body {code, message, details} and Ozon's words for its key errors (OZC section 4);
//   - postings (Ozon's «отправления») of the FBS scheme and the FBO scheme with their statuses, several products and
//     quantities, the deadline of assembly, the split of a posting made in the seller's cabinet, and the buyer's name, phone,
//     address and comment in every answer, which the wyo server must drop (OZC section 6).
//
// Ozon's documents do not say which HTTP status goes with each of its key errors, nor the numbers of the error codes (OZC
// section 4, section 10 point 2). The fake chooses them, and says so in a comment at each place: «the fake's choice, not
// Ozon's». A test that depends on a status or a code depends on the fake, not on Ozon.
//
// The wyo server of a test build calls <the fake's address>/api-seller.ozon.ru/<method> (marketplace.Origin): the fake takes
// that first part of the path off, so a call may carry it or not, and so may a control call. The call log therefore records
// the path without it.
//
// The state is per API key: give every test a key of its own, and set it up with /__control/setup before the key can call
// anything.
package ozon

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/prankyman/wyo/internal/clock"
	"github.com/prankyman/wyo/internal/fakes/fakekit"
)

// Host is the host the real Seller API lives on. A test build of the wyo server puts it as the first part of a path.
const Host = "api-seller.ozon.ru"

// Options builds a Server.
type Options struct {
	// Clock gives every moment the fake reports and counts its limit by.
	Clock clock.Clock
	// Delay waits for a queued "slow" fault. Nil means a real timer.
	Delay fakekit.Delayer
}

// Server is one fake Ozon.
type Server struct {
	opts    Options
	kit     *fakekit.Kit[state]
	routes  map[string]func(*call)
	handler http.Handler
}

// New returns a fake Ozon.
func New(o Options) *Server {
	s := &Server{opts: o}

	s.kit = fakekit.New(fakekit.Options[state]{
		Clock:    o.Clock,
		Delay:    o.Delay,
		NewState: func() *state { return newState(o.Clock) },
		Key:      keyOf,
		Dialect: fakekit.Dialect{
			Unauthorized:    func(w http.ResponseWriter) { writeRefusal(w, refusalInvalidKey) },
			TooManyRequests: tooManyRequests,
			ServerError:     func(w http.ResponseWriter) { writeRefusal(w, refusalInternal) },
		},
	})

	s.routes = map[string]func(*call){
		"/v1/roles":                        (*call).roles,
		"/v1/seller/info":                  (*call).sellerInfo,
		"/v2/warehouse/list":               (*call).warehouses,
		"/v4/posting/fbs/unfulfilled/list": (*call).unfulfilledPostings,
		"/v3/posting/fbs/get":              (*call).onePosting,
		"/v3/posting/fbo/list":             (*call).fboPostings,
		"/v3/product/info/list":            (*call).productInfo,
	}

	control := http.NewServeMux()
	s.controlRoutes(control)

	s.handler = withoutHost(s.kit.Wrap(control, http.HandlerFunc(s.serveAPI)))

	return s
}

// Handler is the whole fake: Ozon's methods and the control calls under /__control/.
func (s *Server) Handler() http.Handler { return s.handler }

// Hosts lists the hosts the fake stands for: the first part of a path that sends a call to it.
func (*Server) Hosts() []string { return []string{Host} }

// keyOf is the key of a request: the Api-Key header, which is how Ozon has it. A control call carries the key in the
// Authorization header, which is how fakekit.Control sends it.
func keyOf(r *http.Request) string {
	if key := strings.TrimSpace(r.Header.Get("Api-Key")); key != "" {
		return key
	}

	return fakekit.KeyFromAuthorization(r)
}

// withoutHost takes the host off the front of a path, so that a test addresses the fake as the wyo server of a test build
// does (<address>/api-seller.ozon.ru/v1/roles) or as the real host would be (<address>/v1/roles).
func withoutHost(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rest, found := strings.CutPrefix(r.URL.Path, "/"+Host+"/")
		if !found {
			next.ServeHTTP(w, r)

			return
		}

		moved := r.Clone(r.Context())
		moved.URL.Path = "/" + rest
		moved.URL.RawPath = ""

		next.ServeHTTP(w, moved)
	})
}

// call is one call being answered, with the state of its key locked.
type call struct {
	w  http.ResponseWriter
	r  *http.Request
	st *state
}

func (c *call) json(status int, v any) { fakekit.JSON(c.w, status, v) }

// body reads the JSON body into dst, and answers 400 itself when it cannot be read; an empty body leaves dst as it was. It
// says whether to go on. Ozon is read as it is written: a field the fake does not know is left alone.
func (c *call) body(dst any) bool {
	raw, err := io.ReadAll(c.r.Body)
	if err == nil && len(raw) > 0 {
		err = json.Unmarshal(raw, dst)
	}

	if err != nil {
		badRequest(c.w, "the request body cannot be read: "+err.Error())

		return false
	}

	return true
}

// serveAPI answers a method of Ozon's API. A path that is not a method is refused first, as «Not Found», and a method called
// with anything but POST as «method is not allowed» (the fake's choice of both, not Ozon's); then the key is looked at under
// its state's lock.
func (s *Server) serveAPI(w http.ResponseWriter, r *http.Request) {
	handle, known := s.routes[r.URL.Path]

	switch {
	case !known:
		writeRefusal(w, refusalNotFound)
	case r.Method != http.MethodPost:
		w.Header().Set("Allow", http.MethodPost)
		writeRefusal(w, refusalMethodNotAllowed)
	default:
		s.kit.Use(s.kit.KeyOf(r), func(st *state) { s.guarded(st, w, r, handle) })
	}
}

// guarded is what stands between a call and its method: the key and its Client ID, the limit, then the state and the roles
// of the key. The order is the fake's: Ozon does not say in which order it looks at them.
func (s *Server) guarded(st *state, w http.ResponseWriter, r *http.Request, handle func(*call)) {
	if st.clientID == "" || r.Header.Get("Client-Id") != st.clientID {
		writeRefusal(w, refusalInvalidKey)

		return
	}

	if !st.admit(w) {
		return
	}

	if refusal, refused := st.refusalFor(r.URL.Path); refused {
		writeRefusal(w, refusal)

		return
	}

	handle(&call{w: w, r: r, st: st})
}

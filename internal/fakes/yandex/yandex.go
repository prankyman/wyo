// Package yandex is the fake Яндекс Маркет: a program in the test's own process that answers the fourteen methods of Яндекс
// Маркет's Partner API that the wyo server uses (.agents/research/yandex-market-official-partner-api-check-2026-11-19.md,
// section 5, cited as YMC), in the shapes of Яндекс Маркет's own description files (api/yandex/), so that the code which talks
// to Яндекс Маркет is tested against something that behaves like it and never calls the real one (rule R3).
//
// The five methods: getAuthTokenInfo (POST /v2/auth/token), getCampaigns (GET /v2/campaigns), getPartnerWarehouses (POST
// /v3/businesses/{businessId}/warehouses), getBusinessOrders (POST /v1/businesses/{businessId}/orders) and getOfferMappings
// (POST /v2/businesses/{businessId}/offer-mappings); and the nine of Plan 8 (writing.go, courier.go, shipments.go): the boxes of
// an order with its marking codes (PUT .../orders/{orderId}/boxes), the verdict on the codes (POST .../identifiers/status), a
// box's label (GET .../delivery/shipments/{shipmentId}/boxes/{boxId}/label), the status READY_TO_SHIP (PUT .../status), the
// courier's code (PUT .../verifyEac), and the daily shipment (PUT .../first-mile/shipments, GET and POST .../confirm and GET
// .../act of .../first-mile/shipments/{shipmentId}). The call log keeps the bodies of the calls, so a test can count the writes
// and read what was sent.
//
// What it keeps of Яндекс Маркет's behaviour, each read from the description files or from YMC:
//   - the header Api-Key (YMC section 2): a call without it is refused as 401, and so is a key that was never set up, one set up
//     as «invalid» and one set up as «revoked», each in Яндекс Маркет's words;
//   - the access levels of the key and the levels each method accepts (x-auth-scopes of each description file; access.go), and
//     the cabinet the key belongs to: another cabinet's number is refused as 403;
//   - the limits of each method for one key (YMC section 5), counted on the injected clock, with the three headers of Яндекс
//     Маркет's 420 answer (YMC section 4), and the wall of at most four requests of one key at once (limit.go);
//   - orders (orders.go) with their shipments, the filters of getBusinessOrders (by shop, status, substatus, program, number,
//     creation, shipment and update date, and the test orders by "fake": YMC section 9), pages of at most 50 by a page token,
//     and the buyer's comment, address and the courier's name and telephone in every answer, which the wyo server must drop
//     (buyer.go);
//   - the notifications Яндекс Маркет sends to the address a seller enters in the cabinet (notify.go; YMC section 8), sent from
//     inside the control call that caused them, with faults on command.
//
// Яндекс Маркет's documents do not say which words go with each of its errors; where the fake had to choose, it says so in a
// comment: «the fake's choice, not Яндекс Маркет's».
//
// The wyo server of a test build calls <the fake's address>/api.partner.market.yandex.ru/<method>: the fake takes that first
// part of the path off, so a call may carry it or not, and so may a control call. The call log therefore records the path
// without it.
//
// The state is per Api-Key: give every test a key of its own, and set it up with /__control/setup (control.go) before the key
// can call anything.
package yandex

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/prankyman/wyo/internal/clock"
	"github.com/prankyman/wyo/internal/fakes/fakekit"
)

// Host is the host the real Partner API lives on. A test build of the wyo server puts it as the first part of a path.
const Host = "api.partner.market.yandex.ru"

// Options builds a Server.
type Options struct {
	// Clock gives every moment the fake reports and counts its limits by.
	Clock clock.Clock
	// Delay waits for a queued "slow" fault. Nil means a real timer.
	Delay fakekit.Delayer
	// Client sends the notifications. Nil means a plain client that gives up after 10 seconds.
	Client *http.Client
}

// Server is one fake Яндекс Маркет.
type Server struct {
	opts    Options
	kit     *fakekit.Kit[state]
	client  *http.Client
	handler http.Handler

	parallelMu sync.Mutex
	parallel   map[string]int
}

// New returns a fake Яндекс Маркет.
func New(o Options) *Server {
	s := &Server{opts: o, client: o.Client, parallel: map[string]int{}}

	if s.client == nil {
		s.client = &http.Client{Timeout: 10 * time.Second}
	}

	s.kit = fakekit.New(fakekit.Options[state]{
		Clock:    o.Clock,
		Delay:    o.Delay,
		NewState: func() *state { return newState(o.Clock) },
		Key:      keyOf,
		Dialect: fakekit.Dialect{
			Unauthorized: func(w http.ResponseWriter) { writeRefusal(w, refusalNoKey) },
			// Яндекс Маркет answers a limit with 420, not 429 (YMC section 4), and a retry is told by the moment it ends.
			TooManyRequests: func(w http.ResponseWriter, retryAfter time.Duration) {
				writeLimited(w, faultLimitAmount, o.Clock.Now().Add(retryAfter))
			},
			ServerError: func(w http.ResponseWriter) { writeRefusal(w, refusalInternal) },
		},
	})

	api := http.NewServeMux()
	s.routes(api)

	control := http.NewServeMux()
	s.controlRoutes(control)

	s.handler = withoutHost(s.limitParallel(s.kit.Wrap(control, api)))

	return s
}

// Handler is the whole fake: Яндекс Маркет's methods and the control calls under /__control/.
func (s *Server) Handler() http.Handler { return s.handler }

// Hosts lists the hosts the fake stands for: the first part of a path that sends a call to it.
func (*Server) Hosts() []string { return []string{Host} }

// keyOf is the key of a request: the header Api-Key, which is how Яндекс Маркет has it. A control call carries the key in the
// Authorization header, which is how fakekit.Control sends it.
func keyOf(r *http.Request) string {
	if key := strings.TrimSpace(r.Header.Get("Api-Key")); key != "" {
		return key
	}

	return fakekit.KeyFromAuthorization(r)
}

func isControl(r *http.Request) bool { return strings.HasPrefix(r.URL.Path, fakekit.ControlPrefix) }

// withoutHost takes the host off the front of a path, so that a test addresses the fake as the wyo server of a test build does
// (<address>/api.partner.market.yandex.ru/v2/campaigns) or as the real host would be (<address>/v2/campaigns).
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

// route is one method of Яндекс Маркет's API: the HTTP method it answers, the access levels that open it, its limit, whether it
// names a cabinet in its path, and what it does.
type route struct {
	name     string
	method   string
	scopes   []string
	limit    limitSpec
	business bool
	// campaign says the path names a shop ({campaignId}), which must be one of the key's.
	campaign bool
	handle   func(*call)
}

// call is one call being answered, with the state of its key locked.
type call struct {
	w   http.ResponseWriter
	r   *http.Request
	st  *state
	srv *Server
}

func (c *call) json(status int, v any) { fakekit.JSON(c.w, status, v) }

func (s *Server) routes(mux *http.ServeMux) {
	table := map[string]route{
		"/v2/auth/token": {name: "getAuthTokenInfo", method: http.MethodPost, scopes: scopesAny, limit: limitToken, handle: (*call).tokenInfo},
		"/v2/campaigns":  {name: "getCampaigns", method: http.MethodGet, scopes: scopesAny, limit: limitCampaigns, handle: (*call).campaigns},
		"/v3/businesses/{businessId}/warehouses": {
			name: "getPartnerWarehouses", method: http.MethodPost, scopes: scopesWarehouses, limit: limitWarehouses, business: true, handle: (*call).warehouses,
		},
		"/v1/businesses/{businessId}/orders": {
			name: "getBusinessOrders", method: http.MethodPost, scopes: scopesOrders, limit: limitOrders, business: true, handle: (*call).orders,
		},
		"/v2/businesses/{businessId}/offer-mappings": {
			name: "getOfferMappings", method: http.MethodPost, scopes: scopesOffers, limit: limitOffers, business: true, handle: (*call).offerMappings,
		},
	}

	for pattern, rt := range writingTable() {
		table[pattern] = rt
	}

	for pattern, rt := range table {
		mux.HandleFunc(pattern, s.serve(rt))
	}

	mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) { writeRefusal(w, refusalNotFound) })
}

// serve makes the handler of a method. A method called with the wrong HTTP method is refused first, then the key is looked at
// under its state's lock.
func (s *Server) serve(rt route) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != rt.method {
			w.Header().Set("Allow", rt.method)
			writeRefusal(w, refusalMethodNotAllowed)

			return
		}

		s.kit.Use(s.kit.KeyOf(r), func(st *state) { s.guarded(st, rt, w, r) })
	}
}

// guarded is what stands between a call and its method: the key and its state, the access levels, the limit, then the cabinet.
// The order is the fake's: Яндекс Маркет does not say in which order it looks at them. The limit is counted once the key and
// its levels are accepted, so that a call that is refused for its key or its levels uses nothing, and every answer after that
// (a request that cannot be used, a cabinet that is not the key's) uses one (YMC section 4: the 4xx answers use a call up).
func (s *Server) guarded(st *state, rt route, w http.ResponseWriter, r *http.Request) {
	switch {
	case !st.ready || st.keyState == keyInvalid:
		writeRefusal(w, refusalInvalidKey)

		return
	case st.keyState == keyRevoked:
		writeRefusal(w, refusalRevokedKey)

		return
	case !st.opens(rt.scopes):
		writeRefusal(w, refusalNoScope)

		return
	case !st.admit(w, rt.name, rt.limit):
		return
	}

	if rt.campaign {
		if !s.knownShop(w, r, st) {
			return
		}
	}

	if rt.business {
		id, ok := businessIDOf(r)
		if !ok {
			badRequest(w, "businessId must be a positive number")

			return
		}

		if id != st.business.ID {
			writeRefusal(w, refusalOtherBusiness(id))

			return
		}
	}

	rt.handle(&call{w: w, r: r, st: st, srv: s})
}

// body reads the JSON body into dst, and answers 400 itself when it cannot be read; an empty body leaves dst as it was. It says
// whether to go on. Яндекс Маркет is read as it is written: a field the fake does not know is left alone.
func (c *call) body(dst any) bool { return c.readBody(dst, false) }

// requireBody is body for a method whose description says the body is required: an empty one is refused with 400.
func (c *call) requireBody(dst any) bool { return c.readBody(dst, true) }

func (c *call) readBody(dst any, required bool) bool {
	raw, err := io.ReadAll(c.r.Body)

	switch {
	case err != nil:
		badRequest(c.w, "the request body cannot be read: "+err.Error())

		return false
	case len(raw) == 0 && required:
		badRequest(c.w, "the request body is required")

		return false
	case len(raw) == 0:
		return true
	}

	if err := json.Unmarshal(raw, dst); err != nil {
		badRequest(c.w, "the request body cannot be read: "+err.Error())

		return false
	}

	return true
}

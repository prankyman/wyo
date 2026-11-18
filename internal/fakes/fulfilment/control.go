package fulfilment

import (
	"crypto/rand"
	"encoding/base64"
	"io"
	"net/http"
	"net/url"
	"strconv"

	"github.com/prankyman/wyo/internal/fakes/fakekit"
	"github.com/prankyman/wyo/internal/fulfilment/fcapi"
)

// The control calls of this fake, besides the ones of fakekit. A test plays the fulfilment centre's part through them.
//
//	POST   /__control/setup                       the account: seller_name, access, api_enabled, maintenance, work_stopped,
//	                                              forbidden, marketplaces, warehouses (only the fields sent change)
//	POST   /__control/products                    unknown, known (articles, barcodes, marketplace ids) and catalog
//	POST   /__control/orders                      an order that did not come through the API (via: cabinet, file, ...)
//	POST   /__control/orders/{id}/{action}        reserve, wait-stock, start-picking, pick, pack, reopen, cancel, hand-over; pack takes
//	                                              the units {line, marking_code, parcel}: a unit names the parcel it lies in (1 when
//	                                              left out), and the parcels numbered 1 to the largest named are all closed
//	POST   /__control/handovers/{id}/{action}     request-documents (an optional body {"transport_boxes": N} names the boxes in the
//	                                              handover.requested event), withdraw-request
//	GET    /__control/orders/{id}/parcels/{number}/label   the label file of a parcel as the sender sent it: content_type, scan_code, data
//	GET    /__control/handovers/{id}/documents            the hand-over's documents as the sender sent them: kind, title, content_type, scan_code, data
//	POST   /__control/ping                        a ping event
//	POST   /__control/webhook, DELETE ...         the address (and secret) the events are sent to
//	GET    /__control/deliveries                  every attempt to send a notification
//	POST   /__control/flush                       waits until everything queued has been sent
//	POST   /__control/resend                      sends an event again (an event_id), with the same id and a new timestamp
func (s *Server) controlRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /__control/setup", s.controlSetup)
	mux.HandleFunc("POST /__control/products", s.controlProducts)
	mux.HandleFunc("POST /__control/orders", s.controlOutsideOrder)
	mux.HandleFunc("POST /__control/orders/{id}/{action}", s.controlOrderStep)
	mux.HandleFunc("POST /__control/handovers/{id}/{action}", s.controlHandoverStep)
	mux.HandleFunc("GET /__control/orders/{id}/parcels/{number}/label", s.controlLabelFile)
	mux.HandleFunc("GET /__control/handovers/{id}/documents", s.controlDocumentFiles)
	mux.HandleFunc("POST /__control/ping", s.controlPing)
	mux.HandleFunc("POST /__control/webhook", s.controlWebhook)
	mux.HandleFunc("DELETE /__control/webhook", s.controlWebhookRemove)
	mux.HandleFunc("GET /__control/deliveries", s.controlDeliveries)
	mux.HandleFunc("POST /__control/flush", s.controlFlush)
	mux.HandleFunc("POST /__control/resend", s.controlResend)
}

// controlBody reads the body of a control call; an empty body is fine when optional.
func controlBody(w http.ResponseWriter, r *http.Request, dst any, optional bool) bool {
	raw, err := io.ReadAll(r.Body)
	if err != nil {
		fakekit.ControlError(w, http.StatusBadRequest, "unreadable body")

		return false
	}

	if len(raw) == 0 && optional {
		return true
	}

	if found := decodeStrict(raw, dst); len(found) > 0 {
		fakekit.ControlError(w, http.StatusBadRequest, "the body is not valid: "+found[0].Path+" "+found[0].Code)

		return false
	}

	return true
}

func done(w http.ResponseWriter) {
	fakekit.JSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) controlSetup(w http.ResponseWriter, r *http.Request) {
	var in struct {
		SellerName   *string                     `json:"seller_name"`
		Access       *string                     `json:"access"`
		APIEnabled   *bool                       `json:"api_enabled"`
		Maintenance  *bool                       `json:"maintenance"`
		WorkStopped  *bool                       `json:"work_stopped"`
		Forbidden    *bool                       `json:"forbidden"`
		Marketplaces *[]fcapi.AccountMarketplace `json:"marketplaces"`
		Warehouses   *[]fcapi.Warehouse          `json:"warehouses"`
	}

	if !controlBody(w, r, &in, true) {
		return
	}

	if in.Access != nil && *in.Access != "read_write" && *in.Access != "read_only" {
		fakekit.ControlError(w, http.StatusBadRequest, "access is read_write or read_only")

		return
	}

	if in.Warehouses != nil && len(*in.Warehouses) == 0 {
		fakekit.ControlError(w, http.StatusBadRequest, "a fulfilment centre has at least one warehouse")

		return
	}

	s.with(s.kit.KeyOf(r), func(st *state) {
		set(&st.sellerName, in.SellerName)
		set(&st.apiOff, negate(in.APIEnabled))
		set(&st.maintenance, in.Maintenance)
		set(&st.workStopped, in.WorkStopped)
		set(&st.forbidden, in.Forbidden)

		if in.Access != nil {
			st.readOnly = *in.Access == "read_only"
		}

		if in.Marketplaces != nil {
			st.marketplaces = *in.Marketplaces
		}

		if in.Warehouses != nil {
			st.warehouses = *in.Warehouses
		}

		st.retryHeld()
	})

	done(w)
}

func set[T any](dst *T, value *T) {
	if value != nil {
		*dst = *value
	}
}

func negate(b *bool) *bool {
	if b == nil {
		return nil
	}

	n := !*b

	return &n
}

func (s *Server) controlProducts(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Unknown []string  `json:"unknown"`
		Known   []string  `json:"known"`
		Catalog []product `json:"catalog"`
	}

	if !controlBody(w, r, &in, true) {
		return
	}

	s.with(s.kit.KeyOf(r), func(st *state) {
		for _, id := range in.Unknown {
			st.unknown[id] = true
		}

		for _, id := range in.Known {
			delete(st.unknown, id)
		}

		for _, card := range in.Catalog {
			replaced := false

			for i := range st.catalog {
				if st.catalog[i].Article == card.Article && st.catalog[i].Barcode == card.Barcode {
					st.catalog[i], replaced = card, true
				}
			}

			if !replaced {
				st.catalog = append(st.catalog, card)
			}
		}

		st.retryHeld()
	})

	done(w)
}

func (s *Server) controlOutsideOrder(w http.ResponseWriter, r *http.Request) {
	var in struct {
		fcapi.OrderIn
		Via fcapi.OrderVia `json:"via"`
	}

	if !controlBody(w, r, &in, false) {
		return
	}

	if found := validateOrderIn(&in.OrderIn); len(found) > 0 {
		fakekit.ControlError(w, http.StatusBadRequest, "the order is not valid: "+found[0].Path+" "+found[0].Code)

		return
	}

	switch in.Via {
	case fcapi.Cabinet, fcapi.File, fcapi.Office, fcapi.Marketplace, fcapi.Shop, fcapi.Api:
	default:
		fakekit.ControlError(w, http.StatusBadRequest, "via is api, cabinet, file, office, marketplace or shop")

		return
	}

	var out any

	s.with(s.kit.KeyOf(r), func(st *state) {
		warehouse := st.warehouses[0].Code
		if in.Warehouse != nil {
			warehouse = *in.Warehouse
		}

		out = map[string]any{"order": st.renderOrder(st.accept(in.OrderIn, in.Via, warehouse))}
	})

	fakekit.JSON(w, http.StatusCreated, out)
}

func (s *Server) controlOrderStep(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Units []struct {
			Line        int    `json:"line"`
			MarkingCode string `json:"marking_code"`
			Parcel      int    `json:"parcel"`
		} `json:"units"`
	}

	if !controlBody(w, r, &in, true) {
		return
	}

	units := make([]unit, len(in.Units))
	for i, u := range in.Units {
		units[i] = unit{line: u.Line, code: u.MarkingCode, parcel: u.Parcel}
	}

	var (
		status = http.StatusOK
		out    any
	)

	s.with(s.kit.KeyOf(r), func(st *state) {
		o, found := st.orders[r.PathValue("id")]
		if !found {
			status, out = http.StatusNotFound, map[string]string{"error": "no such order"}

			return
		}

		var err error

		switch r.PathValue("action") {
		case "reserve":
			err = st.reserve(o)
		case "wait-stock":
			err = st.waitForStock(o)
		case "start-picking":
			err = st.startPicking(o)
		case "pick":
			err = st.pick(o)
		case "pack":
			err = st.pack(o, units)
		case "reopen":
			err = st.reopen(o)
		case "cancel":
			err = st.cancelByWarehouse(o)
		case "hand-over":
			err = st.handOver(o)
		default:
			status, out = http.StatusNotFound, map[string]string{"error": "no such action"}

			return
		}

		if err != nil {
			status, out = http.StatusConflict, map[string]string{"error": err.Error()}

			return
		}

		out = map[string]any{"order": st.renderOrder(o)}
	})

	fakekit.JSON(w, status, out)
}

func (s *Server) controlHandoverStep(w http.ResponseWriter, r *http.Request) {
	var (
		status = http.StatusOK
		out    any
		in     struct {
			TransportBoxes *int `json:"transport_boxes"`
		}
	)

	// Only the request for the documents takes a body: the number of transport boxes the fulfilment centre names in it.
	if r.PathValue("action") == "request-documents" {
		if !controlBody(w, r, &in, true) {
			return
		}

		if in.TransportBoxes != nil && *in.TransportBoxes < 1 {
			fakekit.ControlError(w, http.StatusBadRequest, "transport_boxes is a number of at least 1")

			return
		}
	}

	s.with(s.kit.KeyOf(r), func(st *state) {
		h, found := st.handovers[r.PathValue("id")]
		if !found {
			status, out = http.StatusNotFound, map[string]string{"error": "no such hand-over"}

			return
		}

		var err error

		switch r.PathValue("action") {
		case "request-documents":
			err = st.requestDocuments(h, in.TransportBoxes)
		case "withdraw-request":
			err = st.withdrawRequest(h)
		default:
			status, out = http.StatusNotFound, map[string]string{"error": "no such action"}

			return
		}

		if err != nil {
			status, out = http.StatusConflict, map[string]string{"error": err.Error()}

			return
		}

		out = map[string]any{"handover": st.renderHandover(h)}
	})

	fakekit.JSON(w, status, out)
}

func (s *Server) controlPing(w http.ResponseWriter, r *http.Request) {
	s.with(s.kit.KeyOf(r), func(st *state) { st.emit("ping", nil, nil, nil, nil) })
	done(w)
}

func (s *Server) controlWebhook(w http.ResponseWriter, r *http.Request) {
	var in struct {
		URL    string `json:"url"`
		Secret string `json:"secret"`
	}

	if !controlBody(w, r, &in, false) {
		return
	}

	if parsed, err := url.Parse(in.URL); err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		fakekit.ControlError(w, http.StatusBadRequest, "url is an http or https address")

		return
	}

	if in.Secret == "" {
		raw := make([]byte, 32)
		if _, err := rand.Read(raw); err != nil {
			fakekit.ControlError(w, http.StatusInternalServerError, "no random bytes")

			return
		}

		in.Secret = secretPrefix + base64.StdEncoding.EncodeToString(raw)
	}

	if _, err := decodeSecret(in.Secret); err != nil {
		fakekit.ControlError(w, http.StatusBadRequest, err.Error())

		return
	}

	s.with(s.kit.KeyOf(r), func(st *state) { st.hook = &hookConfig{url: in.URL, secret: in.Secret} })
	fakekit.JSON(w, http.StatusOK, map[string]string{"url": in.URL, "secret": in.Secret})
}

func (s *Server) controlWebhookRemove(w http.ResponseWriter, r *http.Request) {
	s.with(s.kit.KeyOf(r), func(st *state) { st.hook = nil })
	done(w)
}

func (s *Server) controlDeliveries(w http.ResponseWriter, r *http.Request) {
	var out []delivery

	s.with(s.kit.KeyOf(r), func(st *state) { out = append([]delivery{}, st.deliveries...) })
	fakekit.JSON(w, http.StatusOK, map[string][]delivery{"deliveries": out})
}

func (s *Server) controlFlush(w http.ResponseWriter, r *http.Request) {
	if err := s.deliverer(s.kit.KeyOf(r)).flush(r.Context()); err != nil {
		fakekit.ControlError(w, http.StatusServiceUnavailable, err.Error())

		return
	}

	done(w)
}

func (s *Server) controlResend(w http.ResponseWriter, r *http.Request) {
	var in struct {
		EventID string `json:"event_id"`
	}

	if !controlBody(w, r, &in, false) {
		return
	}

	key := s.kit.KeyOf(r)

	var (
		sequence int
		hasHook  bool
	)

	s.kit.Use(key, func(st *state) {
		hasHook = st.hook != nil

		for _, e := range st.events {
			if e.id == in.EventID {
				sequence = e.sequence
			}
		}
	})

	switch {
	case sequence == 0:
		fakekit.ControlError(w, http.StatusNotFound, "no such event")
	case !hasHook:
		fakekit.ControlError(w, http.StatusConflict, "no address to send to: POST /__control/webhook first")
	default:
		s.deliverer(key).enqueue(job{sequence: sequence})
		done(w)
	}
}

// controlLabelFile shows a test the label file of a parcel as the sender sent it: the API never gives a file back.
func (s *Server) controlLabelFile(w http.ResponseWriter, r *http.Request) {
	var (
		status = http.StatusOK
		out    any
	)

	s.with(s.kit.KeyOf(r), func(st *state) {
		o, found := st.orders[r.PathValue("id")]

		var p *parcel

		if found {
			number, err := strconv.Atoi(r.PathValue("number"))
			if err == nil {
				p = st.parcelOf(o, number)
			}
		}

		if p == nil || p.label == nil {
			status, out = http.StatusNotFound, map[string]string{"error": "no such parcel, or it has no label"}

			return
		}

		out = map[string]any{"content_type": p.label.contentType, "scan_code": p.label.scanCode, "data": p.label.data}
	})

	fakekit.JSON(w, status, out)
}

// controlDocumentFiles shows a test the documents of a hand-over as the sender sent them.
func (s *Server) controlDocumentFiles(w http.ResponseWriter, r *http.Request) {
	var (
		status = http.StatusOK
		out    any
	)

	s.with(s.kit.KeyOf(r), func(st *state) {
		h, found := st.handovers[r.PathValue("id")]
		if !found {
			status, out = http.StatusNotFound, map[string]string{"error": "no such hand-over"}

			return
		}

		list := make([]map[string]any, len(h.documents))
		for i, d := range h.documents {
			list[i] = map[string]any{"kind": d.kind, "title": d.title, "content_type": d.contentType, "scan_code": d.scanCode, "data": d.data}
		}

		out = map[string]any{"documents": list}
	})

	fakekit.JSON(w, status, out)
}

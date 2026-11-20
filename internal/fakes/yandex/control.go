package yandex

import (
	"errors"
	"fmt"
	"net/http"
	"slices"

	"github.com/prankyman/wyo/internal/fakes/fakekit"
	"github.com/prankyman/wyo/internal/marketplace/yandex/yandexapi"
)

// The control calls of this fake, besides the ones of fakekit (health, reset, calls, faults). Each takes a JSON body, refuses a
// field it does not know, and answers {"status": "ok"} or 400 with the reason. The calls that send a notification (marked
// «notifies») answer {"status": "ok", "notifications": [{"type", "status"}]} as well: what was sent, and what its address
// answered (0 when it was lost or the address could not be reached).
//
//	POST /__control/setup               business {id, name}, scopes [] (the key's access levels, as getAuthTokenInfo reports them),
//	                                    token_name, key_state ("", "invalid" or "revoked"), campaigns [{id, name, placement_type
//	                                    (FBS, FBY, DBS or LAAS; FBS when left out), api_off}], warehouses [{id, name, models
//	                                    [{placement_type (FBS, DBS or EXPRESS), campaign_id}]}], offers [{offer_id, barcodes}]:
//	                                    replaces the parts it names. Until a setup is made, every call of the key is refused as
//	                                    an invalid key.
//	POST /__control/orders              (notifies ORDER_CREATED) a new order: order_id (numbered by the fake when left out),
//	                                    campaign_id, program_type (FBS when left out), fake (a test order), status and substatus
//	                                    (PROCESSING and STARTED when left out), shipment_date (YYYY-MM-DD), shipment_time (HH:MM),
//	                                    warehouse_id (text), items [{offer_id, offer_name, count, required_instance_types}];
//	                                    answers order_id too
//	POST /__control/orders/status       (notifies ORDER_STATUS_UPDATED) order_id, status, substatus
//	POST /__control/orders/cancel       (notifies ORDER_CANCELLED) order_id, substatus (the reason): sets the status CANCELLED
//	POST /__control/orders/shipment     (notifies ORDER_UPDATED, SHIPMENT_DATE_UPDATED) order_id, shipment_date, shipment_time
//	POST /__control/notifications       (notifies) address (the URL to POST to; setting it sends one PING), lose_next,
//	                                    duplicate_next (how many of the next notifications are lost, and how many sent twice; they
//	                                    count the notifications sent after this call), send_ignored (true: sends one CHAT_CREATED,
//	                                    a kind the wyo server ignores)
//	POST /__control/campaigns/api-off   campaign_id, off
//	POST /__control/orders/verdict      order_id, status (OK, IN_PROGRESS, FAILED, INVALID or NOT_ON_VALIDATION: what every marking
//	                                    code of the order is read as; "" gives back the fake's own course: in progress at the
//	                                    first reading, accepted after), substatus (a reason of Яндекс Маркет's, optional)
//	POST /__control/orders/courier-code order_id, code (the code the Express courier names and the order expects), attempts_left
//	                                    (3 when left out), need_update (true: the next check answers NEED_UPDATE)
//	POST /__control/shipments           (answers shipment_id) a daily shipment: warehouse_id (a number), date (YYYY-MM-DD), kind
//	                                    (WITHDRAW or IMPORT), order_ids (they join it, and the order shows its number),
//	                                    cutoff_passed, excluding_order_ids (orders Яндекс Маркет is taking out of it)
//	POST /__control/shipments/update    shipment_id, and any of cutoff_passed, excluding_order_ids, order_ids (replaces the list)
//
// Every change of an order moves its updateDate to the clock's now.
func (s *Server) controlRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/__control/setup", controlCall(s, false, (*state).setUp))
	mux.HandleFunc("/__control/orders", controlCall(s, true, (*state).addOrder))
	mux.HandleFunc("/__control/orders/status", controlCall(s, true, (*state).setStatus))
	mux.HandleFunc("/__control/orders/cancel", controlCall(s, true, (*state).cancelOrder))
	mux.HandleFunc("/__control/orders/shipment", controlCall(s, true, (*state).moveShipment))
	mux.HandleFunc("/__control/notifications", controlCall(s, true, (*state).setNotifications))
	mux.HandleFunc("/__control/campaigns/api-off", controlCall(s, false, (*state).setAPIOff))
	s.writingControlRoutes(mux)
	mux.HandleFunc("/__control/", func(w http.ResponseWriter, _ *http.Request) {
		fakekit.ControlError(w, http.StatusNotFound, "no such control call")
	})
}

// outcome is what a change of the state leaves to be told: the number of an order it made, and the notifications it caused,
// decided while the state was locked and sent after it is not.
type outcome struct {
	orderID    int64
	shipmentID int64
	notices    []notice
}

// controlAnswer is what a control call answers.
type controlAnswer struct {
	Status        string  `json:"status"`
	OrderID       int64   `json:"order_id,omitempty"`
	ShipmentID    int64   `json:"shipment_id,omitempty"`
	Notifications *[]sent `json:"notifications,omitempty"`
}

// controlCall makes the handler of a control call from the change it makes to a key's state: it reads the body into the change's
// request, and runs the change with the state locked. A request or a change that cannot be used is refused with 400. The
// notifications the change caused are sent after the state is unlocked, because the address they go to may call the fake back.
func controlCall[In any](s *Server, notifies bool, change func(*state, In) (outcome, error)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.Header().Set("Allow", http.MethodPost)
			fakekit.ControlError(w, http.StatusMethodNotAllowed, "method not allowed")

			return
		}

		var in In
		if !fakekit.ControlBody(w, r, &in) {
			return
		}

		var (
			out outcome
			err error
		)

		s.kit.Use(s.kit.KeyOf(r), func(st *state) { out, err = change(st, in) })

		if err != nil {
			fakekit.ControlError(w, http.StatusBadRequest, err.Error())

			return
		}

		answer := controlAnswer{Status: "ok", OrderID: out.orderID, ShipmentID: out.shipmentID}

		if notifies {
			delivered := s.deliver(r.Context(), out.notices)
			answer.Notifications = &delivered
		}

		fakekit.JSON(w, http.StatusOK, answer)
	}
}

type setupRequest struct {
	Business   *business    `json:"business"`
	Scopes     *[]string    `json:"scopes"`
	TokenName  *string      `json:"token_name"`
	KeyState   *string      `json:"key_state"`
	Campaigns  *[]campaign  `json:"campaigns"`
	Warehouses *[]warehouse `json:"warehouses"`
	Offers     *[]offer     `json:"offers"`
}

// setUp checks the whole setup before it changes anything, so that a setup that is refused changes nothing.
func (st *state) setUp(in setupRequest) (outcome, error) {
	if err := in.check(); err != nil {
		return outcome{}, err
	}

	st.ready = true

	if in.Business != nil {
		st.business = *in.Business
	}

	if in.Scopes != nil {
		st.scopes = *in.Scopes
	}

	if in.TokenName != nil {
		st.tokenName = *in.TokenName
	}

	if in.KeyState != nil {
		st.keyState = *in.KeyState
	}

	if in.Campaigns != nil {
		st.campaigns = make([]*campaign, len(*in.Campaigns))

		for i, c := range *in.Campaigns {
			if c.PlacementType == "" {
				c.PlacementType = string(yandexapi.PlacementTypeFBS)
			}

			c.seq = i + 1
			st.campaigns[i] = &c
		}
	}

	if in.Warehouses != nil {
		st.warehouses = *in.Warehouses
		for i := range st.warehouses {
			st.warehouses[i].seq = i + 1
		}
	}

	if in.Offers != nil {
		st.offers = *in.Offers
		for i := range st.offers {
			st.offers[i].seq = i + 1
		}
	}

	return outcome{}, nil
}

func (in setupRequest) check() error {
	if in.Business != nil && in.Business.ID <= 0 {
		return errors.New("the business needs a positive id")
	}

	if in.Scopes != nil {
		if len(*in.Scopes) == 0 {
			return errors.New("scopes needs at least one access level")
		}

		for _, s := range *in.Scopes {
			if !yandexapi.ApiKeyScopeType(s).Valid() {
				return fmt.Errorf("%q is not an access level of Яндекс Маркет's schema ApiKeyScopeType", s)
			}
		}
	}

	if in.TokenName != nil && (*in.TokenName == "" || len(*in.TokenName) > 100) {
		return errors.New("token_name is from 1 to 100 characters")
	}

	if in.KeyState != nil && !slices.Contains(keyStates, *in.KeyState) {
		return fmt.Errorf("key_state is one of %q", keyStates)
	}

	return errors.Join(checkCampaigns(deref(in.Campaigns)), checkWarehouses(deref(in.Warehouses)), checkOffers(deref(in.Offers)))
}

func checkCampaigns(campaigns []campaign) error {
	seen := map[int64]bool{}

	for _, c := range campaigns {
		if c.ID <= 0 || seen[c.ID] {
			return fmt.Errorf("the shop number %d is not a positive number used once", c.ID)
		}

		if c.PlacementType != "" && !yandexapi.PlacementType(c.PlacementType).Valid() {
			return fmt.Errorf("placement_type %q of the shop %d is not one of Яндекс Маркет's", c.PlacementType, c.ID)
		}

		seen[c.ID] = true
	}

	return nil
}

func checkWarehouses(warehouses []warehouse) error {
	seen := map[int64]bool{}

	for _, w := range warehouses {
		if w.ID <= 0 || seen[w.ID] {
			return fmt.Errorf("the warehouse number %d is not a positive number used once", w.ID)
		}

		for _, m := range w.Models {
			if !yandexapi.WarehouseProgramType(m.PlacementType).Valid() {
				return fmt.Errorf("placement_type %q of a model of the warehouse %d is not FBS, DBS or EXPRESS", m.PlacementType, w.ID)
			}

			if m.CampaignID < 0 {
				return fmt.Errorf("campaign_id of a model of the warehouse %d cannot be negative", w.ID)
			}
		}

		seen[w.ID] = true
	}

	return nil
}

func checkOffers(offers []offer) error {
	seen := map[string]bool{}

	for _, o := range offers {
		if o.OfferID == "" || seen[o.OfferID] {
			return fmt.Errorf("the article %q is not a text used once", o.OfferID)
		}

		seen[o.OfferID] = true
	}

	return nil
}

type apiOffRequest struct {
	CampaignID int64 `json:"campaign_id"`
	Off        bool  `json:"off"`
}

func (st *state) setAPIOff(in apiOffRequest) (outcome, error) {
	shop := st.campaignOf(in.CampaignID)
	if shop == nil {
		return outcome{}, fmt.Errorf("there is no shop %d", in.CampaignID)
	}

	shop.APIOff = in.Off

	return outcome{}, nil
}

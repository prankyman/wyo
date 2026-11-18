package wildberries

import (
	"fmt"
	"io"
	"net/http"
	"slices"
	"strconv"
	"time"

	"github.com/prankyman/wyo/internal/fakes/fakekit"
)

// The control calls of this fake, besides the ones of fakekit.
//
//	POST /__control/setup                        seller, warehouses, offices, shipping_points, denied (areas the token may not
//	                                             use: marketplace, statistics, common), shipping_required, token_refused
//	                                             ("expired" or "invalid": every call answers 401; "" lifts it)
//	POST /__control/orders                       a new assembly order (article, nm_id, chrt_id, skus, required_meta,
//	                                             optional_meta, cargo_type, and the buyer's address and comment, which
//	                                             a test sends to prove the connector drops them); answers {"id": ...}
//	POST /__control/orders/{id}/wb-status        what Wildberries has done with the order (waiting, sold, declined_by_client, ...)
//	POST /__control/orders/{id}/reshipment       the order must be shipped again
//	POST /__control/orders/{id}/new-sticker      the order's sticker gets a new number, so a label read again differs (a
//	                                             sticker that changes on command)
//	POST /__control/verdicts                     the verdict Wildberries gives an order's marking codes of one kind
//	POST /__control/supplies/{supplyId}/scan     the supply was scanned at the receiving point
//	POST /__control/feed                         rows of the statistics feed (orders of other schemes)
func (s *Server) controlRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /__control/setup", s.controlSetup)
	mux.HandleFunc("POST /__control/orders", s.controlOrder)
	mux.HandleFunc("POST /__control/orders/{id}/wb-status", s.controlWBStatus)
	mux.HandleFunc("POST /__control/orders/{id}/reshipment", s.controlReshipment)
	mux.HandleFunc("POST /__control/orders/{id}/new-sticker", s.controlNewSticker)
	mux.HandleFunc("POST /__control/verdicts", s.controlVerdict)
	mux.HandleFunc("POST /__control/supplies/{supplyId}/scan", s.controlScan)
	mux.HandleFunc("POST /__control/feed", s.controlFeed)
}

// controlBody reads a control call's body, refusing fields it does not know: a test that misspells one learns it at once.
func controlBody(w http.ResponseWriter, r *http.Request, dst any) bool {
	raw, err := io.ReadAll(r.Body)
	if err != nil {
		fakekit.ControlError(w, http.StatusBadRequest, "unreadable body")

		return false
	}

	if len(raw) == 0 {
		return true
	}

	if err := strictJSON(raw, dst); err != nil {
		fakekit.ControlError(w, http.StatusBadRequest, "the body is not valid: "+err.Error())

		return false
	}

	return true
}

func (s *Server) use(r *http.Request, fn func(st *state)) { s.kit.Use(s.kit.KeyOf(r), fn) }

func done(w http.ResponseWriter) { fakekit.JSON(w, http.StatusOK, map[string]string{"status": "ok"}) }

func (s *Server) controlSetup(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Seller           *seller          `json:"seller"`
		Warehouses       *[]warehouse     `json:"warehouses"`
		Offices          *[]office        `json:"offices"`
		ShippingPoints   *[]shippingPoint `json:"shipping_points"`
		Denied           *[]string        `json:"denied"`
		ShippingRequired *bool            `json:"shipping_required"`
		TokenRefused     *string          `json:"token_refused"`
	}

	if !controlBody(w, r, &in) {
		return
	}

	if in.Denied != nil {
		for _, area := range *in.Denied {
			if !slices.Contains([]string{areaMarketplace, areaStatistics, areaCommon}, area) {
				fakekit.ControlError(w, http.StatusBadRequest, "denied names marketplace, statistics or common")

				return
			}
		}
	}

	if in.TokenRefused != nil && *in.TokenRefused != "" && refusedTokenDetails[*in.TokenRefused] == "" {
		fakekit.ControlError(w, http.StatusBadRequest, "token_refused is expired, invalid or empty")

		return
	}

	if (in.Warehouses != nil && len(*in.Warehouses) == 0) || (in.Offices != nil && len(*in.Offices) == 0) {
		fakekit.ControlError(w, http.StatusBadRequest, "a seller has at least one warehouse and Wildberries at least one office")

		return
	}

	s.use(r, func(st *state) {
		if in.Seller != nil {
			merge(&st.seller.Name, in.Seller.Name)
			merge(&st.seller.SID, in.Seller.SID)
			merge(&st.seller.TIN, in.Seller.TIN)
			merge(&st.seller.TradeMark, in.Seller.TradeMark)
			merge(&st.seller.Country, in.Seller.Country)
		}

		if in.Warehouses != nil {
			st.warehouses = *in.Warehouses
		}

		if in.Offices != nil {
			st.offices = *in.Offices
		}

		if in.ShippingPoints != nil {
			st.points = *in.ShippingPoints
		}

		if in.Denied != nil {
			st.denied = map[string]bool{}
			for _, area := range *in.Denied {
				st.denied[area] = true
			}
		}

		if in.ShippingRequired != nil {
			st.shippingNeeded = *in.ShippingRequired
		}

		if in.TokenRefused != nil {
			st.tokenRefused = *in.TokenRefused
		}
	})

	done(w)
}

func merge(dst *string, value string) {
	if value != "" {
		*dst = value
	}
}

func (s *Server) controlOrder(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Article      string   `json:"article"`
		NmID         int      `json:"nm_id"`
		ChrtID       uint64   `json:"chrt_id"`
		Skus         []string `json:"skus"`
		RequiredMeta []string `json:"required_meta"`
		OptionalMeta []string `json:"optional_meta"`
		CargoType    int      `json:"cargo_type"`
		Address      string   `json:"address"`
		Comment      string   `json:"comment"`
	}

	if !controlBody(w, r, &in) {
		return
	}

	if in.CargoType == 0 {
		in.CargoType = 1
	}

	if in.CargoType < 1 || in.CargoType > 3 {
		fakekit.ControlError(w, http.StatusBadRequest, "cargo_type is 1, 2 or 3")

		return
	}

	for _, key := range append(append([]string{}, in.RequiredMeta...), in.OptionalMeta...) {
		if !slices.Contains(metaKeys, key) {
			fakekit.ControlError(w, http.StatusBadRequest, "a kind of marking code is one of imei, uin, gtin, sgtin, expiration, customsDeclaration")

			return
		}
	}

	var id int64

	s.use(r, func(st *state) {
		id = st.newOrderID()

		o := &order{
			id: id, rid: fmt.Sprintf("rid.%d", id), orderUID: fmt.Sprintf("uid-%d", id), article: in.Article, nmID: in.NmID,
			chrtID: in.ChrtID, skus: in.Skus, warehouseID: int(st.warehouses[0].ID), officeID: st.warehouses[0].OfficeID,
			cargoType: in.CargoType, createdAt: st.now(), required: in.RequiredMeta, optional: in.OptionalMeta,
			supplierStatus: statusNew, wbStatus: "waiting", meta: map[string][]string{}, verdicts: map[string]string{},
			buyerAddress: in.Address, buyerComment: in.Comment,
		}

		if o.article == "" {
			o.article = "ART-1"
		}

		if o.nmID == 0 {
			o.nmID = 12345678
		}

		if o.chrtID == 0 {
			o.chrtID = 987654321
		}

		if o.skus == nil {
			o.skus = []string{"4600000000000"}
		}

		st.orders[id] = o
		st.orderList = append(st.orderList, o)
		st.addFeedRow(o)
	})

	fakekit.JSON(w, http.StatusCreated, map[string]int64{"id": id})
}

// cancelledByWildberries are the statuses after which the statistics show the order as cancelled.
var cancelledByWildberries = []string{"canceled", "canceled_by_client", "declined_by_client", "defect", "canceled_by_carrier"}

var wbStatuses = append([]string{
	"waiting", "sorted", "sold", "ready_for_pickup", "accepted_by_carrier", "sent_to_carrier", "postponed_delivery",
}, cancelledByWildberries...)

func (s *Server) orderFromPath(w http.ResponseWriter, r *http.Request, st *state) *order {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)

	var o *order
	if err == nil {
		o = st.orderOf(id)
	}

	if o == nil {
		fakekit.ControlError(w, http.StatusNotFound, "no such order")
	}

	return o
}

func (s *Server) controlWBStatus(w http.ResponseWriter, r *http.Request) {
	var in struct {
		WBStatus string `json:"wb_status"`
	}

	if !controlBody(w, r, &in) {
		return
	}

	s.use(r, func(st *state) {
		o := s.orderFromPath(w, r, st)
		if o == nil {
			return
		}

		if !slices.Contains(wbStatuses, in.WBStatus) {
			fakekit.ControlError(w, http.StatusBadRequest, "wb_status is one of "+fmt.Sprint(wbStatuses))

			return
		}

		o.wbStatus = in.WBStatus
		st.touchFeed(o, slices.Contains(cancelledByWildberries, in.WBStatus))
		done(w)
	})
}

func (s *Server) controlReshipment(w http.ResponseWriter, r *http.Request) {
	s.use(r, func(st *state) {
		o := s.orderFromPath(w, r, st)
		if o == nil {
			return
		}

		if o.supplierStatus != statusComplete {
			fakekit.ControlError(w, http.StatusConflict, "only an order in delivery needs to be shipped again")

			return
		}

		o.reshipment = true
		done(w)
	})
}

func (s *Server) controlNewSticker(w http.ResponseWriter, r *http.Request) {
	s.use(r, func(st *state) {
		o := s.orderFromPath(w, r, st)
		if o == nil {
			return
		}

		st.nextSticker++
		o.stickerID = st.nextSticker
		done(w)
	})
}

func (s *Server) controlVerdict(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Order    int64  `json:"order"`
		Key      string `json:"key"`
		Decision string `json:"decision"`
	}

	if !controlBody(w, r, &in) {
		return
	}

	s.use(r, func(st *state) {
		o := st.orderOf(in.Order)

		switch {
		case o == nil:
			fakekit.ControlError(w, http.StatusNotFound, "no such order")
		case in.Decision == "":
			fakekit.ControlError(w, http.StatusBadRequest, "a decision is required")
		case !o.takes(in.Key):
			fakekit.ControlError(w, http.StatusConflict, "the order does not take that kind of marking code")
		default:
			o.verdicts[in.Key] = in.Decision
			done(w)
		}
	})
}

func (s *Server) controlScan(w http.ResponseWriter, r *http.Request) {
	s.use(r, func(st *state) {
		sp := st.supplyOf(r.PathValue("supplyId"))
		if sp == nil {
			fakekit.ControlError(w, http.StatusNotFound, "no such supply")

			return
		}

		now := st.now()
		sp.scanDt = &now
		done(w)
	})
}

func (s *Server) controlFeed(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Rows []struct {
			Srid            string     `json:"srid"`
			GNumber         string     `json:"g_number"`
			Date            *time.Time `json:"date"`
			LastChangeDate  *time.Time `json:"last_change_date"`
			WarehouseName   string     `json:"warehouse_name"`
			WarehouseType   string     `json:"warehouse_type"`
			SupplierArticle string     `json:"supplier_article"`
			NmID            int        `json:"nm_id"`
			Barcode         string     `json:"barcode"`
			IsCancel        bool       `json:"is_cancel"`
			RegionName      string     `json:"region_name"`
		} `json:"rows"`
	}

	if !controlBody(w, r, &in) {
		return
	}

	for _, row := range in.Rows {
		if row.Srid == "" || (row.WarehouseType != "" && row.WarehouseType != warehouseOfSeller && row.WarehouseType != warehouseOfWB) {
			fakekit.ControlError(w, http.StatusBadRequest, "a row has an srid, and a warehouse_type of «Склад WB» or «Склад продавца»")

			return
		}
	}

	s.use(r, func(st *state) {
		for _, row := range in.Rows {
			added := &feedRow{
				srid: row.Srid, gNumber: row.GNumber, date: st.now(), lastChange: st.now(), warehouseName: row.WarehouseName,
				warehouseType: row.WarehouseType, article: row.SupplierArticle, nmID: row.NmID, barcode: row.Barcode,
				cancelled: row.IsCancel, cancelDate: st.now(), region: row.RegionName,
			}

			if row.Date != nil {
				added.date = row.Date.UTC()
			}

			if row.LastChangeDate != nil {
				added.lastChange = row.LastChangeDate.UTC()
			}

			if added.warehouseType == "" {
				added.warehouseType = warehouseOfWB
			}

			st.feed = append(st.feed, added)
		}
	})

	done(w)
}

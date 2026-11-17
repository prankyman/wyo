package wildberries

import (
	"net/http"
	"time"
)

// GET /ping: «Метод проверяет, успешно ли запрос доходит до WB API, валидность токена ... и совпадает ли категория токена и
// сервис». The answer is {"Status":"OK","TS":"<time>"}.
func (c *call) ping() {
	c.json(http.StatusOK, map[string]any{"Status": "OK", "TS": c.st.now().Format(time.RFC3339)})
}

// GET /api/v1/seller-info.
func (c *call) sellerInfo() {
	s := c.st.seller

	c.json(http.StatusOK, map[string]any{"name": s.Name, "sid": s.SID, "tin": s.TIN, "tradeMark": s.TradeMark, "country": s.Country})
}

// GET /api/v3/warehouses: the seller's warehouses.
func (c *call) warehouses() {
	out := make([]map[string]any, len(c.st.warehouses))

	for i, w := range c.st.warehouses {
		out[i] = map[string]any{
			"id": w.ID, "name": w.Name, "officeId": w.OfficeID, "cargoType": w.CargoType, "deliveryType": w.DeliveryType,
			"isDeleting": false, "isProcessing": false,
		}
	}

	c.json(http.StatusOK, out)
}

// GET /api/v3/offices: Wildberries' offices (the warehouse a seller warehouse is tied to).
func (c *call) offices() {
	out := make([]map[string]any, len(c.st.offices))

	for i, o := range c.st.offices {
		out[i] = map[string]any{
			"id": o.ID, "name": o.Name, "city": o.City, "address": o.Address, "federalDistrict": o.FederalDistrict,
			"latitude": o.Latitude, "longitude": o.Longitude, "cargoType": 1, "deliveryType": 1, "selected": true,
		}
	}

	c.json(http.StatusOK, out)
}

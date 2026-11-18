package wildberries

import (
	"context"
	"fmt"
	"net/http"
	"slices"

	"github.com/prankyman/wyo/internal/marketplace"
	"github.com/prankyman/wyo/internal/marketplace/wildberries/wbapi/general"
	"github.com/prankyman/wyo/internal/marketplace/wildberries/wbapi/items"
	"github.com/prankyman/wyo/internal/marketplace/wildberries/wbapi/orders"
	"github.com/prankyman/wyo/internal/marketplace/wildberries/wbapi/reports"
)

// The generated clients, always built on the engine's Conn: the adapter never makes a request any other way.

func generalClient(conn marketplace.Conn, origin string) (*general.ClientWithResponses, error) {
	return general.NewClientWithResponses(origin, general.WithHTTPClient(conn.Doer))
}

func itemsClient(conn marketplace.Conn, origin string) (*items.ClientWithResponses, error) {
	return items.NewClientWithResponses(origin, items.WithHTTPClient(conn.Doer))
}

func ordersClient(conn marketplace.Conn, origin string) (*orders.ClientWithResponses, error) {
	return orders.NewClientWithResponses(origin, orders.WithHTTPClient(conn.Doer))
}

func reportsClient(conn marketplace.Conn, origin string) (*reports.ClientWithResponses, error) {
	return reports.NewClientWithResponses(origin, reports.WithHTTPClient(conn.Doer))
}

// pinged is a host /ping is asked on, with the token category it proves.
type pinged struct {
	origin   string
	category string
}

// CheckKey implements marketplace.KeyChecker: /ping on each host the account needs («Метод проверяет … валидность
// токена … и совпадает ли категория токена и сервис», 01-general.yaml), then the seller's information, whose sid must be
// the token's own («Уникальный ID продавца на Wildberries, находящийся в публичном поле токена», WBT §2.1). A test token
// is asked only on the sandbox: the seller's information has no sandbox host (WBC §2), so a test account has no
// seller's name.
func (a *Adapter) CheckKey(ctx context.Context, conn marketplace.Conn, facts marketplace.KeyFacts) (marketplace.CheckedKey, error) {
	hosts := []pinged{{origin: a.marketplaceOrigin(facts.Test), category: categoryMarketplace}}
	if !facts.Test && slices.Contains(facts.Categories, categoryStatistics) {
		hosts = append(hosts, pinged{origin: a.origin(hostStatistics), category: categoryStatistics})
	}

	for _, host := range hosts {
		client, err := generalClient(conn, host.origin)
		if err != nil {
			return marketplace.CheckedKey{}, err
		}

		res, err := client.GetPingWithResponse(ctx)
		if err != nil {
			return marketplace.CheckedKey{}, err
		}

		if res.StatusCode() == http.StatusOK {
			continue
		}

		refusal := Refusal(res.StatusCode(), res.Body, host.category)
		if host.category == categoryStatistics && !refusal.StopsJobs {
			facts.MissingCategories = appendOnce(facts.MissingCategories, categoryStatistics)

			continue
		}

		return marketplace.CheckedKey{}, refusal
	}

	checked := marketplace.CheckedKey{KeyFacts: facts}
	if facts.Test {
		return checked, nil
	}

	name, err := a.seller(ctx, conn, facts.ProfileID)
	if err != nil {
		return marketplace.CheckedKey{}, err
	}

	checked.SellerName = name

	return checked, nil
}

// seller reads the seller's name and refuses a token whose profile is not the seller's.
func (a *Adapter) seller(ctx context.Context, conn marketplace.Conn, profileID string) (string, error) {
	client, err := generalClient(conn, a.origin(hostCommon))
	if err != nil {
		return "", err
	}

	res, err := client.GetV1SellerInfoWithResponse(ctx)
	if err != nil {
		return "", err
	}

	if res.StatusCode() != http.StatusOK || res.JSON200 == nil {
		return "", Refusal(res.StatusCode(), res.Body, "")
	}

	if value(res.JSON200.Sid) != profileID {
		return "", &marketplace.Refusal{Code: codeProfileMismatch, Field: fieldPath, Status: res.StatusCode()}
	}

	return value(res.JSON200.Name), nil
}

// Warehouses implements marketplace.WarehouseReader (GET /api/v3/warehouses, 02-items.yaml).
func (a *Adapter) Warehouses(ctx context.Context, conn marketplace.Conn) ([]marketplace.Warehouse, error) {
	client, err := itemsClient(conn, a.marketplaceOrigin(conn.Test))
	if err != nil {
		return nil, err
	}

	res, err := client.GetV3WarehousesWithResponse(ctx)
	if err != nil {
		return nil, err
	}

	if res.StatusCode() != http.StatusOK || res.JSON200 == nil {
		return nil, Refusal(res.StatusCode(), res.Body, categoryMarketplace)
	}

	list := make([]marketplace.Warehouse, 0, len(*res.JSON200))
	for _, w := range *res.JSON200 {
		list = append(list, marketplace.Warehouse{Number: number(w.Id), Name: value(w.Name)})
	}

	return list, nil
}

func value[T any](p *T) T {
	var zero T
	if p == nil {
		return zero
	}

	return *p
}

// number writes a marketplace's number as text: the connector never counts with it.
func number[T int | int64 | uint64](p *T) string {
	if p == nil {
		return ""
	}

	return fmt.Sprint(*p)
}

func appendOnce(list []string, code string) []string {
	if slices.Contains(list, code) {
		return list
	}

	return append(list, code)
}

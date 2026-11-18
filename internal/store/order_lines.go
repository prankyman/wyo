package store

import (
	"context"

	"github.com/prankyman/wyo/internal/store/db"
)

// insertLines adds an order's lines, numbered from 1 in the order given; a quantity under 1 is kept as 1.
func (q *Queries) insertLines(ctx context.Context, orderID int64, lines []OrderLine) error {
	for i, l := range lines {
		err := q.q.InsertOrderLine(ctx, db.InsertOrderLineParams{
			OrderID: orderID, Line: int64(i + 1), ProductID: l.ProductID, Article: l.Article, Name: l.Name, Barcodes: words(l.Barcodes),
			Kind: l.Kind, Brand: l.Brand, Size: l.Size, Quantity: int64(max(1, l.Quantity)),
		})
		if err != nil {
			return kind(err)
		}
	}

	return nil
}

// ReplaceOrderLines writes an order's lines anew: the ones kept are deleted and the ones given added.
func (q *Queries) ReplaceOrderLines(ctx context.Context, orderID int64, lines []OrderLine) error {
	if err := q.q.DeleteOrderLines(ctx, orderID); err != nil {
		return kind(err)
	}

	return q.insertLines(ctx, orderID, lines)
}

// linesOf reads the lines of the orders given, by order.
func (q *Queries) linesOf(ctx context.Context, ids []int64) (map[int64][]OrderLine, error) {
	found := map[int64][]OrderLine{}
	if len(ids) == 0 {
		return found, nil
	}

	rows, err := q.q.ListOrderLinesOf(ctx, ids)
	if err != nil {
		return nil, kind(err)
	}

	for _, r := range rows {
		found[r.OrderID] = append(found[r.OrderID], OrderLine{
			ProductID: r.ProductID, Article: r.Article, Name: r.Name, Barcodes: wordsOf(r.Barcodes), Kind: r.Kind, Brand: r.Brand,
			Size: r.Size, Quantity: int(r.Quantity),
		})
	}

	return found, nil
}

// withLines adds an order's lines to it.
func (q *Queries) withLines(ctx context.Context, o Order) (Order, error) {
	lines, err := q.linesOf(ctx, []int64{o.ID})
	o.Lines = lines[o.ID]

	return o, err
}

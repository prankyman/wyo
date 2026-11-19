package wildberries

import (
	"context"

	"github.com/prankyman/wyo/internal/marketplace"
)

// MakeBatch implements marketplace.HandoverGrouper: a new supply with the name asked for. Wildberries cannot be asked for
// the supply of a request, so the hand-over keeps the number it gets and never calls this a second time for the same
// hand-over; the warehouse and the day of the request are not used by Wildberries.
func (a *Adapter) MakeBatch(ctx context.Context, conn marketplace.Conn, req marketplace.BatchRequest) (string, error) {
	return a.NewBatch(ctx, conn, req.Name)
}

// FillBatch implements marketplace.HandoverGrouper: the orders the supply does not hold yet are put into it. A supply
// of Wildberries only takes orders and gives none back, and the supply of a hand-over is filled only by the wyo server, so
// afterwards it holds exactly the orders given.
func (a *Adapter) FillBatch(ctx context.Context, conn marketplace.Conn, batch string, numbers []string) error {
	state, err := a.Batch(ctx, conn, batch)
	if err != nil {
		return err
	}

	missing := marketplace.Without(numbers, state.Orders)
	if len(missing) == 0 {
		return nil
	}

	return a.PutInBatch(ctx, conn, batch, missing)
}

// Documents implements marketplace.HandoverDocumenter: the closed supply's code, with the supply's number as the text it
// holds, then the sticker of each of its boxes in the order of the supply's boxes, each titled with its box's number.
func (a *Adapter) Documents(ctx context.Context, conn marketplace.Conn, batch string) ([]marketplace.Document, error) {
	code, err := a.BatchCode(ctx, conn, batch)
	if err != nil {
		return nil, err
	}

	documents := []marketplace.Document{{
		Kind: marketplace.DocHandoverCode, Title: batch, ContentType: code.ContentType, Data: code.Data, Code: batch,
	}}

	state, err := a.Batch(ctx, conn, batch)
	if err != nil {
		return nil, err
	}

	if len(state.Boxes) == 0 {
		return documents, nil
	}

	labels, err := a.BoxLabels(ctx, conn, batch, state.Boxes)
	if err != nil {
		return nil, err
	}

	for i, label := range labels {
		documents = append(documents, marketplace.Document{
			Kind: marketplace.DocBoxLabel, Title: state.Boxes[min(i, len(state.Boxes)-1)], ContentType: label.ContentType, Data: label.Data,
		})
	}

	return documents, nil
}

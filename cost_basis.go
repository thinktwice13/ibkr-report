package main

import (
	"math"
)

type Strategy interface {
	Buy(Trade)
	Sell(float64) []Cost
	Holdings(Rater) []Lot
}

type fifo struct {
	data []Cost
}

func (f *fifo) Buy(t Trade) {
	f.data = append(f.data, Cost{
		TradeTx:  t.TradeTx,
		Quantity: t.Quantity,
	})
}

// Sell returns the cost basis fragments of the given quantity of shares
func (f *fifo) Sell(qty float64) []Cost {
	if qty > 0 {
		panic("Sell quantity must be negative")
	}

	if qty == 0 {
		return nil
	}

	var costs []Cost
	// Remove shares from the front of the queue until the quantity is reached or the queue is empty
	for {
		if qty == 0 {
			break
		}

		// Remove the first element from the queue
		if len(f.data) == 0 {
			f.data = nil
			return nil
		}

		// Find next purchase to be sell shares from
		// Determine maximum possible for sale (max out of two)
		// Adjust incoming qty and purchase item
		p := f.next()
		cost := *p
		cost.Quantity = math.Min(p.Quantity, math.Abs(qty))
		p.Quantity -= cost.Quantity
		qty += cost.Quantity
		costs = append(costs, cost)

		// If lot is completely sold, remove it from the list
		// Reset fifo data if all sold
		if p.Quantity == 0 {
			if len(f.data) == 1 {
				f.data = nil
			} else {
				f.data = f.data[1:]
			}
		}
	}
	return costs
}

func (f *fifo) next() *Cost {
	if len(f.data) == 0 {
		return nil
	}
	return &f.data[0]
}

func (f *fifo) Holdings(r Rater) []Lot {
	return lotsFromTrades(f.data, r)
}

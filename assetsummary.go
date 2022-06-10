package main

import (
	"math"
	"sort"
	"time"
)

type AssetYear struct {
	Pl, Taxable, Fees, Dividends, WithholdingTax float64
}

type Asset struct {
	Instrument
	Holdings []Trade
	Summary  map[int]*AssetYear
}

type AssetSummary map[int]*AssetYear

func (s AssetSummary) year(y int) *AssetYear {
	_, ok := s[y]
	if !ok {
		s[y] = new(AssetYear)
	}
	return s[y]
}

type Rater interface {
	Rate(string, int) float64
}

func summarizeAssets(imports []AssetImport, r Rater) []Asset {
	assets := make([]Asset, len(imports))

	for i, ai := range imports {
		sort.Slice(ai.Trades, func(i, j int) bool {
			return ai.Trades[i].Time.Before(ai.Trades[j].Time)
		})

		// sales, fees, active holdings
		sales, fees, holdings := tradeAsset(ai.Trades)
		fromYear := ai.Trades[0].Time.Year()
		toYear := time.Now().Year()
		if len(holdings) == 0 {
			toYear = ai.Trades[len(ai.Trades)-1].Time.Year()
		}
		sum := make(AssetSummary, toYear-fromYear+1)

		// profits
		for _, s := range sales {
			y := sum.year(s.Time.Year())
			for _, c := range s.Basis {
				proceeds := s.Price * c.Quantity * r.Rate(s.Currency, s.Time.Year())
				cost := c.Price * c.Quantity * r.Rate(c.Currency, s.Time.Year())
				y.Pl += proceeds - cost
				if s.Time.After(TaxableDeadline(c.Time)) {
					continue
				}
				y.Taxable += proceeds - cost
			}
		}

		// fees
		for _, f := range fees {
			amt := f.Amount * r.Rate(f.Currency, f.Year)
			sum.year(f.Year).Fees += amt
		}

		// dividends
		for _, d := range ai.Dividends {
			amt := d.Amount * r.Rate(d.Currency, d.Year)
			sum.year(d.Year).Dividends += amt
		}
		// withholding tax
		for _, t := range ai.WithholdingTax {
			amt := t.Amount * r.Rate(t.Currency, t.Year)
			sum.year(t.Year).WithholdingTax += amt
		}

		assets[i] = Asset{
			Instrument: ai.Instrument,
			Holdings:   holdings,
			Summary:    sum,
		}
	}

	return assets
}

type fifo struct {
	data []Trade
}

type Cost struct {
	Time            time.Time
	Currency        string
	Price, Quantity float64
}

type Sale struct {
	Time     time.Time
	Currency string
	Price    float64
	Basis    []Cost
}

func tradeAsset(ts []Trade) ([]Sale, []Transaction, []Trade) {
	fifo := new(fifo)
	fees := make([]Transaction, 0, len(ts))

	var sales []Sale
	for i, t := range ts {
		fees = append(fees, Transaction{
			Currency: t.Currency,
			Amount:   t.Fee,
			Year:     t.Time.Year(),
		})

		if t.Quantity > 0 {
			// Purchase
			fifo.data = append(fifo.data, ts[i])
			continue
		}

		// Sale
		s := Sale{
			Time:     t.Time,
			Currency: t.Currency,
			Price:    t.Price,
		}
		// Calculate costs
		for {
			if t.Quantity == 0 {
				sales = append(sales, s)
				break
			}

			purchase := &fifo.data[0]
			cost := Cost{
				Time:     purchase.Time,
				Currency: purchase.Currency,
				Price:    purchase.Price,
				Quantity: math.Min(purchase.Quantity, math.Abs(t.Quantity)),
			}
			purchase.Quantity -= cost.Quantity
			t.Quantity += cost.Quantity
			s.Basis = append(s.Basis, cost)
			if purchase.Quantity == 0 {
				// TODO Handle error when this is the last purchase, but there are more trade (import error)
				fifo.data = fifo.data[1:]
			}
		}
	}

	return sales, fees, fifo.data
}

func TaxableDeadline(since time.Time) time.Time {
	return since.AddDate(2, 0, 0)
}

func filterFromTo(l []int, min, max int) []int {
	cap := max - min + 1
	if len(l) < cap {
		cap = len(l)
	}
	filtered := make([]int, 0, cap)
	for _, i := range l {
		if i < min || i > max {
			continue
		}

		filtered = append(filtered, i)
	}

	return filtered
}

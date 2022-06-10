package main

import (
	"math"
	"sort"
	"time"
)

type AssetYear struct {
	Pl, Taxable, Fees, Dividends, WithholdingTax float64
	Year                                         int
}

type Asset struct {
	Instrument
	FirstPurchase time.Time
	Holdings      []Trade
	Years         []AssetYear
}

type AssetSummary map[int]*AssetYear

func (s AssetSummary) year(y int) *AssetYear {
	_, ok := s[y]
	if !ok {
		s[y] = &AssetYear{Year: y}
	}
	return s[y]
}

type SummaryReport []Asset

func (s *SummaryReport) Title() string {
	return "Summary"
}

type Rater interface {
	Rate(string, int) float64
}

func summarizeAssets(imports []AssetImport, r Rater) SummaryReport {
	assets := make(SummaryReport, len(imports))

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
				if s.Time.After(taxableDeadline(c.Time)) {
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

		a := Asset{
			Instrument:    ai.Instrument,
			FirstPurchase: ai.Trades[0].Time,
			Holdings:      holdings,
			Years:         make([]AssetYear, 0, len(sum)),
		}

		for _, data := range sum {
			a.Years = append(a.Years, *data)
		}

		sort.Slice(a.Years, func(i, j int) bool {
			return a.Years[i].Year < a.Years[j].Year
		})

		assets[i] = a
	}

	sort.Slice(assets, func(i, j int) bool {
		return assets[i].FirstPurchase.Before(assets[j].FirstPurchase)
	})

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

func taxableDeadline(since time.Time) time.Time {
	return since.AddDate(2, 0, 0)
}

func (s *SummaryReport) WriteTo(rw RowWriter) error {
	srows := make([][]interface{}, 0, len(*s))
	srows = append(srows, []interface{}{
		"Asset",
		"Category",
		"Year",
		"Profit/Loss",
		"Taxable PL",
		"Fees",
		"Dividends",
		"Withholding Tax",
	})

	for _, a := range *s {
		for _, y := range a.Years {
			srows = append(srows, []interface{}{
				a.Symbols,
				a.Category,
				y.Year,
				y.Pl,
				y.Taxable,
				y.Fees,
				y.Dividends,
				y.WithholdingTax,
			})
		}
	}

	err := rw.WriteRows("Summary", srows)
	if err != nil {
		return err
	}
	return nil
}

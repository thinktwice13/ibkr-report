package main

import (
	"fmt"
	"math"
	"sort"
	"time"
)

// AssetYear represents a summary of profits, fees, dividends and withholding tax paid for a sifnle asset in a signle year
type AssetYear struct {
	Pl, Taxable, Fees, Dividends, WithholdingTax float64
	Year                                         int
}

// Asset holds:
// - info on the instrument representing an asset held,
// - currently active holdinggs (FIFO accounting strategy)
// - yearly summary of profts, dividends, fees and withholding tax paid
type Asset struct {
	Instrument
	FirstPurchase *time.Time
	Holdings      []Lot
	Years         []AssetYear
}

type AssetSummary map[int]*AssetYear

// year returns a requested mapped item. Creates new if does not exist
func (s AssetSummary) year(y int) *AssetYear {
	_, ok := s[y]
	if !ok {
		s[y] = &AssetYear{Year: y}
	}
	return s[y]
}

type SummaryReport []Asset

type Rater interface {
	Rate(string, int) float64
}

// summarizeAssets returns a list of summarized asset imports, given a list of imports and a type implementing a Rater if
func summarizeAssets(imports []AssetImport, r Rater) SummaryReport {
	assets := make(SummaryReport, len(imports))

	for i, ai := range imports {
		// Sort imported trades by date
		// Must be sorted for costbasis accounting strategy
		sort.Slice(ai.Trades, func(i, j int) bool {
			return ai.Trades[i].Time.Before(ai.Trades[j].Time)
		})

		// sales, fees, active holdings
		// Calculate summary size for the asset
		sales, fees, holdings := tradeAsset(ai.Trades)

		toYear := time.Now().Year()
		firstYear := toYear
		if len(ai.Trades) != 0 {
			firstYear = ai.Trades[0].Time.Year()
			if len(holdings) == 0 {
				toYear = ai.Trades[len(ai.Trades)-1].Time.Year()
			}
		}
		sum := make(AssetSummary, toYear-firstYear+1)

		// summarize sales
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

		// fees returned from profits accounting
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
			Instrument: ai.Instrument,
			Holdings:   lotsFromTrades(holdings, r),
			Years:      make([]AssetYear, 0, len(sum)),
		}

		if len(ai.Trades) != 0 {
			a.FirstPurchase = &ai.Trades[0].Time
		}

		for _, data := range sum {
			a.Years = append(a.Years, *data)
		}

		assets[i] = a // TODO Insertion sort?
	}

	sortAssets(assets)
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

// tradeAsset calculates sales, fees and active holdings according to costbasis strategy (FIFO)
// TODO use other strategies
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

		// Add to costbasis strategy if a purchase
		if t.Quantity > 0 {
			// Purchase
			fifo.data = append(fifo.data, ts[i]) // TODO strategy specific
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
			// If trade order fulfilled, break out
			if t.Quantity == 0 {
				sales = append(sales, s)
				break
			}

			// Find next purchase to calculate ciostbasis from
			// Deduct sold quantity from purchase used to calculate the cost basis and sale trade being processed
			// Add costs to curreny sale
			if len(fifo.data) == 0 {
				fmt.Errorf("error calculating trades for asset")
				return nil, nil, nil
			}
			purchase := &fifo.data[0] // TODO strategyu specific
			cost := Cost{
				Time:     purchase.Time,
				Currency: purchase.Currency,
				Price:    purchase.Price,
				Quantity: math.Min(purchase.Quantity, math.Abs(t.Quantity)),
			}
			purchase.Quantity -= cost.Quantity
			t.Quantity += cost.Quantity
			s.Basis = append(s.Basis, cost)

			// Evict entire purchase if completely sold
			if purchase.Quantity == 0 {
				// TODO Handle error when this is the last purchase, but there are more trade (import error)
				fifo.data = fifo.data[1:]
			}
		}
	}

	return sales, fees, fifo.data
}

// taxableDeadline determines rge threshold date for profits tax
// Tax introduced 1/1/2016. For any deadlines before, return provided date
// For any taxable events provided after 1/1/2016, calculate deadline at 24 months after provided time
func taxableDeadline(since time.Time) time.Time {
	if since.Before(time.Date(2016, 1, 1, 1, 0, 0, 0, time.UTC)) {
		return since
	}
	return since.AddDate(2, 0, 0)
}

// WriteTo writes report data to RowWriter implementing type
func (s *SummaryReport) WriteTo(rw RowWriter) error {
	srows := make([][]interface{}, 0, len(*s))
	hrows := make([][]interface{}, 0, len(*s))
	// FIXME rows len = summaries len * years len / holdings len

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

	hrows = append(hrows, []interface{}{
		"Asset",
		"Category",
		"Purchased",
		"TaxableUntil",
		"Quantity",
		"Cost",
	})

	for _, a := range *s {
		for _, y := range a.Years {
			srows = append(srows, []interface{}{
				a.Symbols,
				a.Category,
				y.Year,
				RoundDec(y.Pl, 2),
				RoundDec(y.Taxable, 2),
				RoundDec(y.Fees, 2),
				RoundDec(y.Dividends, 2),
				RoundDec(y.WithholdingTax, 2),
			})
		}

		for _, h := range a.Holdings {
			hrows = append(hrows, []interface{}{
				a.Symbols,
				a.Category,
				h.Purchased,
				h.TaxableUntil,
				h.Quantity,
				RoundDec(h.Cost, 2),
			})
		}
	}

	err := rw.WriteRows("Summary", srows)
	if err != nil {
		return err
	}

	err = rw.WriteRows("Holdings", hrows)
	if err != nil {
		return err
	}

	return nil
}

type Lot struct {
	Purchased    time.Time
	TaxableUntil *time.Time
	Cost         float64
	Quantity     float64
}

func lotsFromTrades(ts []Trade, r Rater) []Lot {
	lots := make([]Lot, len(ts))
	for i, t := range ts {
		// Calculate taxable threshhold date
		// Nil if already passed
		td := taxableDeadline(t.Time)
		tu := &td
		if tu.Before(time.Now()) {
			tu = nil
		}
		lots[i] = Lot{
			Purchased:    t.Time,
			TaxableUntil: tu,
			Cost:         t.Quantity * t.Price * r.Rate(t.Currency, t.Time.Year()),
			Quantity:     t.Quantity,
		}
	}
	// TODO When other costbasis strategus used, trades slice might come in ordered by something other than purchase date. Sort here
	return lots
}

// sortAssets sorts summarized assets and their yearly summaries in chronological order
func sortAssets(as []Asset) {
	for _, a := range as {
		sort.Slice(a.Years, func(i, j int) bool {
			return a.Years[i].Year < a.Years[j].Year
		})
	}

	sort.Slice(as, func(i, j int) bool {
		if as[i].FirstPurchase == nil {
			return true
		}
		if as[j].FirstPurchase == nil {
			return false
		}
		return as[i].FirstPurchase.Before(*as[j].FirstPurchase)
	})
}

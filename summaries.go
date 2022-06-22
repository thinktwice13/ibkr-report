package main

import (
	"fmt"
	"math"
	"sort"
	"time"
)

// AssetYear represents a summary of profits, fees, dividends and withholding tax paid for a single asset in a single year
type AssetYear struct {
	Pl, Taxable, Fees, Dividends, WithholdingTax float64
	Year                                         int
}

type Asset struct {
	// Financial instrument information
	*Instrument
	// Active holdings based on FIFO accounting strategy
	Holdings []Lot
	// Profits, fees, dividends and withholding tax summarized by year
	ByYear map[int]*AssetYear
}

type AssetSummary struct {
	r     Rater
	years map[int]*AssetYear
}

func (as *AssetSummary) AddFee(f *Transaction) {
	as.year(f.Year).Fees += f.Amount * as.r.Rate(f.Currency, f.Year)
}

func (as *AssetSummary) AddSale(s *Sale) {
	for _, c := range s.Basis {
		salePrice := s.Price * as.r.Rate(s.Currency, s.Time.Year())
		purchasePrice := c.Price * as.r.Rate(c.Currency, s.Time.Year())

		profit := (salePrice - purchasePrice) * c.Quantity
		sum := as.year(s.Time.Year())
		sum.Pl += profit
		if s.Time.Before(taxableDeadline(c.Time)) {
			sum.Taxable += profit
		}
	}
}

func (as *AssetSummary) AddDividend(d *Transaction) {
	as.year(d.Year).Dividends += d.Amount * as.r.Rate(d.Currency, d.Year)
}

func (as *AssetSummary) AddWithholdingTax(t *Transaction) {
	as.year(t.Year).WithholdingTax += t.Amount * as.r.Rate(t.Currency, t.Year)
}

// year finds or creates a new AssetYear for the provided year
func (as *AssetSummary) year(y int) *AssetYear {
	_, ok := as.years[y]
	if !ok {
		as.years[y] = &AssetYear{Year: y}
	}
	return as.years[y]
}

func (as *AssetSummary) List() []AssetYear {
	list := make([]AssetYear, 0, len(as.years))
	for _, v := range as.years {
		list = append(list, *v)
	}
	sort.Slice(list, func(i, j int) bool {
		return list[i].Year < list[j].Year
	})
	return list
}

type Profiter interface {
	AddSale(*Sale)
	AddFee(*Transaction)
}

type Strategy interface {
	Buy(Trade)
	Sell(float64) []Cost
	Holdings(Rater) []Lot
}

func TradeAsset(ts []Trade, s Strategy, p Profiter) {
	if len(ts) == 0 {
		return
	}

	// Process trades in chronological order
	sort.Slice(ts, func(i, j int) bool {
		return ts[i].Time.Before(ts[j].Time)
	})

	for i := range ts {
		t := &ts[i]
		if t.Fee != 0 {
			p.AddFee(feeFromTrade(t))
		}

		// Add to cost basis strategy of a purchase
		if t.Quantity > 0 {
			s.Buy(*t)
			continue
		}

		p.AddSale(&Sale{
			TradeTx: t.TradeTx,
			Basis:   s.Sell(t.Quantity),
		})
	}
}

func feeFromTrade(t *Trade) *Transaction {
	return &Transaction{
		Currency: t.Currency,
		Amount:   t.Fee,
		Year:     t.Time.Year(),
	}
}

type AssetSummaries []Asset

type Rater interface {
	Rate(string, int) float64
	Len() int
}

func NewAssetSummary(r Rater, size int) *AssetSummary {
	return &AssetSummary{
		r:     r,
		years: make(map[int]*AssetYear, size),
	}
}

// assetSummaries returns a list of summarized asset imports, given a list of imports and a type implementing a Rater if
// TODO Concurrent prices search
func assetSummaries(imports []AssetImport, r Rater) AssetSummaries {
	summaries := make(AssetSummaries, 0, len(imports))
	for i := range imports {
		ai := &imports[i]
		sum := NewAssetSummary(r, r.Len())
		fifo := new(fifo)
		TradeAsset(ai.Trades, fifo, sum)

		// Summarize dividends and withholding tax
		for i := range ai.Dividends {
			sum.AddDividend(&ai.Dividends[i])
		}
		for i := range ai.WithholdingTax {
			sum.AddWithholdingTax(&ai.WithholdingTax[i])
		}

		summaries = append(summaries, Asset{
			Instrument: ai.Instrument,
			Holdings:   fifo.Holdings(r),
			ByYear:     sum.years,
		})
	}
	return summaries
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

// Sell returns the cost basis of the given quantity of shares
func (f *fifo) Sell(qty float64) []Cost {
	if qty >= 0 {
		fmt.Println("Sell quantity must be negative")
		return nil
	}

	var costs []Cost
	// Remove shares from the front of the queue until the quantity is reached or the queue is empty
	for {
		if qty == 0 {
			break
		}

		//
		if len(f.data) == 0 {
			f.data = nil
			return nil
		}

		p := f.next()
		cost := *p
		cost.Quantity = math.Min(p.Quantity, math.Abs(qty))

		// Update quantity of lot to be sold
		p.Quantity -= cost.Quantity
		qty += cost.Quantity
		costs = append(costs, cost)

		// If lot is fully sold, remove it from the list
		if p.Quantity == 0 {
			f.data = f.data[1:]
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

type Cost struct {
	*TradeTx
	Quantity float64
}

type Sale struct {
	*TradeTx
	Basis []Cost
}

// taxableDeadline determines the last date profits can be taxed
// Tax introduced for assets purchased after 31/12/2015. For any deadlines before, return provided date
// For any taxable events provided after the 31/12/2015, calculate deadline at 24 months after provided time
func taxableDeadline(since time.Time) time.Time {
	if since.Before(time.Date(2016, 1, 1, 1, 0, 0, 0, time.UTC)) {
		return since
	}
	return since.AddDate(2, 0, 0)
}

// WriteTo writes report data to RowWriter implementing type
func (s *AssetSummaries) WriteTo(rw RowWriter) error {
	srows := make([][]interface{}, 0, len(*s))
	hrows := make([][]interface{}, 0, len(*s))
	// FIXME rows len = summaries len * years len / holdings len

	srows = append(srows, []interface{}{
		"Asset",
		"Category",
		"yr",
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
		for _, y := range a.ByYear {
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

func lotsFromTrades(cs []Cost, r Rater) []Lot {
	if len(cs) == 0 {
		return nil
	}

	lots := make([]Lot, len(cs))
	for i, c := range cs {
		cost := taxableDeadline(c.Time)
		tu := &cost
		if tu.Before(time.Now()) {
			tu = nil
		}
		lots[i] = Lot{
			Purchased:    c.Time,
			TaxableUntil: tu,
			Cost:         c.Quantity * c.Price * r.Rate(c.Currency, c.Time.Year()),
			Quantity:     c.Quantity,
		}
	}

	sort.Slice(lots, func(i, j int) bool {
		return lots[i].Purchased.Before(lots[j].Purchased)
	})
	return lots
}

package main

import (
	"time"
)

type Transaction struct {
	Currency string
	Amount   float64
	Year     int
}

type Trade struct {
	Time                 time.Time
	Currency             string
	Price, Quantity, Fee float64
}

type Instrument struct {
	Symbols  []string
	Category string
}

type AssetImport struct {
	Instrument
	Trades                    []Trade
	Dividends, WithholdingTax []Transaction
}

type ImportResults struct {
	assets     assets
	fees       []Transaction
	currencies map[string]bool
	years      map[int]bool
}

func NewImportResults() *ImportResults {
	return &ImportResults{
		assets: map[string]*AssetImport{},
		currencies: map[string]bool{
			"EUR": true,
			"USD": true,
			"GBP": true,
		},
		years: map[int]bool{
			time.Now().Year(): true,
		},
	}
}

func (r *ImportResults) AddInstrumentInfo(symbols []string, cat string) {
	a := r.assets.bySymbols(symbols...)
	if a.Category != "" || cat == "" {
		return
	}
	a.Category = cat
}
func (r *ImportResults) AddTrade(sym, ccy string, tm time.Time, qty, price, fee float64) {
	a := r.assets.bySymbols(sym)
	t := Trade{}
	t.Currency = ccy
	t.Time = tm
	t.Quantity = qty
	t.Price = price
	t.Fee = fee
	a.Trades = append(a.Trades, t)

	r.currencies[ccy] = true
	r.years[tm.Year()] = true

}
func (r *ImportResults) AddDividend(sym, ccy string, yr int, amt float64, isTax bool) {
	a := r.assets.bySymbols(sym)
	d := Transaction{}
	d.Currency = ccy
	d.Amount = amt
	d.Year = yr
	if isTax {
		a.WithholdingTax = append(a.WithholdingTax, d)
		return
	}
	a.Dividends = append(a.Dividends, d)

	r.currencies[ccy] = true
	r.years[yr] = true
}
func (r *ImportResults) AddFee(ccy string, amt float64, yr int) {
	f := Transaction{}
	f.Currency = ccy
	f.Year = yr
	f.Amount = amt
	r.fees = append(r.fees, f)

	r.currencies[ccy] = true
	r.years[yr] = true

}

func (as assets) bySymbols(ss ...string) *AssetImport {
	if len(ss) == 0 {
		return nil
	}

	var base *AssetImport
	newSymbols := make([]string, 0, len(ss))

	for _, s := range ss {
		if s == "" {
			continue
		}

		match, ok := as[s]

		if !ok && base == nil {
			newSymbols = append(newSymbols, s)
			continue
		}

		if !ok {
			base.Symbols = append(base.Symbols, s)
			as[s] = match
			continue
		}

		if base == nil {
			base = match
			for _, s := range newSymbols {
				base.Symbols = append(base.Symbols, s)
			}
			continue
		}

		if base != match {
			// Resolve conflict
			mergeAsset(*match, base)
			continue
		}
	}

	if base == nil {
		base = &AssetImport{Instrument: Instrument{Symbols: ss}}
	}

	for _, s := range base.Symbols {
		as[s] = base
	}

	return base
}

func mergeAsset(src AssetImport, tgt *AssetImport) {
	found := make(map[string]bool, len(src.Symbols))
	for _, symbol := range tgt.Symbols {
		found[symbol] = true
	}
	for _, symbol := range src.Symbols {
		found[symbol] = true
	}

	list := make([]string, 0, len(found))
	for s := range found {
		list = append(list, s)
	}

	tgt.Symbols = list

	// TODO improve and check for mismatch when not empty
	if tgt.Category == "" {
		tgt.Category = src.Category
	}

	tgt.Trades = append(tgt.Trades, src.Trades...)
	tgt.Dividends = append(tgt.Dividends, src.Dividends...)
}

type assets map[string]*AssetImport

func (a assets) list() []AssetImport {
	listed := make(map[*AssetImport]bool)
	var list []AssetImport
	for _, a := range a {
		if _, ok := listed[a]; ok {
			continue
		}

		list = append(list, *a)
		listed[a] = true
	}
	return list
}

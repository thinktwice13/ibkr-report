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
	assets assets

	// List of transactions not related to any specific asset
	// i.e. broker subsctiptions
	fees []Transaction

	// Mapped unique currencies and years found in any imported events
	currencies map[string]bool
	years      map[int]bool
}

// Domicile extracts the instrument country code used fo the ForeignIncome tax report
// Requires ISIN to be one of the symbols to work properly
// Fallback if the first symbol foundn in the symbols array
func (i *Instrument) Domicile() string {
	var isin string
	for _, symbol := range i.Symbols {
		if len(symbol) != 11 {
			continue
		}
		isin = symbol
		break
	}

	if isin == "" {
		return i.Symbols[0]
	}

	return isin[:2]
}

// NewImportResults initializes new import results struct
// Sets default surrencies
// Sets default year to current year
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

// bySymbols func looks up any assets already imported by incoming symbols
// If none found, creates new asset and maps to ll its symbols
// If at least one symbol is matched with existing assets, merges information and all the symbols
//
// If incoming symbols are matched with more than one distinct asset, merges conflict and uses the first found match
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

// mergeAsset merges information on the assets and jions all founc events: trades, dividends and withholding tax tax
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

// assets maps imported asset information by symbol, for easier lookup while importing
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

type YearAmount struct {
	Amount float64
	Year   int
}

func convertFees(fees []Transaction, r Rater) []YearAmount {
	converted := make([]YearAmount, len(fees))
	for i := range fees {
		f := &fees[i]
		converted[i] = YearAmount{
			Amount: f.Amount * r.Rate(f.Currency, f.Year),
			Year:   f.Year,
		}
	}
	return converted

}

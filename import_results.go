package main

import (
	"sync"
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

	l *sync.Mutex
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
		l: new(sync.Mutex),
	}
}

func (r *ImportResults) AddInstrumentInfo(symbols []string, cat string) {
	r.l.Lock()
	defer r.l.Unlock()
	a := r.assets.bySymbols(symbols...)
	if a.Category != "" || cat == "" {
		return
	}
	a.Category = cat
}
func (r *ImportResults) AddTrade(sym, ccy string, tm *time.Time, qty, price, fee float64) {
	if sym == "" || ccy == "" || tm == nil || qty*price == 0 {
		return
	}
	r.l.Lock()
	defer r.l.Unlock()
	a := r.assets.bySymbols(sym)
	t := Trade{}
	t.Currency = ccy
	t.Time = *tm
	t.Quantity = qty
	t.Price = price
	t.Fee = fee
	a.Trades = append(a.Trades, t)

	r.currencies[ccy] = true
	r.years[tm.Year()] = true

}
func (r *ImportResults) AddDividend(sym, ccy string, yr int, amt float64, isTax bool) {
	if sym == "" || ccy == "" || yr == 0 || amt == 0 {
		return
	}
	r.l.Lock()
	defer r.l.Unlock()
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
	if yr == 0 || amt == 0 || ccy == "" {
		return
	}
	r.l.Lock()
	defer r.l.Unlock()
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
	if len(ss) < 1 {
		return nil
	}

	if len(ss) == 1 {
		match, ok := as[ss[0]]
		if !ok {
			a := &AssetImport{Instrument: Instrument{Symbols: ss}}
			as[ss[0]] = a
			return a
		}

		return match
	}

	// Target is the merged asset combining all of the info of incoming symbols and previously included symbols for an instrument
	// Processed symbols used to avoid processing symbols twice
	// Unmatched symbols slcie stores incoming symbols not matched with any existing assets. Once any match has been found, not needed anymore
	var target *AssetImport
	processed := make(map[string]bool, len(ss))
	unmatched := make([]string, 0, len(ss))
	for _, s := range ss {
		// Skip processinf the same symbol twice
		if _, ok := processed[s]; ok {
			continue
		}

		// Find existing match
		// If first match found (target), set target and add include all unmatched symbols
		// If match not found, but target already found, just add the symbol
		// Skip if match found and equal to the target
		// Merge info to target if match found not equal
		match, ok := as[s]
		if target == nil && !ok {
			unmatched = append(unmatched, s)
			continue
		}

		if target == nil {
			target = match
			target.Symbols = append(target.Symbols, unmatched...)
			unmatched = nil
			for _, matchedSymbol := range match.Symbols {
				processed[matchedSymbol] = true
			}
			continue
		}

		if !ok {
			target.Symbols = append(target.Symbols, s)
			processed[s] = true
			continue
		}

		if match == target {
			continue
		}

		for _, matchedSymbol := range match.Symbols {
			processed[matchedSymbol] = true
		}
		mergeAsset(*match, target)
	}

	// If still no matches found
	if target == nil {
		target = &AssetImport{Instrument: Instrument{Symbols: ss}}
	}

	for _, s := range target.Symbols {
		as[s] = target
	}

	return target
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
	if tgt.Category == "" && src.Category != "" {
		tgt.Category = src.Category
	}

	tgt.Trades = append(tgt.Trades, src.Trades...)
	tgt.Dividends = append(tgt.Dividends, src.Dividends...)
	tgt.WithholdingTax = append(tgt.WithholdingTax, src.WithholdingTax...)
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

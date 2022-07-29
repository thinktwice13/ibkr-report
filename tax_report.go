package main

import (
	"math"
	"sort"
)

// CapitalGains represents a year of capital gains to be reported for taxes
type CapitalGains struct {
	Pl, Fees, Dividends float64
}

// FgnIncome represents year of taxed foreign income to be reported
type FgnIncome struct {
	Received, TaxPaid float64
	Src               string
}

// TaxYear is a temporary type used to summarize income from multiple assets
type TaxYear struct {
	CapitalGains
	FgnIncomeBySrc map[string]*FgnIncome
	yr             int
}

// TaxReport is a temporary type used to summarize income from multiple assets
type TaxReport map[int]*TaxYear

// taxReports builds a yearly tax report provided a list of summarized assets and portfolio fees
// Returns a slice of tax years to be reported
func taxReports(assets []Asset, fees []YearAmount, r TaxReport) TaxReport {
	for _, a := range assets {
		for _, sum := range a.ByYear {
			// Get or init a year for tax report
			y := r.year(sum.Year)

			// If any tax withheld for an asset in a given year, report all but fees to foregin income report
			// Otherwise, report all in JOPPD report
			// Only report dividends for equity or when category not defined
			// TODO Search category when empty
			// Always report fees in JOPPD report
			y.Fees += sum.Fees
			if sum.WithholdingTax == 0 {
				y.Pl += sum.Taxable
				if a.Category == "Equity" || a.Category == "" {
					y.Dividends = sum.Dividends
				}
				continue
			}
			// With tax withheld
			// TODO Check if dividends reported when tax taken
			src := y.incomeSource(a.Domicile())
			src.Received += sum.Taxable
			if a.Category == "Equity" || a.Category == "" {
				src.Received += sum.Dividends
			}
			src.TaxPaid += sum.WithholdingTax
		}
	}

	// Apply fees as a deductible from capital gains
	for _, f := range fees {
		r.year(f.Year).Fees += f.Amount
	}

	// Adjust capital gains report: Deduct fees paid in a year from positive yearly sales profits
	// Report zero profit for years ending in negative profit
	// Deduct fees from profit if profit positive
	// Delete entire year from report if profit is zero and no dividends received
	for _, y := range r {
		if y.Pl <= 0 {
			y.Pl = 0
			continue
		}
		deducted := math.Min(y.Pl, math.Abs(y.Fees))
		y.Pl -= deducted
		y.Fees += deducted
	}

	return r
}

// year finds or creates a tax year for the given year
func (r TaxReport) year(y int) *TaxYear {
	_, ok := r[y]
	if !ok {
		r[y] = &TaxYear{
			FgnIncomeBySrc: map[string]*FgnIncome{},
			yr:             y,
		}
	}
	return r[y]
}

// incomeSource finds or creates a foreign income source recorded in the given tax year
func (y *TaxYear) incomeSource(src string) *FgnIncome {
	_, ok := y.FgnIncomeBySrc[src]
	if !ok {
		y.FgnIncomeBySrc[src] = &FgnIncome{Src: src}
	}
	return y.FgnIncomeBySrc[src]
}

func (r TaxReport) WriteTo(rw RowWriter) error {
	list := make([]*TaxYear, 0, len(r))
	for _, y := range r {
		list = append(list, y)
	}
	sort.Slice(list, func(i, j int) bool {
		return list[i].yr < list[j].yr
	})

	jrows := make([][]interface{}, 1, len(r))
	jrows[0] = []interface{}{
		"yr",
		"Dividends",
		"Profit/Loss",
		"Deductible",
	}
	irows := make([][]interface{}, 1, len(r))
	irows[0] = []interface{}{
		"yr",
		"Income Source",
		"Received",
		"Withholding Tax",
	}

	for _, y := range list {
		fgnIncomeYrs := make([]*FgnIncome, 0, len(y.FgnIncomeBySrc))
		for _, fgnIncome := range y.FgnIncomeBySrc {
			fgnIncomeYrs = append(fgnIncomeYrs, fgnIncome)
		}
		sort.Slice(fgnIncomeYrs, func(i, j int) bool {
			return fgnIncomeYrs[i].Src < fgnIncomeYrs[j].Src
		})

		jrows = append(jrows, []interface{}{
			y.yr,
			RoundDec(y.Dividends, 2),
			RoundDec(y.Pl, 2),
			RoundDec(y.Fees, 2),
		})

		for _, fi := range fgnIncomeYrs {
			irows = append(irows, []interface{}{
				y.yr,
				fi.Src,
				RoundDec(fi.Received, 2),
				RoundDec(fi.TaxPaid, 2),
			})
		}
	}

	err := rw.WriteRows("INO-DOH", irows)
	err = rw.WriteRows("JOPPD", jrows)

	if err != nil {
		return err
	}

	return nil
}

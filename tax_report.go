package main

import (
	"math"
	"sort"
)

type Reporter interface {
	WriteTo(RowWriter) error
}

type RowWriter interface {
	WriteRows(string, [][]interface{}) error
}

// CapitalGains represents a year of capital gains to be reported for taxes
type CapitalGains struct {
	Pl, Fees, Dividends float64
}

// FgnIncome represents year of taxed foreign income to be reported
type FgnIncome struct {
	Received, TaxPaid float64
	Src               string
}

// taxYear is a temporary type used to summarize income from multiple assets
type taxYear struct {
	CapitalGains
	taxedForeignIncome map[string]*FgnIncome
}

type TaxReport []TaxYear

// TaxYear is similar to taxYear, but
type TaxYear struct {
	CapitalGains
	TaxedForeignIncome []FgnIncome
	Year               int
}

type taxByYear map[int]*taxYear

// taxReport builds a yearly tax report provided a list of summarized assets and portfolio fees
// Returns a slice of tax years to be reported
func taxReport(assets []Asset, fees []YearAmount, yrs int) TaxReport {
	r := make(taxByYear, yrs)

	// Add portfolio fees first to use as a deductible when adding profits
	for _, f := range fees {
		r.year(f.Year).Fees += f.Amount
	}

	for _, a := range assets {
		for _, sum := range a.Years {
			y := r.year(sum.Year)

			// Profits
			y.Pl += sum.Taxable
			y.Fees += sum.Fees

			// Dividends
			// Do not report dividend income for non-equity assets
			// If asset has tax withheld in a given year, report all of the dividends and tax paid as foreign taxed income. Froup by country of origin
			// Otherwise, report received dividends as capital gains
			if a.Category != "Stocks" {
				continue
			}
			if sum.WithholdingTax == 0 {
				y.Dividends += sum.Dividends
				continue
			}
			src := y.incomeSource(a.Domicile())
			src.Received += sum.Dividends
			src.TaxPaid += sum.WithholdingTax
		}
	}

	// Adjust profits: Deduct fees paid in a year from positive yearly profits
	// Report zero profit for years ending in negative profits
	// Delete entire year from report if profit is zero and no dividends received
	for k, y := range r {
		if y.Pl <= 0 {
			y.Pl = 0
		} else {
			deductible := math.Min(y.Pl, math.Abs(y.Fees))
			y.Pl -= deductible
			y.Fees += deductible
		}

		if y.Pl+y.Dividends == 0 {
			delete(r, k)
		}
	}

	return r.toList()
}

func (r taxByYear) toList() TaxReport {
	list := make(TaxReport, 0, len(r))
	for y, tax := range r {
		ty := TaxYear{
			CapitalGains:       tax.CapitalGains,
			TaxedForeignIncome: nil,
			Year:               y,
		}

		if len(tax.taxedForeignIncome) != 0 {
			for src, income := range tax.taxedForeignIncome {
				income.Src = src
				ty.TaxedForeignIncome = append(ty.TaxedForeignIncome, *income)
			}

			sort.Slice(ty.TaxedForeignIncome, func(i, j int) bool {
				return ty.TaxedForeignIncome[i].Src < ty.TaxedForeignIncome[j].Src
			})
		}

		list = append(list, ty)
	}

	sort.Slice(list, func(i, j int) bool {
		return list[i].Year < list[j].Year
	})

	return list
}

func (r taxByYear) year(y int) *taxYear {
	_, ok := r[y]
	if !ok {
		r[y] = &taxYear{
			taxedForeignIncome: map[string]*FgnIncome{},
		}
	}
	return r[y]
}

func (y *taxYear) incomeSource(src string) *FgnIncome {
	_, ok := y.taxedForeignIncome[src]
	if !ok {
		y.taxedForeignIncome[src] = new(FgnIncome)
	}
	return y.taxedForeignIncome[src]
}

func (r *TaxReport) WriteTo(rw RowWriter) error {
	jrows := make([][]interface{}, 1, len(*r))
	jrows[0] = []interface{}{
		"Year",
		"Dividends",
		"Profit/Loss",
		"Deductible",
	}
	irows := make([][]interface{}, 1, len(*r))
	irows[0] = []interface{}{
		"Year",
		"Income Source",
		"Received",
		"Withholding Tax",
	}

	for _, y := range *r {
		jrows = append(jrows, []interface{}{
			y.Year,
			RoundDec(y.Dividends, 2),
			RoundDec(y.Pl, 2),
			RoundDec(y.Fees, 2),
		})

		if y.TaxedForeignIncome == nil {
			continue
		}

		for _, fi := range y.TaxedForeignIncome {
			irows = append(irows, []interface{}{
				y.Year,
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

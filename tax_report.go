package main

import (
	"math"
	"sort"
)

type RowWriter interface {
	WriteRows(string, [][]interface{}) error
}

type CapitalGains struct {
	Pl, Fees, Dividends float64
}

type FgnIncome struct {
	Received, TaxPaid float64
	Src               string
}

type taxYr struct {
	CapitalGains
	taxedForeignIncome map[string]*FgnIncome
}

type TaxReport []TaxYear
type TaxYear struct {
	CapitalGains
	TaxedForeignIncome []FgnIncome
	Year               int
}

type taxByYear map[int]*taxYr

func (r taxByYear) year(y int) *taxYr {
	_, ok := r[y]
	if !ok {
		r[y] = &taxYr{
			taxedForeignIncome: map[string]*FgnIncome{},
		}
	}
	return r[y]
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

func (y *taxYr) incomeSource(src string) *FgnIncome {
	_, ok := y.taxedForeignIncome[src]
	if !ok {
		y.taxedForeignIncome[src] = new(FgnIncome)
	}
	return y.taxedForeignIncome[src]
}

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

			if y.Pl > 0 {
				deductible := math.Min(y.Pl, math.Abs(y.Fees))
				y.Pl -= deductible
				y.Fees += deductible
			}

			// Dividends
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

	return r.toList()
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
			y.Dividends,
			y.Pl,
			y.Fees,
		})

		if y.TaxedForeignIncome == nil {
			continue
		}

		for _, fi := range y.TaxedForeignIncome {
			irows = append(irows, []interface{}{
				y.Year,
				fi.Src,
				fi.Received,
				fi.TaxPaid,
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

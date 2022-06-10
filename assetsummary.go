package main

type AssetYear struct {
	Pl, Taxable, Fees, Dividends, WithholdingTax float64
}

type Asset struct {
	Instrument
	Summary map[int]*AssetYear
}

func NewAssetSummary(size int) map[int]*AssetYear {
	return make(map[int]*AssetYear, size)
}

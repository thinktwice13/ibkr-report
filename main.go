package main

import (
	"encoding/json"
	"fmt"
)

func main() {
	fmt.Println("Hello World")
	assets, _, _, _ := readDir()
	PrettyPrint(assets)

}

func PrettyPrint(a any) {
	s, _ := json.MarshalIndent(a, "", "\t")
	fmt.Println(string(s))
}

type AssetYear struct {
	Pl, Taxable, Fees, Dividends, WithholdingTax float64
}

type Asset struct {
	Instrument
	Summary map[int]*AssetYear
}

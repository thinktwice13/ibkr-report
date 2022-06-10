package main

import (
	"encoding/json"
	"fmt"
	"ibkr-report/fx"
	"log"
)

func main() {
	fmt.Println("Hello World")
	assets, fees, years, currencies := readDir()

	rates, err := fx.New(currencies, years)
	if err != nil {
		log.Fatalln(err)
	}

	summaries := summarizeAssets(assets, rates)
	print(len(summaries))
	PrettyPrint(summaries)
	convFees := convertFees(fees, rates)
	tr := buildTaxReport(summaries, convFees, len(years))
	PrettyPrint(tr)
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

func PrettyPrint(a any) {
	s, _ := json.MarshalIndent(a, "", "\t")
	fmt.Println(string(s))
}

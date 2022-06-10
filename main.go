package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"
)

func main() {
	t := time.Now()
	assets, fees, years, currencies := readDir()

	if len(assets) == 0 {
		fmt.Println("No data found. Exiting")
		os.Exit(0)
	}

	// Fetch currency conversion rates per year
	rates, err := NewFxRates(currencies, years)
	if err != nil {
		log.Fatalln(err)
	}

	// Summarize imported asset events by year
	summaries := summarizeAssets(assets, rates)

	// Convert global fees (not related to asset events)
	convFees := convertFees(fees, rates)

	// Build tax report from asset summaries
	tr := taxReport(summaries, convFees, len(years))

	// Write asset summaries and tax reports to spreadsheet
	r := NewReport("Portfolio Report")
	err = writeReport(&tr, r)
	err = writeReport(&summaries, r)
	err = r.Save()
	if err != nil {
		log.Fatalln(err)
	}
	fmt.Println("Finished in:", time.Since(t))
}

func PrettyPrint(a any) {
	s, _ := json.MarshalIndent(a, "", "\t")
	fmt.Println(string(s))
}

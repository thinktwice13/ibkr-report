package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"time"
)

var maxWorkers int

func main() {
	t := time.Now()
	maxWorkers = runtime.NumCPU()
	SetPWD()

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
	writeReport(&tr, r)
	writeReport(&summaries, r)
	err = r.Save()
	err = createXlsTemplate()
	if err != nil {
		log.Fatalln(err)
	}
	fmt.Println("Finished in", time.Since(t))
}

func SetPWD() {
	if os.Getenv("GOPATH") == "" {
		dirErr := "could not read directory"
		exec, err := os.Executable()
		if err != nil {
			log.Fatalln(dirErr)
		}
		err = os.Setenv("PWD", filepath.Dir(exec))
		if err != nil {
			log.Fatalln(dirErr)
		}
	}
}

func PrettyPrint(a any) {
	s, _ := json.MarshalIndent(a, "", "\t")
	fmt.Println(string(s))
}

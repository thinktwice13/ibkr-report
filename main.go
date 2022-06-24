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
	setPWD()
	ir := readDir()
	if len(ir.assets) == 0 {
		log.Fatalf("No portfolio data found in %s\n", os.Getenv("PWD"))
	}

	rates := NewFxRates(list(ir.currencies), list(ir.years))
	tr := make(TaxReport, len(list(ir.years)))

	summaries := assetSummaries(ir.assets.list(), rates)
	// TODO Search prices
	taxReports(summaries, convFees(ir.fees, rates), tr)

	f := NewReport("Portfolio Report")
	summaries.WriteTo(f)
	tr.WriteTo(f)
	f.Save()

	createXlsTemplate()
	fmt.Println("Finished in", time.Since(t))
}

// setPWD sets the current working directory to the directory of the executable
func setPWD() {
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

// writeFile writes items received on the reporters channel to a spreadsheet file
func writeFile(rs <-chan Reporter) <-chan bool {
	done := make(chan bool, 1)
	go func() {
		file := NewReport("Portfolio Report")
		for r := range rs {
			err := r.WriteTo(file)
			if err != nil {
				log.Fatalf("Error: %v\n", err)
			}
		}
		err := file.Save()
		if err != nil {
			log.Fatalf("Error: %v\n", err)
		}
		done <- true
	}()
	return done
}

func PrettyPrint(a any) {
	s, _ := json.MarshalIndent(a, "", "\t")
	fmt.Println(string(s))
}

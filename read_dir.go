package main

import (
	"bufio"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// readDir imports all data found in curent directory
func readDir() ([]AssetImport, []Transaction, []int, []string) {
	files := findFiles()
	ir := NewImportResults()
	for _, file := range files {
		ReadStatement(file, ir)
	}

	return ir.assets.list(), ir.fees, list(ir.years), list(ir.currencies)
}

// findFiles walks the current directory and looks for .csv files
func findFiles() []string {
	var files []string
	filepath.WalkDir(os.Getenv("PWD"), func(path string, d fs.DirEntry, err error) error {
		if d.IsDir() && d.Name()[:1] == "." {
			return filepath.SkipDir
		}
		if filepath.Ext(d.Name()) == ".csv" {
			files = append(files, path)
		}
		return nil
	})
	fmt.Printf("%d files found. Working... \n", len(files))
	return files
}

// ReadStatement reads single csv IBKR file
// Filters csv lines by relevant sections and uses *ImportResults to send files to
func ReadStatement(filename string, ir *ImportResults) {
	file, err := os.Open(filename)
	if err != nil {
		fmt.Printf("cannot open file %s %v", filename, err)
	}
	defer file.Close()

	reader := csv.NewReader(bufio.NewReader(file))
	reader.FieldsPerRecord = -1 // Disable record length test in the CSV
	reader.LazyQuotes = true    // Allow quote in unquoted field

	// Not all csv lines are neded. Get new section handlers for each file
	sections := ibkrSections()
	// header slice keeps the csv line used as a header for the section currently being read
	var header []string
	for {
		line, err := reader.Read()
		if err == io.EOF {
			break
		}

		if err != nil {
			continue
		}

		// Section is recognized by the first field item in the line slice
		// If not found in the ibkr line handlers map, ignore entire line
		handle, ok := sections[line[0]]
		if !ok {
			continue
		}

		if line[1] == "Header" {
			header = line
			continue
		}

		lm, err := mapLine(line, header)
		if err != nil {
			continue
		}

		handle(lm, ir)
	}
}

type lineHandler func(map[string]string, *ImportResults)

// ibkrSections returns map of IBKR csv line handlers mapped by relevant section
func ibkrSections() map[string]lineHandler {
	return map[string]lineHandler{
		"Financial Instrument Information": handleInstrumentLine,
		"Trades":                           handleTradeLine,
		"Dividends":                        handleDividendLine,
		"Withholding Tax":                  handleWithholdingTaxLine,
		"Fees":                             handleFeeLine,
	}
}

// mapLine uses a csv line and a related header line to construct a value to field map for easier field lookup while importing lines
func mapLine(data, header []string) (map[string]string, error) {
	if header == nil {
		return nil, errors.New("cannot convert to row from empty header")
	}

	if len(header) != len(data) {
		return nil, errors.New("header and line length mismatch")
	}

	header[0], data[0] = "Section", header[0]

	lm := make(map[string]string, len(data))
	for pos, field := range header {
		lm[field] = data[pos]
	}

	return lm, nil
}

// handleInstrumentLine handles the instrument information lines of the IBKR csv statement
func handleInstrumentLine(lm map[string]string, ir *ImportResults) {
	symbols := append(strings.Split(strings.ReplaceAll(lm["Symbol"], " ", ""), ","), formatISIN(lm["Security ID"]))
	if len(symbols) == 0 {
		return
	}

	ir.AddInstrumentInfo(symbols, lm["Asset Category"])
}

// Adds "US" prefix to US security ISIN codes and removes the 12th check digit
func formatISIN(sID string) string {
	if sID == "" {
		log.Fatal("empty security ID")
	}

	if len(sID) > 8 && len(sID) < 11 {
		// US ISIN number. Add country code
		sID = "US" + sID
	}

	if len(sID) == 12 {
		// Remove ISIN check digit
		sID = sID[:11]
		return sID
	}

	return sID
}

// handleTradeLine handles the trade lines of the IBKR csv statement
func handleTradeLine(lm map[string]string, ir *ImportResults) {
	if lm["Date/Time"] == "" || lm["Asset Category"] == "Forex" || lm["Symbol"] == "" {
		return
	}
	t := timeFromExact(lm["Date/Time"])
	c := lm["Currency"]
	ir.AddTrade(lm["Symbol"], c, t, amountFromString(lm["Quantity"]), amountFromString(lm["T. Price"]), amountFromString(lm["Comm/Fee"]))
}

// handleDividendLine handles the dividend lines of the IBKR csv statement
func handleDividendLine(lm map[string]string, ir *ImportResults) {
	if lm["Date"] == "" {
		return
	}
	symbol, err := symbolFromDescription(lm["Description"])
	if err != nil {
		return
	}
	ir.AddDividend(symbol, lm["Currency"], yearFromDate(lm["Date"]), amountFromString(lm["Amount"]), false)
}

// yearFromDate extracts a year from IBKR csv date field
func yearFromDate(s string) int {
	y, err := strconv.Atoi(s[:4])
	if err != nil {
		return 0
	}
	return y
}

// handleWithholdingTaxLine handles the withohlding tax lines of the IBKR csv statement
// TODO reuse dividend line handler
func handleWithholdingTaxLine(lm map[string]string, ir *ImportResults) {
	if lm["Date"] == "" {
		return
	}

	symbol, err := symbolFromDescription(lm["Description"])
	if err != nil {
		return
	}

	ir.AddDividend(symbol, lm["Currency"], yearFromDate(lm["Date"]), amountFromString(lm["Amount"]), true)
}

// handleFeeLine handles the fee lines of the IBKR csv statement
func handleFeeLine(lm map[string]string, ir *ImportResults) {
	if lm["Date"] == "" {
		return
	}

	ir.AddFee(lm["Currency"], amountFromString(lm["Amount"]), yearFromDate(lm["Date"]))
}

// symbolFromDescription extracts a symbol from IBKR csv dividend lines
func symbolFromDescription(d string) (string, error) {
	if d == "" {
		return "", errors.New("cannot create asset event without symbol")
	}

	// This is a dividend or withholding tax
	parensIdx := strings.Index(d, "(")
	if parensIdx == -1 {
		return "", errors.New("cannot create asset event without symbol")
	}

	symbol := strings.ReplaceAll(d[:parensIdx], " ", "")
	if symbol == "" {
		return "", errors.New("cannor find symbol in description")
	}
	return symbol, nil
}

// amountFromString formats number strings to float64 type
func amountFromString(s string) float64 {
	var v float64
	if s == "" {
		log.Fatalf("Cannot create amount from %s", s)
	}
	s = strings.ReplaceAll(s, ",", "")
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		log.Printf("error parsing float from from %v", err)
	}
	return v
}

// timeFromExact extracts time.Time from IBKR csv time field
func timeFromExact(t string) time.Time {
	timeStr := strings.Join(strings.Split(t, ","), "")
	tm, err := time.Parse("2006-01-02 15:04:05", timeStr)
	if err != nil {
		panic(err)
	}

	return tm
}

type key interface {
	string | float64 | int
}

// list returns a list of map keys if the key implements key interface
func list[T key](m map[T]bool) []T {
	var l []T
	for k := range m {
		l = append(l, k)
	}
	return l
}

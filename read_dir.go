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

func readDir() {
	files := findFiles()
	for _, file := range files {
		readFile(file)
	}
}

func findFiles() []string {
	var files []string
	err := filepath.WalkDir(".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			fmt.Printf("prevent panic by handling failure accessing a path %q: %v\n", path, err)
			return err
		}

		ext := filepath.Ext(d.Name())
		if ext == ".csv" {
			files = append(files, d.Name())
		}

		return nil
	})

	if err != nil {
		fmt.Printf("error walking the path %q: %v\n", ".", err)
		return nil
	}

	return files
}

func readFile(filename string) {
	file, err := os.Open(filename)
	if err != nil {
		fmt.Printf("cannot open file %s", filename)
	}
	defer file.Close()

	reader := csv.NewReader(bufio.NewReader(file))
	reader.FieldsPerRecord = -1 // Disable record length test in the CSV
	reader.LazyQuotes = true    // Allow quote in unquoted field

	sections := ibkrSections()
	var header []string
	for {
		line, err := reader.Read()
		if err == io.EOF {
			break
		}

		if err != nil {
			continue
		}

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

		handle(lm)
	}
}

type lineHandler func(map[string]string)

func handleLine(m map[string]string) {
	fmt.Println(m)
}

func ibkrSections() map[string]lineHandler {
	return map[string]lineHandler{
		"Financial Instrument Information": handleInstrumentLine,
		"Trades":                           handleTradeLine,
		"Dividends":                        handleDividendLine,
		"Withholding Tax":                  handleWithholdingTaxLine,
		"Fees":                             handleFeeLine,
	}
}

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

func handleInstrumentLine(lm map[string]string) {
	symbols := append(strings.Split(strings.ReplaceAll(lm["Symbol"], " ", ""), ","), formatISIN(lm["Security ID"]))
	if len(symbols) == 0 {
		return
	}

	fmt.Println(symbols, lm["Asset Category"])
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

func handleTradeLine(lm map[string]string) {
	if lm["Date/Time"] == "" || lm["Asset Category"] == "Forex" || lm["Symbol"] == "" {
		return
	}
	t := timeFromExact(lm["Date/Time"])
	c := lm["Currency"]
	fmt.Println(lm["Symbol"], c, t, amountFromString(lm["Quantity"]), amountFromString(lm["T. Price"]), amountFromString(lm["Comm/Fee"]))
}
func handleDividendLine(lm map[string]string) {
	if lm["Date"] == "" {
		return
	}
	symbol, err := symbolFromDescription(lm["Description"])
	if err != nil {
		return
	}
	fmt.Println(symbol, lm["Currency"], yearFromDate(lm["Date"]), amountFromString(lm["Amount"]), false)
}

func yearFromDate(s string) int {
	y, err := strconv.Atoi(s[:4])
	if err != nil {
		return 0
	}
	return y
}

func handleWithholdingTaxLine(lm map[string]string) {
	if lm["Date"] == "" {
		return
	}

	symbol, err := symbolFromDescription(lm["Description"])
	if err != nil {
		return
	}

	fmt.Println(symbol, lm["Currency"], yearFromDate(lm["Date"]), amountFromString(lm["Amount"]), true)
}
func handleFeeLine(lm map[string]string) {
	if lm["Date"] == "" {
		return
	}

	fmt.Println(lm["Currency"], amountFromString(lm["Amount"]), yearFromDate(lm["Date"]))
}

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

func amountFromString(s string) (v float64) {
	if s == "" {
		log.Fatalf("Cannot create amount from %s", s)
	}
	s = strings.ReplaceAll(s, ",", "")
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		log.Printf("error parsing float from from %v", err)
	}
	return
}

func timeFromExact(t string) time.Time {
	timeStr := strings.Join(strings.Split(t, ","), "")
	tm, err := time.Parse("2006-01-02 15:04:05", timeStr)
	if err != nil {
		panic(err)
	}

	return tm
}

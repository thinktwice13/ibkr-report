package ibkr

import (
	"bufio"
	"encoding/csv"
	"errors"
	"fmt"
	"ibkr-report/broker"
	"io"
	"log"
	"os"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"
)

type instrument struct {
	isin     string
	category string
}

type reader struct {
	header []string
	rows   []map[string]string
	isins  map[string]instrument
}

func (r *reader) readRow(row []string) {
	sections := []string{"Financial Instrument Information", "Trades", "Dividends", "Withholding Tax", "Fees"}
	// Ignore if not a section we're interested in
	if !slices.Contains(sections, row[0]) {
		return
	}

	// Update header if new section
	if row[1] == "Header" {
		r.header = row
		return
	}

	// If this is financial ISIN information, add symbols to isins map
	// otherwise, map the line and store for later
	if row[0] == "Financial Instrument Information" {
		lm, err := mapIbkrLine(row, r.header)
		if err != nil {
			return
		}
		for _, s := range strings.Split(strings.ReplaceAll(lm["Symbol"], " ", ""), ",") {
			r.isins[s] = instrument{isin: formatISIN(lm["Security ID"]), category: importCategory(lm["Asset Category"])}
		}
		return
	}

	lm, err := mapIbkrLine(row, r.header)
	if err != nil || lm["Header"] != "Data" {
		return
	}

	r.rows = append(r.rows, lm)

}

// Read reads single csv IBKR file
// Filters csv lines by relevant sections and uses *ImportResults to send files to
func Read(filename string) (stmt *broker.Statement, err error) {
	file, err := os.Open(filename)
	if err != nil {
		fmt.Printf("cannot open file %s %v", filename, err)
	}
	defer func() {
		if fErr := file.Close(); fErr != nil {
			err = errors.Join(err, fErr)
		}
	}()

	csvRdr := csv.NewReader(bufio.NewReader(file))
	// Disable record length test in the CSV
	csvRdr.FieldsPerRecord = -1
	// Allow quote in unquoted field
	csvRdr.LazyQuotes = true

	rdr := reader{isins: make(map[string]instrument)}
	for {
		row, err := csvRdr.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Printf("could not read csv row: %v", err)
			continue
		}

		rdr.readRow(row)
	}

	return rdr.statement(filename)
}

func (r *reader) statement(filename string) (*broker.Statement, error) {
	stmt := &broker.Statement{Filename: filename, Broker: "IBKR"}
	for _, row := range r.rows {
		currency := row["Currency"]

		section := row["Section"]
		if section == "Trades" {
			if row["Date/Time"] == "" || row["Asset Category"] == "Forex" || row["Symbol"] == "" {
				continue
			}

			t, err := timeFromExact(row["Date/Time"])
			if err != nil {
				continue
			}

			stmt.Trades = append(stmt.Trades, broker.Trade{
				ISIN:     r.isins[row["Symbol"]].isin,
				Category: r.isins[row["Symbol"]].category,
				Time:     *t,
				Currency: currency,
				Quantity: amountFromString(row["Quantity"]),
				Price:    amountFromString(row["T. Price"]),
			})

			stmt.Fees = append(stmt.Fees, broker.Tx{
				Category: r.isins[row["Symbol"]].category,
				Currency: currency,
				Amount:   amountFromString(row["Comm/Fee"]),
				Year:     t.Year(),
			})

			continue
		}

		// All other sections only need Year as Time
		if row["Date"] == "" {
			continue
		}

		if section == "Fees" {
			stmt.Fees = append(stmt.Fees, broker.Tx{
				Currency: currency,
				Amount:   amountFromString(row["Amount"]),
				Year:     yearFromDate(row["Date"]),
			})

			continue
		}

		// Dividends and withholding Tax have the same structure and need to get a symbol from the description
		symbol, err := symbolFromDescription(row["Description"])
		if err != nil {
			continue
		}

		tx := broker.Tx{
			ISIN:     r.isins[symbol].isin,
			Category: r.isins[symbol].category,
			Currency: currency,
			Amount:   amountFromString(row["Amount"]),
			Year:     yearFromDate(row["Date"]),
		}

		if section == "Dividends" {
			stmt.FixedIncome = append(stmt.FixedIncome, tx)
		} else {
			stmt.Tax = append(stmt.Tax, tx)
		}
	}

	return stmt, nil
}

// symbolFromDescription extracts a symbol from IBKR csv dividend lines
func symbolFromDescription(d string) (string, error) {
	if d == "" {
		return "", errors.New("empty input")
	}

	// This is a dividend or withholding Tax
	parensIdx := strings.Index(d, "(")
	if parensIdx == -1 {
		return "", errors.New("cannot create asset event without symbol")
	}

	symbol := strings.ReplaceAll(d[:parensIdx], " ", "")
	if symbol == "" {
		return "", errors.New("cannot find symbol in description")
	}
	return symbol, nil
}

// yearFromDate extracts a Year from IBKR csv date field
func yearFromDate(s string) int {
	if s == "" {
		return 1900
	}
	y, err := strconv.Atoi(s[:4])
	if err != nil {
		return 0
	}
	return y
}

// timeFromExact extracts time.Time from IBKR csv Time field
func timeFromExact(t string) (*time.Time, error) {
	timeStr := strings.Join(strings.Split(t, ","), "")
	tm, err := time.Parse("2006-01-02 15:04:05", timeStr)
	if err != nil {
		return nil, errors.New("could not parse Time")
	}

	return &tm, nil
}

// mapIbkrLine uses a csv line and a related header line to construct a value to field map for easier field lookup while importing lines
func mapIbkrLine(data, header []string) (map[string]string, error) {
	if header == nil {
		return nil, errors.New("cannot convert to row from empty header")
	}

	if data == nil {
		return nil, errors.New("no data to map")
	}

	if len(header) != len(data) {
		return nil, errors.New("header and line length mismatch")
	}

	lm := make(map[string]string, len(data))
	for pos, field := range header {
		lm[field] = data[pos]
	}

	lm["Section"] = data[0]

	return lm, nil
}

// Adds "US" prefix to US security ISIN codes and removes the 12th check digit
func formatISIN(sID string) string {
	if sID == "" || len(sID) < 9 || len(sID) > 12 {
		return sID // Not ISIN
	}
	if len(sID) < 11 {
		// US ISIN number. Add country code
		sID = "US" + sID
	}
	if len(sID) == 12 {
		// Remove ISIN check digit
		return sID[:11]
	}

	return sID
}

func importCategory(c string) string {
	if c == "" {
		return ""
	}

	lc := strings.ToLower(c)
	if strings.HasPrefix(lc, "stock") || strings.HasPrefix(lc, "equit") {
		return "Equity"
	}

	return c
}

func amountFromStringOld(s string) float64 {
	if s == "" {
		return 0

	}
	// Remove all but numbers, commas and points
	re := regexp.MustCompile(`[0-9.,-]`)
	ss := strings.Join(re.FindAllString(s, -1), "")
	isNeg := ss[0] == '-'
	// Find all commas and points
	// If none found, return 0, print error
	signs := regexp.MustCompile(`[.,]`).FindAllString(ss, -1)
	if len(signs) == 0 {
		f, err := strconv.ParseFloat(ss, 64)
		if err != nil {
			fmt.Printf("could not convert %s to number", s)
			return 0
		}

		return f
	}

	// Use last sign as decimal separator and ignore others
	// Find idx and replace whatever sign was to a decimal point
	sign := signs[len(signs)-1]
	signIdx := strings.LastIndex(ss, sign)
	sign = "."
	left := regexp.MustCompile(`[0-9]`).FindAllString(ss[:signIdx], -1)
	right := ss[signIdx+1:]
	n, err := strconv.ParseFloat(strings.Join(append(left, []string{sign, right}...), ""), 64)
	if err != nil {
		fmt.Printf("could not convert %s to number", s)
		return 0
	}
	if isNeg {
		n = n * -1
	}
	return n
}

func amountFromString(s string) float64 {
	if s == "" {
		return 0
	}

	// Remove commas, spaces and all but the last decimal point
	s = strings.ReplaceAll(s, ",", "")
	s = strings.ReplaceAll(s, " ", "")
	if strings.LastIndex(s, ".") != -1 {
		s = strings.Replace(s, ".", "", strings.Count(s, ".")-1)
	}

	// Convert to float
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		log.Fatal(err)
	}

	return f
}

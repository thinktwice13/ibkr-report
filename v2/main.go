package v2

import (
	"bufio"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"math"
	"math/rand"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

var (
	workers      = runtime.NumCPU()
	baseCurrency = "EUR"
	taxRate      = 0.12
	allowance    = 15.0
)

type foreign struct {
	gains, taxPaid float64
}

type taxYear struct {
	year          int
	realizedPL    float64
	foreignIncome map[string]*foreign
}

type tx struct {
	isin, category, currency string
	amount                   float64
	year                     int
}

type brokerStatement struct {
	trades                 []trade
	fixedIncome, tax, fees []tx
	ID                     int
}

type report map[int]*taxYear

func (r report) print() {
	for _, year := range r {
		fmt.Printf("Year: %d\n", year.year)
		fmt.Printf("Realized P/L: %.2f\n", year.realizedPL)
		tax := math.Max(0, year.realizedPL-allowance) * taxRate
		fmt.Printf("Tax: %.2f\n", tax)

		if len(year.foreignIncome) > 0 {
			fmt.Println("Foreign Income:")
			for source, f := range year.foreignIncome {
				fmt.Printf("Received %.2f, paid %.2f at %s\n", f.gains, f.taxPaid, source)
			}
		}
	}
}

type pl struct {
	amount float64
	source string
	year   int
}

type ledger struct {
	tax, profits []pl
	deductable   map[int]float64
}

func Run() {
	files, err := findFiles()
	fmt.Println("Files found:", len(files))
	if err != nil {
		fmt.Println("Error finding files:", err)
		return
	}

	reports := readFiles(files)
	l := newLedger(reports)
	r := newReport(l)
	r.print()
}

type fxExchange struct {
	m     *sync.RWMutex
	rates map[string]float64
}

func (fx *fxExchange) rate(currency string, year int) float64 {
	if currency == baseCurrency {
		return 1.0
	}

	key := fmt.Sprintf("%s%d", currency, year)
	if rate, ok := fx.rates[key]; ok {
		return rate
	}

	// TODO: fetch from API
	fx.m.RLock()
	defer fx.m.RUnlock()
	fx.rates[key] = 1.0
	return 1.0
}

func findFiles() ([]string, error) {
	var files []string
	err := filepath.Walk(".", func(path string, info os.FileInfo, err error) error {
		// Only consider .csv files
		if filepath.Ext(path) == ".csv" {
			files = append(files, path)
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	return files, nil
}

func readFiles(files []string) <-chan brokerStatement {
	out := make(chan brokerStatement, len(files))
	go func() {
		for _, file := range files {
			// read file
			// transform data into a file report
			bs, err := readIbkrStatement(file)
			if err != nil {
				fmt.Println("Error reading file:", err)
				continue
			}
			bs.ID = rand.Intn(1000)
			out <- *bs
		}
		close(out)
	}()
	return out
}

type trade struct {
	// isin is the International Securities Identification Number. FIFO method needs to be partitioned by ISIN
	isin string
	// category is the type of trade: equity, bond, option, forex, crypto, etc.
	category string
	time     time.Time
	// currency is the currency of the trade
	currency string `validate:"required,iso4217"`
	// quantity is the number of shares, contracts, or units
	quantity float64
	// price is the price per share, contract, or unit
	price float64
}

func fifo(ts []trade, r rater) []pl {
	// Don't check it is sorted, assume it is not. Trades come from file out of order
	sort.Slice(ts, func(i, j int) bool {
		return ts[i].time.Before(ts[j].time)
	})

	// group by ISIN
	instrs := make(map[string][]trade)
	for _, t := range ts {
		instrs[t.isin] = append(instrs[t.isin], t)
	}

	// FIFO
	var pls []pl
	for _, ts := range instrs {
		purchase, sale := 0, 0
		for {
			// find next sale
			for sale < len(ts) && ts[sale].quantity >= 0 {
				sale++
			}

			if sale == len(ts) {
				break
			}

			// Find next purchase. Must have some quantity left to sell
			for purchase < sale && ts[purchase].quantity <= 0 {
				purchase++
			}

			if purchase == sale {
				break
			}

			pp := &ts[purchase]
			ss := &ts[sale]
			qtyToSell := math.Min(math.Abs(ss.quantity), math.Abs(pp.quantity))
			pp.quantity -= qtyToSell
			ss.quantity += qtyToSell

			// Discard is not taxable (2 years hold)
			if ss.time.After(pp.time.AddDate(2, 0, 0)) {
				continue
			}

			// Convert both currencies with the sale conversion year
			pl := pl{
				amount: qtyToSell * (ss.price*r.rate(ss.currency, ss.time.Year()) - pp.price*r.rate(pp.currency, ss.time.Year())),
				year:   ss.time.Year(),
				source: pp.isin[:2],
			}

			pls = append(pls, pl)
		}
	}

	return pls
}

func newLedger(statements <-chan brokerStatement) *ledger {
	// Store all in ledger to provide to tax report all at once
	l := &ledger{deductable: make(map[int]float64)}
	fx := &fxExchange{rates: make(map[string]float64), m: &sync.RWMutex{}}

	var trades []trade
	for stmt := range statements {
		l.tax = append(l.tax, plsFromTxs(stmt.tax, fx)...)
		l.profits = append(l.profits, plsFromTxs(stmt.fixedIncome, fx)...)
		trades = append(trades, stmt.trades...)

		for _, fee := range stmt.fees {
			if _, ok := l.deductable[fee.year]; !ok {
				l.deductable[fee.year] = 0
			}
			l.deductable[fee.year] += fx.rate(fee.currency, fee.year)
		}
	}

	// We have all the trades. Calculate taxable realized profits
	l.profits = append(l.profits, fifo(trades, fx)...)

	return l
}

type rater interface {
	rate(currency string, year int) float64
}

func plsFromTxs(txs []tx, r rater) []pl {
	pls := make([]pl, 0, len(txs))

	for _, tx := range txs {
		p := pl{amount: tx.amount * r.rate(tx.currency, tx.year), year: tx.year}
		if tx.isin != "" {
			p.source = tx.isin[:2]
		}
		pls = append(pls, p)
	}
	return pls
}

func newReport(l *ledger) report {
	r := make(report)

	// Apply withholding tax
	for _, pl := range l.tax {
		// Add year to report if not present
		if _, ok := r[pl.year]; !ok {
			r[pl.year] = &taxYear{year: pl.year, foreignIncome: make(map[string]*foreign)}
		}

		// Add foreign income for the source
		if _, ok := r[pl.year].foreignIncome[pl.source]; !ok {
			r[pl.year].foreignIncome[pl.source] = &foreign{}
		}

		r[pl.year].foreignIncome[pl.source].taxPaid += math.Abs(pl.amount)
	}

	// Apply profits
	for _, pl := range l.profits {
		if _, ok := r[pl.year]; !ok {
			r[pl.year] = &taxYear{year: pl.year, foreignIncome: make(map[string]*foreign)}
		}

		// If this is a positive gain and tax was already paid at source, add the gain to the foreign income
		fi := r[pl.year].foreignIncome[pl.source]
		if pl.amount > 0 && fi != nil && fi.taxPaid > 0 {
			fi.gains += pl.amount
		} else {
			r[pl.year].realizedPL += pl.amount
		}
	}

	// Deduct fees from main income report only
	// Do not report a new year just for the fees
	for yr, amount := range l.deductable {
		if year, ok := r[yr]; ok {
			year.realizedPL -= math.Abs(amount)
		}
	}

	// Balance it out. Do not report income if negative
	// Remove year from report if no realized PL and no foreign income
	//for _, year := range r {
	//	if year.realizedPL <= 0 {
	//		year.realizedPL = 0
	//
	//		if len(year.foreignIncome) == 0 {
	//			delete(r, year.year)
	//		}
	//	}
	//}

	return r
}

type instrument struct {
	isin     string
	category string
}

// readIbkrStatement reads single csv IBKR file
// Filters csv lines by relevant sections and uses *ImportResults to send files to
func readIbkrStatement(filename string) (*brokerStatement, error) {
	file, err := os.Open(filename)
	if err != nil {
		fmt.Printf("cannot open file %s %v", filename, err)
	}
	defer file.Close()

	reader := csv.NewReader(bufio.NewReader(file))
	// Disable record length test in the CSV
	reader.FieldsPerRecord = -1
	// Allow quote in unquoted field
	reader.LazyQuotes = true

	// Not all csv lines are needed. Get new section handlers for each file
	sections := []string{"Financial Instrument Information", "Trades", "Dividends", "Withholding Tax", "Fees"}
	// header keeps the csv line used as a header for the section currently being read
	var header []string
	var rows []map[string]string
	isins := make(map[string]instrument)
	for {
		row, err := reader.Read()
		if err == io.EOF {
			break
		}

		if err != nil {
			continue
		}

		// Ignore if not a section we're interested in
		if !slices.Contains(sections, row[0]) {
			continue
		}

		// Update header if new section
		if row[1] == "Header" {
			header = row
			continue
		}

		// If this is financial isin information, add symbols to isins map
		// otherwise, map the line and store for later
		if row[0] == "Financial Instrument Information" {
			lm, err := mapIbkrLine(row, header)
			if err != nil {
				continue
			}
			for _, s := range strings.Split(strings.ReplaceAll(lm["Symbol"], " ", ""), ",") {
				isins[s] = instrument{isin: formatISIN(lm["Security ID"]), category: importCategory(lm["Asset Category"])}
			}
			continue
		}

		lm, err := mapIbkrLine(row, header)
		if err != nil || lm["Header"] != "Data" {
			continue
		}

		rows = append(rows, lm)
	}

	return bsFromRows(rows, isins)
}

func bsFromRows(rows []map[string]string, isins map[string]instrument) (*brokerStatement, error) {
	bs := &brokerStatement{}

	for _, row := range rows {
		// All types have a currency
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

			bs.trades = append(bs.trades, trade{
				isin:     isins[row["Symbol"]].isin,
				category: isins[row["Symbol"]].category,
				time:     *t,
				currency: currency,
				quantity: amountFromString(row["Quantity"]),
				price:    amountFromString(row["T. Price"]),
			})

			bs.fees = append(bs.fees, tx{
				category: isins[row["Symbol"]].category,
				currency: currency,
				amount:   amountFromString(row["Comm/Fee"]),
				year:     t.Year(),
			})

			continue
		}

		// All other sections only need year as time
		if row["Date"] == "" {
			continue
		}

		if section == "Fees" {
			bs.fees = append(bs.fees, tx{
				currency: currency,
				amount:   amountFromString(row["Amount"]),
				year:     yearFromDate(row["Date"]),
			})

			continue
		}

		// Dividends and withholding tax have the same structure and need to get a symbol from the description
		symbol, err := symbolFromDescription(row["Description"])
		if err != nil {
			continue
		}

		tx := tx{
			isin:     isins[symbol].isin,
			category: isins[symbol].category,
			currency: currency,
			amount:   amountFromString(row["Amount"]),
			year:     yearFromDate(row["Date"]),
		}

		if section == "Dividends" {
			bs.fixedIncome = append(bs.fixedIncome, tx)
		} else {
			bs.tax = append(bs.tax, tx)
		}
	}

	return bs, nil
}

// symbolFromDescription extracts a symbol from IBKR csv dividend lines
// TODO Check for ISINs
func symbolFromDescription(d string) (string, error) {
	if d == "" {
		return "", errors.New("empty input")
	}

	// This is a dividend or withholding tax
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

// yearFromDate extracts a year from IBKR csv date field
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

// amountFromString formats number strings to float64 type
func amountFromString(s string) float64 {
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

// timeFromExact extracts time.Time from IBKR csv time field
func timeFromExact(t string) (*time.Time, error) {
	timeStr := strings.Join(strings.Split(t, ","), "")
	tm, err := time.Parse("2006-01-02 15:04:05", timeStr)
	if err != nil {
		return nil, errors.New("could not parse time")
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

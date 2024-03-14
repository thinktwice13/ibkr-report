package v2

import (
	"fmt"
	"math"
	"math/rand"
	"os"
	"path/filepath"
	"runtime"
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

type fileReport struct {
	ID          int
	trades      []trade
	fixedIncome []any
	tax         []any
	fees        []any
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
				fmt.Printf("  %s: %.2f\n", source, f.gains)
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

func readFiles(files []string) <-chan fileReport {
	out := make(chan fileReport, len(files))
	go func() {
		for _, file := range files {
			// read file
			// transform data into a file report
			fmt.Println("Reading file...", file)
			out <- fileReport{ID: rand.Intn(100)}
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

func fifo(ts []trade) []pl {
	// Don't check is sorted, assume it is not as file

	// FIFO

	// Reduce to taxable converted profits

	return []pl{}
}

func newLedger(reports <-chan fileReport) *ledger {
	// Store all in ledger to provide to tax report all at once
	l := &ledger{}
	fx := &fxExchange{rates: make(map[string]float64), m: &sync.RWMutex{}}

	var trades []trade
	for r := range reports {
		fmt.Printf("Processing report %d\n", r.ID)
		l.tax = append(l.tax, reportTax(r.tax, fx)...)
		l.profits = append(l.profits, dividends(r.fixedIncome, fx)...)
		l.deductable = deductable(r.trades, fx)
		trades = append(trades, r.trades...)
	}

	// We have all the trades. Calculate taxable realized profits
	l.profits = append(l.profits, fifo(trades)...)

	return l
}

type rater interface {
	rate(currency string, year int) float64
}

func deductable(trades []trade, r rater) map[int]float64 {
	return nil
}

func dividends(income []any, r rater) []pl {
	return []pl{}
}

func reportTax(tax []any, r rater) []pl {
	return []pl{}
}

func fees(fees []any, r rater) []pl {
	return []pl{}
}

func newReport(l *ledger) report {
	r := make(report)

	// Apply withholding tax
	for _, pl := range l.tax {
		if _, ok := r[pl.year]; !ok {
			r[pl.year] = &taxYear{year: pl.year, foreignIncome: make(map[string]*foreign)}
		}

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

		// realized PL can be both positive and negative
		r[pl.year].realizedPL += pl.amount
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
	for _, year := range r {
		if year.realizedPL < 0 {
			year.realizedPL = 0

			if len(year.foreignIncome) == 0 {
				delete(r, year.year)
			}
		}
	}

	return r
}

package main

import (
	"bufio"
	"errors"
	"fmt"
	"ibkr-report/broker"
	"ibkr-report/fx"
	"ibkr-report/ibkr"
	"log"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

func main() {
	t := time.Now()
	defer func() {
		fmt.Println("Finished in", time.Since(t))
	}()

	r := newReport(newLedger(readFiles(findFiles())))
	if err := writeFile(r.toRows()); err != nil {
		log.Fatalf("Error writing report: %v\n", err)
	}
}

// foreign is a representation of capital gains and Tax paid at foreign source in a single Year
type foreign struct {
	// gains is the total foreign income received
	gains float64
	// taxPaid is the total Tax paid at the foreign source
	taxPaid float64
}

// taxYear represents a single year of taxable income to be reported
type taxYear struct {
	year int
	// currency is the currency of the Tax year
	// this is to track the Croatian HRK to EUR currency change in 2023
	currency string
	// realizedPL is the taxable profit from Trades, dividends and interest
	// matches JOPPD form main input
	realizedPL float64
	// foreignIncome serves the entries in INO-DOH form, accounting for income Tax was fully or partially paid at the foreign source
	foreignIncome map[string]*foreign
}

type report map[int]*taxYear

type pl struct {
	amount float64
	source string
	year   int
}

// ledger collects all broker data into a single structure to be reported on.
// it groups profits and losses, discards non-taxable profits and calculates deductible expenses. It also runs a FIFO strategy on trades to calculate taxable trading profits
// TODO ledger should not be concerned with the filtering and FIFO strategy. It should only collect data and let another component handle it
type ledger struct {
	tax, profits []pl
	deductible   map[int]float64
}

// findFiles looks for .csv files in the current directory tree, while avoiding duplicates
func findFiles() <-chan string {
	files := make(map[string]struct{})
	ch := make(chan string, 10)
	go func() {
		defer close(ch)
		err := filepath.Walk(".", func(path string, info os.FileInfo, err error) error {
			if filepath.Ext(path) != ".csv" {
				return nil
			}
			if _, ok := files[path]; !ok {
				files[path] = struct{}{}
				ch <- path
			}
			return nil
		})
		if err != nil {
			fmt.Println("Error finding files:", err)
		}
	}()

	return ch
}

// readFiles creates a Statement for each provided file
func readFiles(files <-chan string) <-chan *broker.Statement {
	out := make(chan *broker.Statement, len(files))

	wg := &sync.WaitGroup{}
	workers := runtime.NumCPU() / 2
	wg.Add(workers)
	// worker pool
	for i := 0; i < workers; i++ {
		go func() {
			defer wg.Done()
			for file := range files {
				// read file
				// transform data into a file report
				// TODO Factory of broker.Reader to include other brokers. Problem: Files are broker-specific and need to be sorted before processing
				bs, err := ibkr.Read(file)
				if err != nil {
					fmt.Println("Error reading file:", err)
					continue
				}
				out <- bs
			}
		}()
	}

	go func() {
		wg.Wait()
		close(out)
	}()
	return out
}

func fifo(ts []broker.Trade, r fx.Rater) []pl {
	var pls []pl
	for _, ts := range tradesByISIN(ts) {
		purchase, sale := 0, 0
		for {
			// find next sale
			for sale < len(ts) && ts[sale].Quantity >= 0 {
				sale++
			}

			if sale == len(ts) {
				break
			}

			// Find matching purchase. Must be before the sale and have some quantity left
			for purchase < sale && ts[purchase].Quantity <= 0 {
				purchase++
			}

			if purchase == sale {
				break
			}

			if pl, taxable := profitFromTrades(&ts[purchase], &ts[sale], r); taxable {
				pls = append(pls, pl)
			}
		}
	}

	return pls
}

// tradesByISIN maps trades by ISIN
func tradesByISIN(ts []broker.Trade) map[string][]broker.Trade {
	sort.Slice(ts, func(i, j int) bool {
		return ts[i].Time.Before(ts[j].Time)
	})
	grouped := make(map[string][]broker.Trade)
	for _, t := range ts {
		grouped[t.ISIN] = append(grouped[t.ISIN], t)
	}
	return grouped
}

// profitFromTrades, returns the pl from a single purchase and sale Trade, as well as a bool indicating if the Trade was taxable
func profitFromTrades(purchase, sale *broker.Trade, r fx.Rater) (pl, bool) {
	qtyToSell := math.Min(math.Abs(sale.Quantity), math.Abs(purchase.Quantity))
	purchase.Quantity -= qtyToSell
	sale.Quantity += qtyToSell

	return pl{
		amount: qtyToSell * (sale.Price*r.Rate(sale.Currency, sale.Time.Year()) - purchase.Price*r.Rate(purchase.Currency, sale.Time.Year())),
		year:   sale.Time.Year(),
		source: purchase.ISIN[:2],
	}, sale.Time.Before(purchase.Time.AddDate(2, 0, 0))
}

func newLedger(statements <-chan *broker.Statement) *ledger {
	// Store all in ledger to provide to Tax report all at once
	l := &ledger{deductible: make(map[int]float64)}
	rtr := fx.New()
	var trades []broker.Trade
	for stmt := range statements {
		l.tax = append(l.tax, profitsFromTransactions(stmt.Tax, rtr)...)
		l.profits = append(l.profits, profitsFromTransactions(stmt.FixedIncome, rtr)...)
		trades = append(trades, stmt.Trades...)
		for _, fee := range stmt.Fees {
			if _, ok := l.deductible[fee.Year]; !ok {
				l.deductible[fee.Year] = 0
			}
			l.deductible[fee.Year] += rtr.Rate(fee.Currency, fee.Year)
		}
	}

	// We have all the Trades. Calculate taxable realized profits
	l.profits = append(l.profits, fifo(trades, rtr)...)

	return l
}

func profitsFromTransactions(txs []broker.Tx, r fx.Rater) []pl {
	pls := make([]pl, 0, len(txs))

	for _, tx := range txs {
		rate := r.Rate(tx.Currency, tx.Year)
		p := pl{amount: tx.Amount * rate, year: tx.Year}
		if tx.ISIN != "" {
			p.source = tx.ISIN[:2]
		}
		pls = append(pls, p)
	}

	return pls
}

func newReport(l *ledger) report {
	r := make(report)
	r.withWitholdingTax(l.tax)
	r.withProfits(l.profits)
	r.withDeductibles(l.deductible)
	return r
}

func (r report) toRows() [][]string {
	data := make([][]string, 0, len(r))
	for _, year := range r {
		yr := strconv.Itoa(year.year)
		ccy := "EUR"
		if year.year < 2023 {
			ccy = "HRK"
		}
		data = append(data, []string{yr, ccy, "JOPPD", fmt.Sprintf("%.2f", math.Max(0, year.realizedPL)), "", ""})
		for source, f := range year.foreignIncome {
			data = append(data, []string{yr, ccy, "INO-DOH", fmt.Sprintf("%.2f", f.gains), source, fmt.Sprintf("%.2f", f.taxPaid)})
		}
	}

	// sort by Year, then report type, then source
	// JOPPD before INO-DOH
	sort.Slice(data, func(i, j int) bool {
		if data[i][0] == data[j][0] {
			if data[i][1] == data[j][1] {
				return data[i][4] < data[j][4]
			}
			return data[i][1] > data[j][1]
		}
		return data[i][0] < data[j][0]
	})

	// With header
	return append([][]string{{"Godina", "Valuta", "Izvješće", "Dobit", "Izvor prihoda", "Plaćeni porez"}}, data...)
}

func (r report) withWitholdingTax(tax []pl) {
	for _, pl := range tax {
		// Add Year to report if not present
		if _, ok := r[pl.year]; !ok {
			ccy := "EUR"
			if pl.year < 2023 {
				ccy = "HRK"
			}
			r[pl.year] = &taxYear{year: pl.year, foreignIncome: make(map[string]*foreign), currency: ccy}
		}

		// Add foreign income for the source
		if _, ok := r[pl.year].foreignIncome[pl.source]; !ok {
			r[pl.year].foreignIncome[pl.source] = &foreign{}
		}

		r[pl.year].foreignIncome[pl.source].taxPaid += math.Abs(pl.amount)
	}
}

func (r report) withProfits(profits []pl) {
	for _, pl := range profits {
		if _, ok := r[pl.year]; !ok {
			ccy := "EUR"
			if pl.year < 2023 {
				ccy = "HRK"
			}
			r[pl.year] = &taxYear{year: pl.year, foreignIncome: make(map[string]*foreign), currency: ccy}
		}

		// If this is a profit and Tax was paid at source, add it to foreign income
		fi := r[pl.year].foreignIncome[pl.source]
		if pl.amount > 0 && fi != nil && fi.taxPaid > 0 {
			fi.gains += pl.amount
		} else {
			r[pl.year].realizedPL += pl.amount
		}
	}
}

func (r report) withDeductibles(deductible map[int]float64) {
	for yr, amount := range deductible {
		if year, ok := r[yr]; ok {
			year.realizedPL -= math.Abs(amount)
		}
	}

	// Balance it out. Do not report income if negative
	// Remove Year from report if no realized PL and no foreign income
	for _, year := range r {
		if year.realizedPL <= 0 {
			year.realizedPL = 0

			if len(year.foreignIncome) == 0 {
				delete(r, year.year)
			}
		}
	}
}

func writeFile(data [][]string) (err error) {
	// Calculate column widths
	widths := colWidths(data)

	// Print tab-separated to file
	file, err := os.Create("report.txt")
	if err != nil {
		return
	}
	defer func() {
		if fErr := file.Close(); fErr != nil {
			err = errors.Join(err, fErr)
		}
	}()

	w := bufio.NewWriter(file)
	defer func() {
		if fErr := w.Flush(); fErr != nil {
			err = errors.Join(err, fErr)
		}
	}()
	for _, row := range data {
		for i, cell := range row {
			// Right-align 'Dobit' and 'Plaćeni porez' columns
			if i == 3 || i == 5 {
				cell = strings.Repeat(" ", widths[i]-len(cell)) + cell
			}
			_, err = fmt.Fprintf(w, "%-*s", widths[i]+2, cell)
			if err != nil {
				return
			}
		}
		err = w.WriteByte('\n')
		if err != nil {
			return
		}
	}

	return
}

func colWidths(data [][]string) []int {
	widths := make([]int, len(data[0]))
	for _, row := range data {
		for i, cell := range row {
			if len(cell) > widths[i] {
				widths[i] = len(cell)
			}
		}
	}
	return widths
}

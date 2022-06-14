package main

import (
	"fmt"
	"github.com/xuri/excelize/v2"
	"regexp"
	"strconv"
	"time"
)

// readXls read custom spreadsheet investments tracker
func readXls(filename string, ir *ImportResults) {
	f, err := excelize.OpenFile(filename)
	if err != nil {
		fmt.Println(err)
	}

	defer func() {
		// Close the spreadsheet
		if err := f.Close(); err != nil {
			fmt.Println(err)
		}
	}()

	for name, handle := range xlsSheets() {
		handle(mapXlsRows(name, f), ir)
	}
}

type sheetLineHandler func([]map[string]string, *ImportResults)
type xlsSheetHandlers map[string]sheetLineHandler

// xlsSheets returns a map of handlers mapped by sheet (event type)
func xlsSheets() xlsSheetHandlers {
	return map[string]sheetLineHandler{
		"Dividends":       handleXlsDividends,
		"Trades":          handleXlsTrades,
		"Withholding Tax": handleXlsWithholdingTax,
		"Fees":            handleXlsFees,
	}
}

// handleXlsDividends handles spreadsheet tracker dividend sheet lines
func handleXlsDividends(lines []map[string]string, ir *ImportResults) {
	for _, lm := range lines {
		if lm["Year"] == "" {
			continue
		}

		yr := yearFromDate(lm["Year"])
		symbols := symbolsFromCell(lm["Asset"])
		if len(symbols) > 1 {
			ir.AddInstrumentInfo(symbols, "")
		}
		ir.AddDividend(symbols[0], lm["Currency"], yr, amountFromString(lm["Amount"]), false)
	}
}

// handleXlsTrades handles spreadsheet tracker trade sheet lines
func handleXlsTrades(lines []map[string]string, ir *ImportResults) {
	for _, lm := range lines {
		if lm["Time"] == "" {
			continue
		}

		symbols := symbolsFromCell(lm["Asset"])
		if len(symbols) > 1 {
			ir.AddInstrumentInfo(symbols, "")
		}
		timeField := lm["Time"]
		if timeField == "" {
			continue
		}

		time, err := xlsTimeFromCall(timeField)
		if err != nil {
			fmt.Println(err)
			continue
		}
		ir.AddTrade(symbols[0], lm["Currency"], time, amountFromString(lm["Quantity"]), amountFromString(lm["Price"]), amountFromString(lm["Fee"]))
	}
}

// handleXlsWithholdingTax handles spreadsheet tracker withholding tax sheet lines
// TODO Consider merging with dividends sheet
func handleXlsWithholdingTax(lines []map[string]string, ir *ImportResults) {
	for _, lm := range lines {
		if lm["Year"] == "" {
			continue
		}

		yr := yearFromDate(lm["Year"])
		symbols := symbolsFromCell(lm["Asset"])
		if len(symbols) > 1 {
			ir.AddInstrumentInfo(symbols, "")
		}
		ir.AddDividend(symbols[0], lm["Currency"], yr, amountFromString(lm["Amount"]), true)
	}
}

// handleXlsFees handles spreadsheet tracker fees sheet lines
func handleXlsFees(lines []map[string]string, ir *ImportResults) {
	for _, lm := range lines {
		if lm["Year"] == "" {
			continue
		}

		yr := yearFromDate(lm["Year"])
		ir.AddFee(lm["Currency"], amountFromString(lm["Amount"]), yr)
	}
}

func mapXlsRows(sheet string, f *excelize.File) []map[string]string {
	rows, err := f.GetRows(sheet, excelize.Options{RawCellValue: true})

	// Ignore error when sheet not found or not enough data rows
	if err != nil || len(rows) < 2 {
		return nil
	}

	lines := make([]map[string]string, 0, len(rows)-1)
	header := rows[0]
	for i := 1; i < len(rows); i++ {
		e := rows[i]

		lm := make(map[string]string, len(e))
		for pos, field := range header {
			if len(e) <= pos {
				continue
			}
			lm[field] = e[pos]
		}

		lines = append(lines, lm)
	}
	return lines
}

// xlsTimeFromCall extracts time.Time from spreadsheet time-formatted cell
func xlsTimeFromCall(t string) (*time.Time, error) {
	excelDate, err := strconv.ParseFloat(t, 64)
	if err != nil {
		return nil, err
	}
	excelTime, err := excelize.ExcelDateToTime(excelDate, false)
	if err != nil {
		return nil, err
	}
	return &excelTime, nil
}

// symbolsFromCell extracts instrument ticker or ISIN from a spreadsheet cell
// Assumes multiple symbols are separated by non-alphanumeric chars
func symbolsFromCell(s string) []string {
	re := regexp.MustCompile(`\w+`)
	matches := re.FindAllString(s, -1)
	var symbols []string
	for _, m := range matches {
		if len(m) > 8 && len(m) < 13 {
			// Potentially ISIN
			m = formatISIN(m)
		}
		symbols = append(symbols, m)
	}
	return symbols
}

package main

import (
	"fmt"
	"github.com/xuri/excelize/v2"
	"log"
	"math"
	"os"
	"path/filepath"
	"strconv"
)

// Report uses excelize.File and implements RowWriter interface needed by the reports to be written
type Report struct {
	f        *excelize.File
	filename string
}

func (r *Report) WriteRows(sheet string, rows [][]interface{}) error {
	r.f.NewSheet(sheet)

	for i := range rows {
		row := &rows[i]
		err := r.f.SetSheetRow(sheet, "A"+strconv.Itoa(i+1), row)
		if err != nil {
			return err
		}
	}

	cols := genColumns(len(rows[0]))
	lowerRight := cols[len(cols)-1] + strconv.Itoa(len(rows))
	err := r.f.AddTable(sheet, "A1", lowerRight, tableOptions())

	if err != nil {
		fmt.Printf("Error: %v\n", err)
	}
	return nil
}

func (r *Report) Save() error {
	r.f.DeleteSheet("Sheet1")
	err := r.f.SaveAs(filepath.Join(os.Getenv("PWD"), r.filename+".xlsx"))
	if err != nil {
		fmt.Printf("Error: %v\n", err)
	}
	fmt.Println(r.filename+".xlsx", "created")
	return nil
}

func NewReport(filename string) *Report {
	return &Report{f: excelize.NewFile(), filename: filename}
}

// RoundDec rounds a float number to provided number of decimal places
func RoundDec(v float64, places int) float64 {
	f := math.Pow(10, float64(places))
	return math.Round(v*f) / f
}

func writeReport(r Reporter, rw RowWriter) {
	err := r.WriteTo(rw)
	if err != nil {
		log.Fatalf("Error: %v\n", err)
	}
}

func createXlsTemplate() {
	filename := "Portfolio Tracker.xlsx"
	fpath := filepath.Join(os.Getenv("PWD"), filename)
	var err error

	_, err = os.Stat(fpath)
	if err == nil {
		return
	}

	f := excelize.NewFile()
	f.NewSheet("Trades")
	err = f.SetSheetRow("Trades", "A1", &[]interface{}{
		"Broker",
		"Asset",
		"Asset Category",
		"Currency",
		"Time",
		"Quantity",
		"Price",
		"Fee",
	})
	if err != nil {
		fmt.Printf("Error: %v\n", err)
	}

	err = f.AddTable("Trades", "A1", "H1001", tableOptions())

	if err != nil {
		fmt.Printf("Error: %v\n", err)
	}

	// TODO Sample data
	// f.SetSheetRow("Trades", "A2", &[]interface{}{
	// 	"khbliyubl",
	// 	"jlhblyb",
	// 	"EUR",
	// 	time.Now(),
	// 	-22.789,
	// 	574.89,
	// 	-4.2,
	// })

	f.NewSheet("Dividends")
	err = f.SetSheetRow("Dividends", "A1", &[]interface{}{
		"Broker",
		"Asset",
		"Asset Category",
		"Currency",
		"Year",
		"Amount",
	})
	if err != nil {
		fmt.Printf("Error: %v\n", err)
	}
	err = f.AddTable("Dividends", "A1", "F1001", tableOptions())

	if err != nil {
		fmt.Printf("Error: %v\n", err)
	}

	f.NewSheet("Withholding Tax")
	err = f.SetSheetRow("Withholding Tax", "A1", &[]interface{}{
		"Broker",
		"Asset",
		"Asset Category",
		"Currency",
		"Year",
		"Amount",
	})
	if err != nil {
		fmt.Printf("Error: %v\n", err)
	}
	err = f.AddTable("Withholding Tax", "A1", "F1001", tableOptions())

	if err != nil {
		fmt.Printf("Error: %v\n", err)
	}

	f.NewSheet("Fees")
	err = f.SetSheetRow("Fees", "A1", &[]interface{}{
		"Currency",
		"Year",
		"Amount",
		"Note",
	})
	if err != nil {
		fmt.Printf("Error: %v\n", err)
	}
	err = f.AddTable("Fees", "A1", "D1001", tableOptions())

	if err != nil {
		fmt.Printf("Error: %v\n", err)
	}

	f.DeleteSheet("Sheet1")
	err = f.SaveAs(fpath)
	if err != nil {
		fmt.Println("error creating tracker template")
	}
}

func tableOptions() string {
	return `{
        "table_name": "table",
		"table_style": "TableStyleMedium2",
        "show_first_column": true,
        "show_last_column": false,
        "show_row_stripes": false,
        "show_column_stripes": false
    }`
}

// genColumns generates a slice of strings representing spreadsheet column letters up to a provided size
func genColumns(n int) []string {
	a := 65
	z := 90
	maxLen := z - a + 1
	if n < maxLen {
		maxLen = n
	}
	AZ := make([]string, 0, maxLen)
	for c := a; c < a+maxLen; c++ {
		AZ = append(AZ, fmt.Sprintf("%s", string(rune(c))))
	}

	if n <= z-a+1 {
		return AZ
	}

	// Copy template slice and append A-Z template to prefix
	prefix := AZ
	for i := 0; i < n; i++ {
		for _, suffix := range AZ {
			prefix = append(prefix, prefix[i]+suffix)
		}
	}

	return prefix
}

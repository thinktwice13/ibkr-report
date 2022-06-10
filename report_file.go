package main

import (
	"fmt"
	"github.com/xuri/excelize/v2"
	"math"
	"strconv"
)

// Report extends excelize.File type with custom WriteTo method to implement RowWriter interface needed by the reports to be written
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
			fmt.Println(err)
			return err
		}
	}

	cols := genColumns(len(rows[0]))
	lowerRight := cols[len(cols)-1] + strconv.Itoa(len(rows))
	err := r.f.AddTable(sheet, "A1", lowerRight, `{
        "table_name": "table",
		"table_style": "TableStyleMedium2",
        "show_first_column": true,
        "show_last_column": false,
        "show_row_stripes": false,
        "show_column_stripes": false
    }`)

	if err != nil {
		fmt.Println(err)
	}
	return nil
}

func (r *Report) Save() error {
	r.f.DeleteSheet("Sheet1")
	err := r.f.SaveAs(r.filename)
	if err != nil {
		return err
	}
	return nil
}

func NewReport(filename string) *Report {
	return &Report{f: excelize.NewFile(), filename: filename + ".xlsx"}
}

// RoundDec rounds a float number to provided number of decimal places
func RoundDec(v float64, places int) float64 {
	f := math.Pow(10, float64(places))
	return math.Round(v*f) / f
}

func writeReport(r Reporter, rw RowWriter) error {
	err := r.WriteTo(rw)
	if err != nil {
		return err
	}
	return nil
}

// genColumns generates a slice of strings representing spreadsheet column letters up to a provided size
func genColumns(n int) []string {
	a := 65
	z := 90
	length := z - a + 1
	if n < length {
		length = n
	}
	AZ := make([]string, 0, length)
	for c := 65; c < a+length; c++ {
		AZ = append(AZ, fmt.Sprintf("%s", string(c)))
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

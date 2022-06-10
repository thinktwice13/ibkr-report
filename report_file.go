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

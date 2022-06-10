package main

import (
	"fmt"
	"github.com/xuri/excelize/v2"
	"strconv"
)

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
	err := r.f.SaveAs(r.filename)
	if err != nil {
		return err
	}
	return nil
}

func NewReport(filename string) *Report {
	return &Report{f: excelize.NewFile(), filename: filename + ".xlsx"}
}

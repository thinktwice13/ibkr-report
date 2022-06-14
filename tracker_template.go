package main

import (
	"fmt"
	"github.com/xuri/excelize/v2"
	"os"
	"path/filepath"
	"time"
)

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
		"Currency",
		"Time",
		"Quantity",
		"Price",
		"Fee",
	})
	if err != nil {
		fmt.Println(err)
	}

	err = f.AddTable("Trades", "A1", "G1001", `{
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

	f.SetSheetRow("Trades", "A2", &[]interface{}{
		"khbliyubl",
		"jlhblyb",
		"EUR",
		time.Now(),
		-22.789,
		574.89,
		-4.2,
	})

	f.NewSheet("Dividends")
	err = f.SetSheetRow("Dividends", "A1", &[]interface{}{
		"Broker",
		"Asset",
		"Currency",
		"Year",
		"Amount",
	})
	if err != nil {
		fmt.Println(err)
	}

	f.NewSheet("Withholding Tax")
	err = f.SetSheetRow("Withholding Tax", "A1", &[]interface{}{
		"Broker",
		"Asset",
		"Currency",
		"Year",
		"Amount",
	})
	if err != nil {
		fmt.Println(err)
	}

	f.NewSheet("Fees")
	err = f.SetSheetRow("Fees", "A1", &[]interface{}{
		"Currency",
		"Year",
		"Amount",
		"Note",
	})
	if err != nil {
		fmt.Println(err)
	}

	f.DeleteSheet("Sheet1")
	err = f.SaveAs(fpath)
	if err != nil {
		fmt.Println("error creating tracker template")
	}
}

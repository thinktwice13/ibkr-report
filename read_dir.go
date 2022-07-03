package main

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// readDir reads all files in the current directory and returns a list of assets, fees and years and currencies
func readDir() *ImportResults {
	ir := NewImportResults()
	var wg sync.WaitGroup
	files := make(chan string, 10)
	for w := 1; w <= maxWorkers; w++ {
		wg.Add(1)
		go func(wg *sync.WaitGroup, csvs <-chan string, r *ImportResults) {
			for f := range csvs {
				if strings.HasPrefix(filepath.Ext(f), ".xls") {
					readXls(f, ir)
					continue
				}
				readStatement(f, ir)
			}

			wg.Done()
		}(&wg, files, ir)
	}
	findFiles(files)
	wg.Wait()
	return ir
}

// findFiles walks the current directory and looks for .csv files
func findFiles(csvs chan<- string) {
	m := make(map[string]bool)
	filepath.WalkDir(os.Getenv("PWD"), func(path string, d fs.DirEntry, err error) error {
		// Skip hidden directories
		if d.IsDir() && d.Name()[:1] == "." {
			return filepath.SkipDir
		}

		// Skip exact filenames already read
		if ok := m[d.Name()]; ok {
			return nil
		}

		// Only consider csv and xls* files, but skip exported spreadsheet
		ext := filepath.Ext(d.Name())
		if ext == ".csv" || strings.HasPrefix(ext, ".xls") {
			if d.Name() == "Portfolio Report.xlsx" {
				return nil
			}
			m[d.Name()] = true
			csvs <- path
		}
		return nil
	})
	close(csvs)
}

type key interface {
	string | float64 | int
}

// list returns a list of map keys if the key implements key interface
func list[T key](m map[T]bool) []T {
	var l []T
	for k := range m {
		l = append(l, k)
	}
	return l
}

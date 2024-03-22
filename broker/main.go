package broker

import (
	"errors"
	"path/filepath"
	"time"
)

var ErrNotRecognized = errors.New("statement not recognized")

// Tx is a catch-all transaction type
// in this version, it can represent all transaction types except for trades, which need to track quotes at a specific time (Year is not enough)
type Tx struct {
	ISIN               string
	Category, Currency string
	Amount             float64
	Year               int
}

type Trade struct {
	ISIN               string
	Time               time.Time
	Category, Currency string
	Quantity, Price    float64
}

// Statement is an envelope for all relevant broker data found in a single broker statement file
type Statement struct {
	Broker                 string
	Filename               string
	Trades                 []Trade
	FixedIncome, Tax, Fees []Tx
}

type StatementReader func(filename string) (*Statement, error)

type Reader struct {
	readers map[string][]StatementReader
	done    map[string]struct{}
}

func NewReader() *Reader {
	return &Reader{readers: make(map[string][]StatementReader), done: make(map[string]struct{})}
}

func (r *Reader) Register(ext string, readers ...StatementReader) error {
	if _, ok := r.readers[ext]; ok {
		return errors.New("extension already registered")
	}
	r.readers[ext] = append(r.readers[ext], readers...)

	return nil
}

func (r *Reader) Read(filename string) (*Statement, error) {
	// Has this file been read before?
	if _, ok := r.done[filename]; ok {
		return nil, nil
	}
	readers, ok := r.readers[filepath.Ext(filename)]
	if !ok {
		return nil, ErrNotRecognized
	}
	for _, read := range readers {
		stmt, err := read(filename)
		if err == nil {
			return stmt, nil
		}
		if errors.Is(err, ErrNotRecognized) {
			continue
		}
		return nil, err
	}

	return nil, ErrNotRecognized
}

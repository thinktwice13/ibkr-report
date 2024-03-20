package broker

import "time"

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

// Statement is an envelope for all relevant broker data from a single file.
// This approach chosen as a middle ground between managing one channel per data type and marshal+switch option
type Statement struct {
	Broker                 string
	Filename               string
	Trades                 []Trade
	FixedIncome, Tax, Fees []Tx
}

type Reader interface {
	Read(filename string) (Statement, error)
}

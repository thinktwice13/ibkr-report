package fx

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

type Exchange struct {
	grabRetries int
	rates       map[string]float64
	wg          *sync.WaitGroup
}

func (fx *Exchange) Rate(currency string, year int) float64 {
	baseCurrency := "EUR"
	if year < 2023 {
		baseCurrency = "HRK"
	}

	if currency == baseCurrency {
		return 1.0
	}

	key := fmt.Sprintf("%s%d", currency, year)
	// Check for the requested rate. If not found, wait until existing fetches are done
	// It is likely requested rate will be fetched by the ongoing lookup, so check again after waiting
	// If still not found, fetch it
	if rate, ok := fx.rates[key]; ok {
		return rate
	}
	fx.wg.Wait()
	if rate, ok := fx.rates[key]; ok {
		return rate
	}

	if err := fx.grabRates(year, currency, fx.wg); err != nil {
		log.Fatal(err)
	}
	fx.wg.Wait()
	return fx.rates[key]
}

type Rater interface {
	Rate(currency string, year int) float64
}

// amountFromString formats number strings to float64 type
func amountFromString(s string) float64 {
	if s == "" {
		return 0

	}
	// Remove all but numbers, commas and points
	re := regexp.MustCompile(`[0-9.,-]`)
	ss := strings.Join(re.FindAllString(s, -1), "")
	isNeg := ss[0] == '-'
	// Find all commas and points
	// If none found, return 0, print error
	signs := regexp.MustCompile(`[.,]`).FindAllString(ss, -1)
	if len(signs) == 0 {
		f, err := strconv.ParseFloat(ss, 64)
		if err != nil {
			fmt.Printf("could not convert %s to number", s)
			return 0
		}

		return f
	}

	// Use last sign as decimal separator and ignore others
	// Find idx and replace whatever sign was to a decimal point
	sign := signs[len(signs)-1]
	signIdx := strings.LastIndex(ss, sign)
	sign = "."
	left := regexp.MustCompile(`[0-9]`).FindAllString(ss[:signIdx], -1)
	right := ss[signIdx+1:]
	n, err := strconv.ParseFloat(strings.Join(append(left, []string{sign, right}...), ""), 64)
	if err != nil {
		fmt.Printf("could not convert %s to number", s)
		return 0
	}
	if isNeg {
		n = n * -1
	}
	return n
}

// url composes the fx exchange rate url for a given currency and year
// It accounts for the 2024 currency change and has a default set of currencies to get rates for, to avoid multiple fetches in common currencies
func url(currency string, year int) string {
	// Url base
	url := strings.Builder{}
	url.WriteString("https://api.hnb.hr/tecajn")
	// Year-specific version
	if year < 2023 {
		url.WriteString("/v2")
	} else {
		url.WriteString("-eur/v3")
	}
	// Date
	url.WriteString("?datum-primjene=")
	if year == time.Now().Year() {
		url.WriteString(time.Now().UTC().Format("2006-01-02"))
	} else {
		url.WriteString(strconv.Itoa(year) + "-12-31")
	}
	// Fetch for requested currencies plus a default set of common ones
	currencyIncluded := false
	for _, curr := range []string{"EUR", "USD", "GBP", "CHF", "CAD", "AUD", "JPY"} {
		url.WriteString("&valuta=" + curr)
		if curr == currency {
			currencyIncluded = true
		}
	}
	if !currencyIncluded {
		url.WriteString("&valuta=" + currency)
	}

	return url.String()
}

func (fx *Exchange) grabRates(year int, currency string, wg *sync.WaitGroup) error {
	if year <= 1900 {
		return errors.New("invalid year")
	}

	wg.Add(1)
	defer wg.Done()

	var resp ratesResponse
	var err error
	var response *http.Response
	for r := 0; r < fx.grabRetries; r++ {
		response, err = http.Get(url(currency, year))
		if err == nil {
			break
		}
		time.Sleep(time.Second)
	}
	if err != nil {
		return err
	}

	if response == nil {
		return errors.New("no response")
	}

	defer response.Body.Close()
	contents, err := io.ReadAll(response.Body)
	if err != nil {
		return errors.New("cannot read response")
	}

	err = json.Unmarshal(contents, &resp.Rates)
	if err != nil {
		fmt.Println("whoops:", err)
	}

	for _, r := range resp.Rates {
		storeKey := fmt.Sprintf("%s%d", r.Currency, year)
		fx.rates[storeKey] = amountFromString(r.Rate)
	}

	return nil
}

// rateResponse is the response from hnb.hr
type rateResponse struct {
	Currency string `json:"valuta"`
	Rate     string `json:"srednji_tecaj"`
}

type ratesResponse struct {
	Rates []rateResponse `json:"rates"`
}

func New() *Exchange {
	return &Exchange{grabRetries: 3, rates: make(map[string]float64), wg: new(sync.WaitGroup)}
}

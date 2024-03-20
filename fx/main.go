package fx

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type Exchange struct {
	// grabRetries is the number of times to retry fetching the rates
	// The HNB api is not the most reliable, so it is better to retry a few times
	grabRetries int
	// rates map a rate toa currency-year key (e.g. "EUR2023")
	// This is all that's needed for Croatian tax report as the tha rate used is always from Dec 31 of the requested year
	rates map[string]float64
}

// Rate returns the exchange rate for a given currency and year
func (fx *Exchange) Rate(currency string, year int) float64 {
	// Determine base currency for the year. Currency changed in 2023
	baseCurrency := "EUR"
	if year < 2023 {
		baseCurrency = "HRK"
	}
	if currency == baseCurrency {
		return 1.0
	}

	// Return appropriate rate. Fetch from HNB api if needed
	key := fmt.Sprintf("%s%d", currency, year)
	if rate, ok := fx.rates[key]; ok {
		return rate
	}
	if err := fx.grabRates(year, currency); err != nil {
		log.Fatal(err)
	}
	return fx.rates[key]
}

// Rater is an interface for the Rate method
type Rater interface {
	Rate(currency string, year int) float64
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

func (fx *Exchange) grabRates(year int, currency string) (err error) {
	if year <= 1900 {
		return errors.New("invalid year")
	}

	var resp hnbApiResponse
	var response *http.Response
	for r := 0; r < fx.grabRetries; r++ {
		response, err = http.Get(url(currency, year))
		if err == nil {
			break
		}
		time.Sleep(time.Second)
	}
	if err != nil {
		return
	}

	if response == nil {
		return errors.New("no response")
	}

	defer func() {
		if bErr := response.Body.Close(); bErr != nil {
			err = errors.Join(err, bErr)
		}
	}()
	contents, err := io.ReadAll(response.Body)
	if err != nil {
		return errors.New("error reading API response")
	}

	err = json.Unmarshal(contents, &resp.Rates)
	if err != nil {
		return
	}

	for _, r := range resp.Rates {
		storeKey := fmt.Sprintf("%s%d", r.Currency, year)
		rate, err := parseFloat(r.Rate)
		if err != nil {
			log.Println("could not convert rate", r.Rate, "to float")
		}
		fx.rates[storeKey] = rate
	}
	return
}

type hnbApiResponse struct {
	Rates []struct {
		Currency string `json:"valuta"`
		Rate     string `json:"srednji_tecaj"`
	} `json:"rates"`
}

func parseFloat(s string) (float64, error) {
	// Remove dots, replace commas with dot
	s = strings.ReplaceAll(s, ".", "")
	s = strings.ReplaceAll(s, ",", ".")
	// Convert to float
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, err
	}

	return f, nil
}

// New returns a new currency exchange rate provider, implementing the Rater interface
func New() *Exchange {
	return &Exchange{grabRetries: 3, rates: make(map[string]float64)}
}

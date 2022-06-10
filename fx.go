package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

type Rates map[int]map[string]float64

func (r Rates) Rate(ccy string, yr int) float64 {
	return r[yr][ccy] // TODO Check errors
}

func NewFxRates(currencies []string, years []int) (Rates, error) {
	r := make(Rates, len(years))
	var m sync.Mutex
	var wg sync.WaitGroup
	wg.Add(len(years))
	for _, y := range years {
		go func(r Rates, y int, m *sync.Mutex, wg *sync.WaitGroup) {
			defer wg.Done()
			rates, err := grabHRKRates(y, currencies, 3)
			if err != nil {
				fmt.Printf("failed getting exchange rates for year %d", y)
				return
			}
			m.Lock()
			defer m.Unlock()
			r[y] = rates
		}(r, y, &m, &wg)
	}
	wg.Wait()
	return r, nil
}

// TODO Other currencies https://ec.europa.eu/info/funding-tenders/procedures-guidelines-tenders/information-contractors-and-beneficiaries/exchange-rate-inforeuro_en
type rateResponse struct {
	Currency string `json:"Valuta"`
	Rate     string `json:"Srednji za devize"`
}

type ratesResponse struct {
	Rates []rateResponse `json:"rates"`
}

func grabHRKRates(year int, c []string, retries int) (map[string]float64, error) {
	if year <= 1900 {
		log.Fatal("Cannot get currency rates for a year before 1901")
	}

	// TODO Other currencies than HRK
	date := LastDateForYear(year)

	// url := fmt.Sprintf("https://api.hnb.hr/tecajn/v1?datum-od=%s&datum-do=%s", from.Format("2006-01-02"), to.Format("2006-01-02"))
	url := fmt.Sprintf("https://api.hnb.hr/tecajn/v1?datum=%s", date.Format("2006-01-02"))
	for _, curr := range c {
		url = url + "&valuta=" + curr
	}

	var resp ratesResponse
	response, err := http.Get(url)
	if err != nil && retries <= 0 {
		fmt.Println(err)
		// TODO If all retries failed, check Internet conn. Update message
		return nil, errors.New("cannot fetch rates")
	}

	if err != nil {
		time.Sleep(time.Second)
		retries--
		return grabHRKRates(year, c, retries)
	}

	defer response.Body.Close()
	contents, err := ioutil.ReadAll(response.Body)
	if err != nil {
		log.Fatalf("%s", err)
	}

	err3 := json.Unmarshal(contents, &resp.Rates)
	if err3 != nil {
		fmt.Println("whoops:", err3)
	}

	rm := make(map[string]float64, len(c))
	for _, r := range resp.Rates {
		rm[r.Currency] = formatApiRate(r.Rate)
	}

	return rm, nil
}

func LastDateForYear(y int) (d time.Time) {
	if y == time.Now().Year() {
		d = time.Now().UTC()
	} else {
		d = time.Date(y, 12, 31, 0, 0, 0, 0, time.UTC)
	}

	return
}

func formatApiRate(r string) float64 {
	s := strings.ReplaceAll(strings.ReplaceAll(r, ".", ""), ",", ".")

	if s == "" {
		log.Fatalf("Cannot create amount from %s", s)
	}
	s = strings.ReplaceAll(s, ",", "")
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		log.Printf("error parsing float from from %v", err)
	}
	return v
}

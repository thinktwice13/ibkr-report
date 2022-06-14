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

type Rates struct {
	l     *sync.Mutex
	rates map[int]map[string]float64
}

func (r *Rates) Rate(ccy string, yr int) float64 {
	return r.rates[yr][ccy] // Should not be any errors here. Throw if not fetched correctly
}

func (r *Rates) setRates(y int, rates map[string]float64) {
	r.l.Lock()
	defer r.l.Unlock()
	r.rates[y] = rates
}

// NewFxRates creates a new Rates struct by fetching currency exhange rates for provided years and currencies
// TODO Do not fetch in New
func NewFxRates(currencies []string, years []int) (*Rates, error) {
	r := &Rates{
		l:     new(sync.Mutex),
		rates: map[int]map[string]float64{},
	}
	var wg sync.WaitGroup
	workers := maxWorkers
	if len(years) < maxWorkers {
		workers = len(years)
	}
	wg.Add(workers)
	yrs := make(chan int)
	for w := 0; w < workers; w++ {
		go func(r *Rates, yrs <-chan int, wg *sync.WaitGroup) {
			defer wg.Done()
			for y := range yrs {
				rates, err := grabHRKRates(y, currencies, 3)
				if err != nil {
					log.Fatalln("failed getting currency exchange rates from hnb.hr. Please try again later")
				}
				r.setRates(y, rates)
			}
		}(r, yrs, &wg)
	}

	for _, y := range years {
		yrs <- y
	}

	close(yrs)
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

// grabHRKRates fetches HRK exchange rates for a list of currencies in a provided year from hnb.hr
func grabHRKRates(year int, c []string, retries int) (map[string]float64, error) {
	if year <= 1900 {
		log.Fatal("Cannot get currency rates for a year before 1901")
	}

	// TODO Other currencies than HRK
	date := lastDateForYear(year)

	// url := fmt.Sprintf("https://api.hnb.hr/tecajn/v1?datum-od=%s&datum-do=%s", from.Format("2006-01-02"), to.Format("2006-01-02"))
	baseUrl := "https://api.hnb.hr/tecajn/v1"
	url := fmt.Sprintf(baseUrl+"?datum=%s", date.Format("2006-01-02"))
	for _, curr := range c {
		url = url + "&valuta=" + curr
	}

	var resp ratesResponse
	var err error
	var response *http.Response
	for r := 0; r < retries; r++ {
		response, err = http.Get(url)
		if err != nil {
			time.Sleep(time.Second)
			continue
		}

		break
	}

	if response == nil {
		return nil, errors.New("cannot fetch rates")
	}

	defer response.Body.Close()
	contents, err := ioutil.ReadAll(response.Body)
	if err != nil {
		log.Fatalf("%s", err)
	}

	err = json.Unmarshal(contents, &resp.Rates)
	if err != nil {
		fmt.Println("whoops:", err)
	}

	rm := make(map[string]float64, len(c))
	for _, r := range resp.Rates {
		rm[r.Currency] = formatApiRate(r.Rate)
	}

	return rm, nil
}

// lastDateForYear calculates last day of the year for the input
// If year is current year, returns today
// Return time set to UTC
func lastDateForYear(y int) (d time.Time) {
	if y == time.Now().Year() {
		d = time.Now().UTC()
	} else {
		d = time.Date(y, 12, 31, 0, 0, 0, 0, time.UTC)
	}

	return
}

// formatApiRate formats the api response currency rate received from hnb.hr
func formatApiRate(r string) float64 {
	s := strings.ReplaceAll(strings.ReplaceAll(r, ".", ""), ",", ".")
	if s == "" {
		log.Fatalf("cannot create amount from %s", s)
	}
	s = strings.ReplaceAll(s, ",", "")
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		log.Printf("error parsing float from from %v", err)
	}
	return v
}

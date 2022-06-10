package fx

import (
	"fmt"
	"sync"
	"time"
)

type Rates map[int]map[string]float64

func (r Rates) Rate(ccy string, yr int) float64 {
	return r[yr][ccy] // TODO Check errors
}

func New(currencies []string, years []int) (Rates, error) {
	t := time.Now()
	r := make(Rates, len(years))

	var m sync.Mutex
	var wg sync.WaitGroup
	wg.Add(len(years))
	for _, y := range years {
		go func(r Rates, y int, m *sync.Mutex, wg *sync.WaitGroup) {
			defer wg.Done()
			rates, err := grabHRKRates(y, currencies, 3)
			if err != nil {
				fmt.Println(err)
				fmt.Printf("cannot get rates for year %d", y)
				return
			}
			m.Lock()
			defer m.Unlock()
			r[y] = rates
		}(r, y, &m, &wg)
	}
	wg.Wait()

	fmt.Println("Rates fetched in:", time.Since(t))
	return r, nil
}

func (r Rates) Print() {
	fmt.Println("Fx")
	for y := range r {
		for c := range r[y] {
			fmt.Printf("%d %s %g\n", y, c, r[y][c])
		}
	}
}

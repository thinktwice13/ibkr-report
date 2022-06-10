package main

import (
	"encoding/json"
	"fmt"
	"ibkr-report/fx"
	"log"
)

func main() {
	fmt.Println("Hello World")
	assets, _, years, currencies := readDir()

	rates, err := fx.New(currencies, years)
	if err != nil {
		log.Fatalln(err)
	}
	PrettyPrint(rates)

	// var ss []Asset
	for _, assetimport := range assets {
		PrettyPrint(assetimport)
	}

}

func PrettyPrint(a any) {
	s, _ := json.MarshalIndent(a, "", "\t")
	fmt.Println(string(s))
}

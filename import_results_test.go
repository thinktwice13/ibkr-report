package main

import (
	"testing"
)

func TestAssetBySymbols(t *testing.T) {
	a1 := &AssetImport{}
	a1.Symbols = []string{"A"}
	a2 := &AssetImport{}
	a2.Symbols = []string{"B", "C"}
	a3 := &AssetImport{}
	a3.Symbols = []string{"A", "B", "D"}

	assets := assets{}
	assets.bySymbols(a1.Symbols...)
	assets.bySymbols(a2.Symbols...)
	assets.bySymbols(a3.Symbols...)

	assetPtrs := make(map[*AssetImport]bool)
	for _, a := range assets {
		assetPtrs[a] = true
	}
	if len(assetPtrs) != 1 {
		t.Errorf("Expected 1 unique asset. Got %d", len(assetPtrs))
	}
}

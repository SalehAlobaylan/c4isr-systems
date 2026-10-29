//go:build ignore

// Run with go run ./scripts/ops/check_imagery_pair.go -before ... -after ...
// This offline scientific check grants no admission or publication authority.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"github.com/SalehAlobaylan/c4isr-systems/internal/imageryanalysis"
	"os"
)

func main() {
	before := flag.String("before", "", "retained before subset")
	after := flag.String("after", "", "retained after subset")
	out := flag.String("out", "", "result artifact")
	flag.Parse()
	load := func(path string) imageryanalysis.Tile {
		raw, e := os.ReadFile(path)
		if e != nil {
			panic(e)
		}
		v, e := imageryanalysis.Decode(raw)
		if e != nil {
			panic(e)
		}
		return v
	}
	a, b := load(*before), load(*after)
	result, e := imageryanalysis.Compare(context.Background(), a, b, imageryanalysis.Parameters{DeclineThreshold: .2, MinValidFraction: .5})
	if e != nil {
		fmt.Fprintln(os.Stderr, e)
		os.Exit(1)
	}
	raw, _ := json.Marshal(result)
	if *out != "" {
		if e = os.WriteFile(*out, raw, 0600); e != nil {
			panic(e)
		}
	}
	summary := map[string]any{"synthetic": result.Synthetic, "before": a.SceneID, "after": b.SceneID, "validCells": result.ValidCells, "maskedCells": result.MaskedCells, "declineCells": result.DeclineCells, "meanDelta": result.MeanDelta, "totalCells": len(a.Red), "clearFraction": float64(result.ValidCells) / float64(len(a.Red)), "algorithm": result.Algorithm, "limitations": result.Limitations}
	data, _ := json.Marshal(summary)
	fmt.Println(string(data))
}

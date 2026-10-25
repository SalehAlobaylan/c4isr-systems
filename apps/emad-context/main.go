// emad-context validates a bounded context bundle without a database or network.
package main

import (
	"encoding/json"
	"fmt"
	"github.com/SalehAlobaylan/c4isr-systems/internal/contextpackage"
	"io"
	"os"
	"time"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: emad-context <bundle.json>")
		os.Exit(2)
	}
	if err := validate(os.Args[1]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
func validate(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return err
	}
	if info.Size() > 12*1024*1024 {
		return fmt.Errorf("context bundle exceeds 12 MiB")
	}
	d := json.NewDecoder(io.LimitReader(f, 12*1024*1024+1))
	d.DisallowUnknownFields()
	var b contextpackage.Bundle
	if err = d.Decode(&b); err != nil {
		return err
	}
	var extra any
	if err = d.Decode(&extra); err != io.EOF {
		return fmt.Errorf("expected exactly one bundle")
	}
	if err = b.Validate(time.Now(), true); err != nil {
		return err
	}
	fmt.Printf("Verified %s: %d retained assets; live feeds are unnecessary.\n", b.ID, len(b.Assets))
	return nil
}

// Package jsoncanon provides a versioned identity representation for retained
// JSON values. It does not rewrite source bytes or round numbers to float64.
package jsoncanon

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"strings"
)

// Marshal canonicalizes JSON object order and exact decimal values. Numbers
// use a minimal integer coefficient and decimal exponent, without expanding
// arbitrarily large exponents or losing integer precision through float64.
func Marshal(value any) ([]byte, error) {
	raw, e := json.Marshal(value)
	if e != nil {
		return nil, e
	}
	return Canonical(raw)
}
func Canonical(raw []byte) ([]byte, error) {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	var value any
	if e := dec.Decode(&value); e != nil {
		return nil, e
	}
	if dec.Decode(new(any)) != io.EOF {
		return nil, fmt.Errorf("ambiguous JSON value")
	}
	var walk func(any) (any, error)
	walk = func(value any) (any, error) {
		switch v := value.(type) {
		case json.Number:
			return number(v)
		case []any:
			for i, child := range v {
				next, e := walk(child)
				if e != nil {
					return nil, e
				}
				v[i] = next
			}
		case map[string]any:
			for key, child := range v {
				next, e := walk(child)
				if e != nil {
					return nil, e
				}
				v[key] = next
			}
		}
		return value, nil
	}
	value, e := walk(value)
	if e != nil {
		return nil, e
	}
	return json.Marshal(value)
}
func number(in json.Number) (json.Number, error) {
	raw := strings.ToLower(in.String())
	sign := ""
	if strings.HasPrefix(raw, "-") {
		sign = "-"
		raw = raw[1:]
	}
	pieces := strings.SplitN(raw, "e", 2)
	coefficient := pieces[0]
	exponent := new(big.Int)
	if len(pieces) == 2 {
		if _, ok := exponent.SetString(pieces[1], 10); !ok {
			return "", fmt.Errorf("invalid decimal exponent")
		}
	}
	if dot := strings.IndexByte(coefficient, '.'); dot >= 0 {
		exponent.Sub(exponent, big.NewInt(int64(len(coefficient)-dot-1)))
		coefficient = coefficient[:dot] + coefficient[dot+1:]
	}
	coefficient = strings.TrimLeft(coefficient, "0")
	if coefficient == "" {
		return json.Number("0"), nil
	}
	trimmed := strings.TrimRight(coefficient, "0")
	exponent.Add(exponent, big.NewInt(int64(len(coefficient)-len(trimmed))))
	return json.Number(sign + trimmed + "e" + exponent.String()), nil
}

// Package kladr — KLADR code VO. Format: CCC-TT.RRR-TT.CCC-TT.DDD-TT.SSS
// (country, region, city, district, street); block 00.000 = level not set.
package kladr

import (
	"fmt"
	"regexp"
	"strings"
)

var format = regexp.MustCompile(`^\d{3}(-\d{2}\.\d{3}){4}$`)

const zeroBlock = "00.000"

type Level int

const (
	Country Level = iota
	Region
	City
	District
	Street
)

type Code struct{ raw string }

func Parse(raw string) (Code, error) {
	if !format.MatchString(raw) {
		return Code{}, fmt.Errorf("invalid KLADR code: %q", raw)
	}
	return Code{raw}, nil
}

func (c Code) String() string { return c.raw }

func (c Code) blocks() []string { return strings.Split(c.raw, "-") }

// Level — the deepest non-zero level of the code.
func (c Code) Level() Level {
	b := c.blocks()
	switch {
	case b[4] != zeroBlock:
		return Street
	case b[3] != zeroBlock:
		return District
	case b[2] != zeroBlock:
		return City
	case b[1] != zeroBlock:
		return Region
	default:
		return Country
	}
}

// Prefix — code without trailing 00.000 blocks.
func (c Code) Prefix() string {
	b := c.blocks()
	last := 0
	for i := 1; i < len(b); i++ {
		if b[i] != zeroBlock {
			last = i
		}
	}
	return strings.Join(b[:last+1], "-")
}

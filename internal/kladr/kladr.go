// Package kladr — VO кода КЛАДР. Формат: CCC-TT.RRR-TT.CCC-TT.DDD-TT.SSS
// (страна, регион, город, район, улица); блок 00.000 = уровень не задан.
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

// Level — самый глубокий ненулевой уровень кода.
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

// Prefix — код без хвостовых блоков 00.000.
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

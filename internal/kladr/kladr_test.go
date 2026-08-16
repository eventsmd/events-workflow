package kladr

import "testing"

func TestParse_Invalid(t *testing.T) {
	for _, raw := range []string{"", "123", "123-45.678", "abc-00.000-00.000-00.000-00.000"} {
		if _, err := Parse(raw); err == nil {
			t.Fatalf("expected error for %q", raw)
		}
	}
}

func TestLevel(t *testing.T) {
	cases := map[string]Level{
		"123-00.000-00.000-00.000-00.000": Country,
		"123-01.001-00.000-00.000-00.000": Region,
		"123-01.001-02.002-00.000-00.000": City,
		"123-01.001-02.002-03.003-00.000": District,
		"123-01.001-02.002-03.003-04.004": Street,
	}
	for raw, want := range cases {
		c, err := Parse(raw)
		if err != nil {
			t.Fatalf("%s: %v", raw, err)
		}
		if c.Level() != want {
			t.Fatalf("%s: got %v want %v", raw, c.Level(), want)
		}
	}
}

func TestPrefix_TrimsTrailingZeroBlocks(t *testing.T) {
	c, _ := Parse("123-01.001-02.002-00.000-00.000")
	if got := c.Prefix(); got != "123-01.001-02.002" {
		t.Fatal(got)
	}
	c, _ = Parse("123-00.000-00.000-00.000-00.000")
	if got := c.Prefix(); got != "123" {
		t.Fatal(got)
	}
	// a 'hole' in the middle: non-empty street after zero district — prefix up to street
	c, _ = Parse("123-01.001-00.000-00.000-04.004")
	if got := c.Prefix(); got != "123-01.001-00.000-00.000-04.004" {
		t.Fatal(got)
	}
}

// Additional test cases from Java KladrCodeTest.java

func TestRaw_ReturnsOriginalCode(t *testing.T) {
	code, _ := Parse("001-03.004-05.048-00.000-71.152")
	if got := code.String(); got != "001-03.004-05.048-00.000-71.152" {
		t.Fatalf("got %q, want %q", got, "001-03.004-05.048-00.000-71.152")
	}
}

func TestLevel_StreetLevelWhenAllBlocksNonZero(t *testing.T) {
	code, _ := Parse("001-03.004-05.048-00.000-71.152")
	if got := code.Level(); got != Street {
		t.Fatalf("got %v, want %v", got, Street)
	}
}

func TestLevel_DistrictLevelWhenStreetIsZeroAndDistrictNonZero(t *testing.T) {
	code, _ := Parse("001-03.004-05.048-02.007-00.000")
	if got := code.Level(); got != District {
		t.Fatalf("got %v, want %v", got, District)
	}
}

func TestLevel_CityLevelWhenDistrictAndStreetAreZero(t *testing.T) {
	code, _ := Parse("001-03.004-05.048-00.000-00.000")
	if got := code.Level(); got != City {
		t.Fatalf("got %v, want %v", got, City)
	}
}

func TestLevel_RegionLevelWhenOnlyCountryAndRegionNonZero(t *testing.T) {
	code, _ := Parse("001-05.001-00.000-00.000-00.000")
	if got := code.Level(); got != Region {
		t.Fatalf("got %v, want %v", got, Region)
	}
}

func TestLevel_CountryLevelWhenAllBlocksAfterCountryAreZero(t *testing.T) {
	code, _ := Parse("001-00.000-00.000-00.000-00.000")
	if got := code.Level(); got != Country {
		t.Fatalf("got %v, want %v", got, Country)
	}
}

func TestPrefix_ParameterizedCases(t *testing.T) {
	cases := []struct {
		raw    string
		expect string
	}{
		{"001-03.004-05.048-00.000-71.152", "001-03.004-05.048-00.000-71.152"},
		{"001-03.004-05.048-02.007-00.000", "001-03.004-05.048-02.007"},
		{"001-03.004-05.048-00.000-00.000", "001-03.004-05.048"},
		{"001-05.001-00.000-00.000-00.000", "001-05.001"},
		{"001-00.000-00.000-00.000-00.000", "001"},
	}
	for _, tc := range cases {
		code, _ := Parse(tc.raw)
		if got := code.Prefix(); got != tc.expect {
			t.Fatalf("Parse(%q).Prefix() = %q, want %q", tc.raw, got, tc.expect)
		}
	}
}

func TestValidCodesAreParsedSuccessfully(t *testing.T) {
	validCodes := []string{
		"001-03.004-05.048-00.000-71.152",
		"001-00.000-00.000-00.000-00.000",
		"999-99.999-99.999-99.999-99.999",
		"000-00.000-00.000-00.000-00.000",
	}
	for _, raw := range validCodes {
		if _, err := Parse(raw); err != nil {
			t.Fatalf("Parse(%q) should not error, got: %v", raw, err)
		}
	}
}

func TestInvalidCodesThrowError(t *testing.T) {
	invalidCodes := []string{
		"",
		"abc",
		"001-03.004-05.048-00.000",
		"001-03.004-05.048-00.000-71.1521",
		"001_03.004-05.048-00.000-71.152",
		"01-03.004-05.048-00.000-71.152",
		"0011-03.004-05.048-00.000-71.152",
		"001-3.004-05.048-00.000-71.152",
		"001-03.04-05.048-00.000-71.152",
	}
	for _, raw := range invalidCodes {
		if _, err := Parse(raw); err == nil {
			t.Fatalf("Parse(%q) should error", raw)
		}
	}
}

func TestEquality_EqualCodesAreEqual(t *testing.T) {
	a, _ := Parse("001-03.004-05.048-00.000-71.152")
	b, _ := Parse("001-03.004-05.048-00.000-71.152")
	if a != b {
		t.Fatalf("codes should be equal")
	}
}

func TestEquality_DifferentCodesAreNotEqual(t *testing.T) {
	a, _ := Parse("001-03.004-05.048-00.000-71.152")
	b, _ := Parse("001-03.004-05.048-00.000-00.000")
	if a == b {
		t.Fatalf("codes should not be equal")
	}
}

func TestToString_ContainsRawCode(t *testing.T) {
	code, _ := Parse("001-03.004-05.048-00.000-71.152")
	str := code.String()
	if str != "001-03.004-05.048-00.000-71.152" {
		t.Fatalf("toString should contain raw code, got: %s", str)
	}
}

package domain

import (
	"fmt"
	"strings"
	"time"
)

const (
	secondsLayout = "2006-01-02T15:04:05"
	minuteLayout  = "2006-01-02T15:04"
)

var parseLayouts = []string{
	"2006-01-02T15:04:05.999999999",
	secondsLayout,
	minuteLayout,
}

func parseLocal(s string) (time.Time, error) {
	for _, l := range parseLayouts {
		if t, err := time.Parse(l, s); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("cannot parse local datetime %q", s)
}

// LocalDateTime — java.time.LocalDateTime: without timezone; on serialization
// seconds are omitted if they are zero (Jackson ISO_LOCAL_DATE_TIME behavior).
type LocalDateTime struct{ time.Time }

func (t LocalDateTime) String() string {
	if t.Second() == 0 && t.Nanosecond() == 0 {
		return t.Format(minuteLayout)
	}
	return t.Format(secondsLayout)
}

func (t LocalDateTime) MarshalJSON() ([]byte, error) {
	return []byte(`"` + t.String() + `"`), nil
}

func (t *LocalDateTime) UnmarshalJSON(b []byte) error {
	s := strings.Trim(string(b), `"`)
	if s == "null" || s == "" {
		return nil
	}
	parsed, err := parseLocal(s)
	if err != nil {
		return err
	}
	t.Time = parsed
	return nil
}

// MinuteDateTime — fields with @JsonFormat(pattern = "yyyy-MM-dd'T'HH:mm").
type MinuteDateTime struct{ time.Time }

func (t MinuteDateTime) MarshalJSON() ([]byte, error) {
	return []byte(`"` + t.Format(minuteLayout) + `"`), nil
}

func (t *MinuteDateTime) UnmarshalJSON(b []byte) error {
	s := strings.Trim(string(b), `"`)
	if s == "null" || s == "" {
		return nil
	}
	parsed, err := parseLocal(s)
	if err != nil {
		return err
	}
	t.Time = parsed
	return nil
}

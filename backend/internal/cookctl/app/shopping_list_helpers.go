package app

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// parseISODate validates a YYYY-MM-DD date string and returns its time value.
func parseISODate(field, raw string) (time.Time, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return time.Time{}, fmt.Errorf("%s is required", field)
	}
	parsed, err := time.Parse(isoDateLayout, trimmed)
	if err != nil {
		return time.Time{}, fmt.Errorf("%s must be YYYY-MM-DD", field)
	}
	return parsed, nil
}

// parseOptionalFloat parses a float if the string is not empty.
func parseOptionalFloat(value string) (*float64, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil, nil
	}
	parsed, err := strconv.ParseFloat(trimmed, 64)
	if err != nil {
		return nil, err
	}
	return &parsed, nil
}

// csvStrings accumulates repeatable string flags.
type csvStrings struct {
	values []string
}

func (c *csvStrings) Set(value string) error {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil
	}
	c.values = append(c.values, trimmed)
	return nil
}

func (c *csvStrings) String() string {
	return strings.Join(c.values, ",")
}

func (c *csvStrings) Values() []string {
	return append([]string{}, c.values...)
}

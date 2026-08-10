package ledger

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"time"
)

var ErrInvalidEntryTime = errors.New("invalid entry time")

var rfc3339EntryTime = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(?:\.\d+)?(?:Z|[+-]\d{2}:\d{2})$`)

// EntryTimePrecision distinguishes a calendar date from an instant with a
// UTC offset. The distinction is retained when an entry is exported.
type EntryTimePrecision uint8

const (
	EntryDate EntryTimePrecision = iota + 1
	EntryDateTime
)

// EntryTime is the accounting time of a journal entry. Its fields are private
// so values must be created through the validating constructors or parser.
type EntryTime struct {
	value     time.Time
	precision EntryTimePrecision
}

// ParseEntryTime parses either YYYY-MM-DD or a timezone-qualified RFC 3339
// timestamp. A timestamp without Z or a numeric UTC offset is rejected.
func ParseEntryTime(text string) (EntryTime, error) {
	if len(text) == len(time.DateOnly) {
		value, err := time.Parse(time.DateOnly, text)
		if err != nil {
			return EntryTime{}, fmt.Errorf("%w: %q is not a valid date", ErrInvalidEntryTime, text)
		}
		return EntryTime{value: value, precision: EntryDate}, nil
	}

	if !rfc3339EntryTime.MatchString(text) || !validRFC3339Offset(text) {
		return EntryTime{}, fmt.Errorf("%w: %q is not a timezone-qualified RFC 3339 timestamp", ErrInvalidEntryTime, text)
	}
	value, err := time.Parse(time.RFC3339Nano, text)
	if err != nil {
		return EntryTime{}, fmt.Errorf("%w: %q is not a valid RFC 3339 timestamp", ErrInvalidEntryTime, text)
	}
	return NewEntryDateTime(value)
}

// NewEntryDate creates a date-only entry time.
func NewEntryDate(year int, month time.Month, day int) (EntryTime, error) {
	value := time.Date(year, month, day, 0, 0, 0, 0, time.UTC)
	if year < 1 || year > 9999 || value.Year() != year || value.Month() != month || value.Day() != day {
		return EntryTime{}, fmt.Errorf("%w: invalid date", ErrInvalidEntryTime)
	}
	return EntryTime{value: value, precision: EntryDate}, nil
}

// NewEntryDateTime creates a timestamp entry time while retaining its UTC
// offset. RFC 3339 represents offsets at minute precision.
func NewEntryDateTime(value time.Time) (EntryTime, error) {
	if value.IsZero() || value.Year() < 1 || value.Year() > 9999 {
		return EntryTime{}, fmt.Errorf("%w: invalid timestamp", ErrInvalidEntryTime)
	}
	_, offset := value.Zone()
	const largestRFC3339Offset = 23*time.Hour + 59*time.Minute
	if offset%int(time.Minute/time.Second) != 0 || offset < -int(largestRFC3339Offset/time.Second) || offset > int(largestRFC3339Offset/time.Second) {
		return EntryTime{}, fmt.Errorf("%w: UTC offset is not representable in RFC 3339", ErrInvalidEntryTime)
	}
	return EntryTime{value: value, precision: EntryDateTime}, nil
}

// Precision reports whether the value is a date or a timestamp.
func (t EntryTime) Precision() EntryTimePrecision {
	return t.precision
}

// Time returns the underlying value. For EntryDate values it is midnight UTC.
func (t EntryTime) Time() time.Time {
	return t.value
}

// String returns the canonical, precision-preserving representation.
func (t EntryTime) String() string {
	switch t.precision {
	case EntryDate:
		return t.value.Format(time.DateOnly)
	case EntryDateTime:
		return t.value.Format(time.RFC3339Nano)
	default:
		return ""
	}
}

func (t EntryTime) validate() error {
	switch t.precision {
	case EntryDate:
		if t.value.IsZero() || t.value.Year() < 1 || t.value.Year() > 9999 {
			return fmt.Errorf("%w: invalid date", ErrInvalidEntryTime)
		}
		return nil
	case EntryDateTime:
		_, err := NewEntryDateTime(t.value)
		return err
	default:
		return fmt.Errorf("%w: unknown precision", ErrInvalidEntryTime)
	}
}

func validRFC3339Offset(text string) bool {
	if text[len(text)-1] == 'Z' {
		return true
	}
	hour, _ := strconv.Atoi(text[len(text)-5 : len(text)-3])
	minute, _ := strconv.Atoi(text[len(text)-2:])
	return hour <= 23 && minute <= 59
}

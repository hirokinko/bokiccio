package ledger

import (
	"errors"
	"testing"
	"time"
)

func TestParseEntryTime(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		text      string
		precision EntryTimePrecision
		want      string
	}{
		{name: "date", text: "2026-08-10", precision: EntryDate, want: "2026-08-10"},
		{name: "UTC timestamp", text: "2026-08-10T05:30:00Z", precision: EntryDateTime, want: "2026-08-10T05:30:00Z"},
		{name: "offset timestamp", text: "2026-08-10T14:30:00+09:00", precision: EntryDateTime, want: "2026-08-10T14:30:00+09:00"},
		{name: "fractional timestamp", text: "2026-08-10T14:30:00.123400+09:00", precision: EntryDateTime, want: "2026-08-10T14:30:00.1234+09:00"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			got, err := ParseEntryTime(test.text)
			if err != nil {
				t.Fatalf("ParseEntryTime(%q) error = %v", test.text, err)
			}
			if got.Precision() != test.precision {
				t.Errorf("Precision() = %v, want %v", got.Precision(), test.precision)
			}
			if got.String() != test.want {
				t.Errorf("String() = %q, want %q", got.String(), test.want)
			}
		})
	}
}

func TestParseEntryTimeRejectsInvalidValues(t *testing.T) {
	t.Parallel()
	for _, text := range []string{
		"",
		"2026-02-30",
		"2026-08-10T14:30:00",
		"2026-08-10 14:30:00+09:00",
		"2026-08-10T14:30:00+0900",
		"2026-08-10T14:30:00,5+09:00",
		"2026-08-10T14:30:00+24:00",
		"2026-08-10T14:30:00+09:60",
	} {
		t.Run(text, func(t *testing.T) {
			t.Parallel()
			if _, err := ParseEntryTime(text); !errors.Is(err, ErrInvalidEntryTime) {
				t.Fatalf("ParseEntryTime(%q) error = %v, want %v", text, err, ErrInvalidEntryTime)
			}
		})
	}
}

func TestNewEntryDate(t *testing.T) {
	t.Parallel()
	got, err := NewEntryDate(2024, time.February, 29)
	if err != nil {
		t.Fatalf("NewEntryDate() error = %v", err)
	}
	if got.Precision() != EntryDate || got.String() != "2024-02-29" {
		t.Fatalf("NewEntryDate() = (%v, %q), want (%v, %q)", got.Precision(), got.String(), EntryDate, "2024-02-29")
	}
	if _, err := NewEntryDate(2026, time.February, 29); !errors.Is(err, ErrInvalidEntryTime) {
		t.Fatalf("NewEntryDate(invalid) error = %v, want %v", err, ErrInvalidEntryTime)
	}
}

func TestNewEntryDateTimeRejectsUnrepresentableOffset(t *testing.T) {
	t.Parallel()
	value := time.Date(2026, time.August, 10, 14, 30, 0, 0, time.FixedZone("seconds", 9*60*60+1))
	if _, err := NewEntryDateTime(value); !errors.Is(err, ErrInvalidEntryTime) {
		t.Fatalf("NewEntryDateTime() error = %v, want %v", err, ErrInvalidEntryTime)
	}
}

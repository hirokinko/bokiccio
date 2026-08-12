package tacklerfmt

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/hirokinko/bokiccio/internal/ledger"
)

func TestParseSubsetEntries(t *testing.T) {
	t.Parallel()
	input := []byte("2026-08-10  'サンプル店舗\n    ; source: sample\n    費用:食費 120.50 JPY ; lunch\n    資産:現金\n\n2026-08-10T14:30:00+09:00  'サンプル薬局\n    費用:日用品 +544 JPY\n    資産:銀行口座:普通 -544.00 JPY\n")

	entries, err := Parse(input)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("len(Parse()) = %d, want 2", len(entries))
	}
	first := entries[0]
	if first.Date.String() != "2026-08-10" || first.Description != "サンプル店舗" || len(first.Comments) != 1 || first.Comments[0] != "source: sample" {
		t.Fatalf("first entry = %+v", first)
	}
	if got := first.Postings[0].Amount.Value.String(); got != "120.50" {
		t.Fatalf("amount = %q, want 120.50", got)
	}
	if first.Postings[1].Amount != nil {
		t.Fatalf("final amount = %+v, want omitted", first.Postings[1].Amount)
	}
	if got := entries[1].Date.String(); got != "2026-08-10T14:30:00+09:00" {
		t.Fatalf("timestamp = %q", got)
	}
	if got := entries[1].Postings[0].Amount.Value.String(); got != "544" {
		t.Fatalf("signed plus amount normalized to %q, want 544", got)
	}
}

func TestParseRoundTripsExportedSubset(t *testing.T) {
	t.Parallel()
	entries := goldenEntries(t)
	output, err := Export(entries, Options{OmittedAmounts: PreserveOmitted})
	if err != nil {
		t.Fatalf("Export() error = %v", err)
	}
	parsed, err := Parse(output)
	if err != nil {
		t.Fatalf("Parse(Export()) error = %v", err)
	}
	if len(parsed) != len(entries) {
		t.Fatalf("len(parsed) = %d, want %d", len(parsed), len(entries))
	}
	for index := range entries {
		if parsed[index].Date.String() != entries[index].Date.String() ||
			parsed[index].Description != entries[index].Description ||
			len(parsed[index].Postings) != len(entries[index].Postings) {
			t.Fatalf("parsed[%d] = %+v, want %+v", index, parsed[index], entries[index])
		}
	}
}

func TestParseTotalPriceValuePosition(t *testing.T) {
	t.Parallel()
	input := []byte("2026-08-10  '匿名投資取引\n    資産:投資信託  350 口 = 675 JPY     ; sample\n    資産:購入予定\n")

	entries, err := Parse(input)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	posting := entries[0].Postings[0]
	if posting.Amount == nil || posting.Amount.Value.String() != "350" || posting.Amount.Commodity != "口" {
		t.Fatalf("posting amount = %+v", posting.Amount)
	}
	if posting.TotalPrice == nil || posting.TotalPrice.Value.String() != "675" || posting.TotalPrice.Commodity != "JPY" {
		t.Fatalf("posting total price = %+v", posting.TotalPrice)
	}
	output, err := Export(entries, Options{OmittedAmounts: PreserveOmitted})
	if err != nil {
		t.Fatalf("Export() error = %v", err)
	}
	if !bytes.Equal(output, input) {
		t.Fatalf("Export(Parse()) = %q, want %q", output, input)
	}
}

func TestParseRejectsOutsideSubset(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		input string
		err   error
	}{
		{name: "posting before header", input: "    費用:食費 120 JPY\n", err: ErrInvalidInput},
		{name: "empty", input: "\n\n", err: ErrInvalidInput},
		{name: "timestamp without offset", input: "2026-08-10T14:30:00  'local\n    費用:食費 120 JPY\n    資産:現金\n", err: ledger.ErrInvalidEntryTime},
		{name: "metadata", input: "2026-08-10  'metadata\n    # uuid: sample\n    費用:食費 120 JPY\n    資産:現金\n", err: ErrInvalidInput},
		{name: "posting without commodity", input: "2026-08-10  'missing commodity\n    費用:食費 120\n    資産:現金\n", err: ErrInvalidInput},
		{name: "unit price", input: "2026-08-10  'unit price\n    費用:食費 2 USD @ 150 JPY\n    資産:現金\n", err: ErrInvalidInput},
		{name: "non-final omission", input: "2026-08-10  'non final\n    費用:食費\n    資産:現金 -120 JPY\n", err: ledger.ErrInvalidOmitted},
		{name: "unbalanced", input: "2026-08-10  'unbalanced\n    費用:食費 120 JPY\n    資産:現金 -100 JPY\n", err: ledger.ErrUnbalancedEntry},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := Parse([]byte(test.input))
			if !errors.Is(err, test.err) {
				t.Fatalf("Parse() error = %v, want %v", err, test.err)
			}
		})
	}
}

func TestParseErrorMessageIsPrivateSafe(t *testing.T) {
	t.Parallel()
	input := []byte("2026-08-10  'private shop\n    秘密:口座 private-amount JPY ; private comment\n    資産:現金\n")

	_, err := Parse(input)
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("Parse() error = %v, want %v", err, ErrInvalidInput)
	}
	got := SafeDiagnostic(err)
	for _, private := range []string{"private shop", "秘密:口座", "private-amount", "private comment"} {
		if strings.Contains(got, private) {
			t.Fatalf("SafeDiagnostic() = %q contains private text %q", got, private)
		}
	}
	for _, want := range []string{"invalid Tackler input", "line 2", "posting amount"} {
		if !strings.Contains(got, want) {
			t.Fatalf("SafeDiagnostic() = %q, want to contain %q", got, want)
		}
	}
}

func TestParseDetailedDiagnosticIncludesCause(t *testing.T) {
	t.Parallel()
	input := []byte("2026-08-10  'private shop\n    秘密:口座 private-amount JPY ; private comment\n    資産:現金\n")

	_, err := Parse(input)
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("Parse() error = %v, want %v", err, ErrInvalidInput)
	}
	got := DetailedDiagnostic(err)
	for _, want := range []string{"invalid Tackler input", "line 2", "posting amount", "private-amount"} {
		if !strings.Contains(got, want) {
			t.Fatalf("DetailedDiagnostic() = %q, want to contain %q", got, want)
		}
	}
}

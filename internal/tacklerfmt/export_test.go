package tacklerfmt

import (
	"bytes"
	"errors"
	"os"
	"testing"
	"time"

	"bokiccio/internal/ledger"
)

func TestExportExplicitEntriesGolden(t *testing.T) {
	t.Parallel()
	entries := goldenEntries(t)

	got, err := Export(entries, Options{OmittedAmounts: PreserveOmitted})
	if err != nil {
		t.Fatalf("Export() error = %v", err)
	}
	want, err := os.ReadFile("testdata/explicit_entries.golden")
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("Export() mismatch\n--- got ---\n%s--- want ---\n%s", got, want)
	}
}

func TestExportIsDeterministic(t *testing.T) {
	t.Parallel()
	entries := goldenEntries(t)
	options := Options{OmittedAmounts: FillOmitted}

	first, err := Export(entries, options)
	if err != nil {
		t.Fatalf("first Export() error = %v", err)
	}
	second, err := Export(entries, options)
	if err != nil {
		t.Fatalf("second Export() error = %v", err)
	}
	if !bytes.Equal(first, second) {
		t.Fatal("Export() returned different bytes for the same input")
	}
}

func TestExportEmptyInput(t *testing.T) {
	t.Parallel()
	got, err := Export(nil, Options{OmittedAmounts: PreserveOmitted})
	if err != nil {
		t.Fatalf("Export() error = %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("Export() = %q, want empty output", got)
	}
}

func TestExportOmittedFinalAmount(t *testing.T) {
	t.Parallel()
	entry := omittedEntry(t)
	tests := []struct {
		name     string
		mode     OmittedAmountMode
		wantLast string
	}{
		{name: "preserve", mode: PreserveOmitted, wantLast: "    資産:現金     ; 自動均衡\n"},
		{name: "fill", mode: FillOmitted, wantLast: "    資産:現金  -107.50 JPY     ; 自動均衡\n"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := Export([]ledger.JournalEntry{entry}, Options{OmittedAmounts: test.mode})
			if err != nil {
				t.Fatalf("Export() error = %v", err)
			}
			if !bytes.HasSuffix(got, []byte(test.wantLast)) {
				t.Fatalf("Export() = %q, want suffix %q", got, test.wantLast)
			}
			if entry.Postings[2].Amount != nil {
				t.Fatal("Export() mutated the omitted posting")
			}
		})
	}
}

func TestExportRejectsInvalidInputWithoutPartialOutput(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		entries func(*testing.T) []ledger.JournalEntry
		options Options
		err     error
	}{
		{
			name:    "missing option",
			entries: goldenEntries,
			options: Options{},
			err:     ErrInvalidOptions,
		},
		{
			name: "invalid second entry",
			entries: func(t *testing.T) []ledger.JournalEntry {
				entries := goldenEntries(t)
				entries[1].Description = ""
				return entries
			},
			options: Options{OmittedAmounts: PreserveOmitted},
			err:     ledger.ErrInvalidEntry,
		},
		{
			name: "non-final omission",
			entries: func(t *testing.T) []ledger.JournalEntry {
				entries := goldenEntries(t)
				entries[0].Postings[0].Amount = nil
				return entries
			},
			options: Options{OmittedAmounts: PreserveOmitted},
			err:     ledger.ErrInvalidOmitted,
		},
		{
			name: "multiple omissions",
			entries: func(t *testing.T) []ledger.JournalEntry {
				entry := omittedEntry(t)
				entry.Postings[1].Amount = nil
				return []ledger.JournalEntry{entry}
			},
			options: Options{OmittedAmounts: FillOmitted},
			err:     ledger.ErrInvalidOmitted,
		},
		{
			name: "commodity mismatch",
			entries: func(t *testing.T) []ledger.JournalEntry {
				entries := goldenEntries(t)
				entries[0].Postings[1].Amount.Commodity = "USD"
				return entries
			},
			options: Options{OmittedAmounts: FillOmitted},
			err:     ledger.ErrCommodityMismatch,
		},
		{
			name: "unbalanced explicit amounts",
			entries: func(t *testing.T) []ledger.JournalEntry {
				entries := goldenEntries(t)
				entries[0].Postings[1].Amount = amount(t, "-206", "JPY")
				return entries
			},
			options: Options{OmittedAmounts: FillOmitted},
			err:     ledger.ErrUnbalancedEntry,
		},
		{
			name: "inferred amount overflow",
			entries: func(t *testing.T) []ledger.JournalEntry {
				entry := omittedEntry(t)
				entry.Postings = []ledger.Posting{
					{Account: "費用:一時A", Amount: amount(t, "79228162514264337593543950335", "JPY")},
					{Account: "費用:一時B", Amount: amount(t, "1", "JPY")},
					{Account: "資産:現金"},
				}
				return []ledger.JournalEntry{entry}
			},
			options: Options{OmittedAmounts: FillOmitted},
			err:     ledger.ErrDecimalOverflow,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := Export(test.entries(t), test.options)
			if !errors.Is(err, test.err) {
				t.Fatalf("Export() error = %v, want %v", err, test.err)
			}
			if got != nil {
				t.Fatalf("Export() output = %q, want nil", got)
			}
		})
	}
}

func omittedEntry(t *testing.T) ledger.JournalEntry {
	t.Helper()
	return ledger.JournalEntry{
		Date:        time.Date(2026, time.August, 11, 0, 0, 0, 0, time.UTC),
		Description: "サンプル商店",
		Postings: []ledger.Posting{
			{Account: "費用:食費", Amount: amount(t, "100.00", "JPY")},
			{Account: "費用:日用品", Amount: amount(t, "7.5", "JPY")},
			{Account: "資産:現金", Comment: "自動均衡"},
		},
	}
}

func goldenEntries(t *testing.T) []ledger.JournalEntry {
	t.Helper()
	return []ledger.JournalEntry{
		{
			Date:        time.Date(2026, time.August, 9, 18, 30, 0, 0, time.FixedZone("JST", 9*60*60)),
			Description: "サンプル店舗",
			Comments:    []string{"source: receipt/sample-001.jpg"},
			Postings: []ledger.Posting{
				{
					Account: "費用:食費",
					Amount:  amount(t, "207", "JPY"),
					Comment: "商品A / 割引: クーポン -11 JPY",
				},
				{Account: "資産:現金", Amount: amount(t, "-207.00", "JPY")},
			},
		},
		{
			Date:        time.Date(2026, time.August, 10, 0, 0, 0, 0, time.UTC),
			Description: "サンプル薬局",
			Postings: []ledger.Posting{
				{
					Account: "費用:日用品",
					Amount:  amount(t, "544", "JPY"),
					Comment: "商品B / WARN: 支払金額を確認してください",
				},
				{Account: "資産:銀行口座:普通", Amount: amount(t, "-544", "JPY")},
			},
		},
	}
}

func amount(t *testing.T, value string, commodity ledger.Commodity) *ledger.Amount {
	t.Helper()
	decimal, err := ledger.ParseDecimal(value)
	if err != nil {
		t.Fatalf("ParseDecimal(%q) error = %v", value, err)
	}
	return &ledger.Amount{Value: decimal, Commodity: commodity}
}

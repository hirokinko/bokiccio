package ledger

import (
	"errors"
	"testing"
)

func TestValidateBalancedJournalEntry(t *testing.T) {
	t.Parallel()
	entry := validEntry(t)
	if err := Validate(entry); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}

func TestValidateAllowsFinalOmittedAmount(t *testing.T) {
	t.Parallel()
	entry := validEntry(t)
	entry.Postings[1].Amount = nil
	if err := Validate(entry); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}

func TestValidateRejectsInvalidEntries(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		change func(*JournalEntry)
		err    error
	}{
		{name: "zero date", change: func(e *JournalEntry) { e.Date = EntryTime{} }, err: ErrInvalidEntry},
		{name: "unknown date precision", change: func(e *JournalEntry) {
			e.Date.precision = EntryTimePrecision(255)
		}, err: ErrInvalidEntryTime},
		{name: "empty description", change: func(e *JournalEntry) { e.Description = "  " }, err: ErrInvalidEntry},
		{name: "description line break", change: func(e *JournalEntry) { e.Description = "shop\nnext" }, err: ErrInvalidEntry},
		{name: "entry comment line break", change: func(e *JournalEntry) { e.Comments = []string{"source: a\nb"} }, err: ErrInvalidEntry},
		{name: "posting shortage", change: func(e *JournalEntry) { e.Postings = e.Postings[:1] }, err: ErrInvalidEntry},
		{name: "empty account", change: func(e *JournalEntry) { e.Postings[0].Account = "" }, err: ErrInvalidAccount},
		{name: "invalid account", change: func(e *JournalEntry) { e.Postings[0].Account = "費用::食費" }, err: ErrInvalidAccount},
		{name: "invalid leading account", change: func(e *JournalEntry) { e.Postings[0].Account = "1費用:食費" }, err: ErrInvalidAccount},
		{name: "identifier outside tackler bmp range", change: func(e *JournalEntry) { e.Postings[0].Account = "費用:𐐀" }, err: ErrInvalidAccount},
		{name: "posting comment line break", change: func(e *JournalEntry) { e.Postings[0].Comment = "a\rb" }, err: ErrInvalidPosting},
		{name: "empty commodity", change: func(e *JournalEntry) { e.Postings[0].Amount.Commodity = "" }, err: ErrInvalidCommodity},
		{name: "invalid commodity", change: func(e *JournalEntry) { e.Postings[0].Amount.Commodity = "JP Y" }, err: ErrInvalidCommodity},
		{name: "commodity mismatch", change: func(e *JournalEntry) { e.Postings[1].Amount.Commodity = "USD" }, err: ErrCommodityMismatch},
		{name: "unbalanced", change: func(e *JournalEntry) { e.Postings[1].Amount.Value = mustDecimal(t, "-99.99") }, err: ErrUnbalancedEntry},
		{name: "non-final omission", change: func(e *JournalEntry) { e.Postings[0].Amount = nil }, err: ErrInvalidOmitted},
		{name: "multiple omissions", change: func(e *JournalEntry) {
			e.Postings = append(e.Postings, Posting{Account: "資産:現金"})
			e.Postings[1].Amount = nil
		}, err: ErrInvalidOmitted},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			entry := validEntry(t)
			test.change(&entry)
			if err := Validate(entry); !errors.Is(err, test.err) {
				t.Fatalf("Validate() error = %v, want %v", err, test.err)
			}
		})
	}
}

func TestValidateSupportsTacklerIdentifierSubset(t *testing.T) {
	t.Parallel()
	entry := validEntry(t)
	entry.Postings[0].Account = "資産:積立投資信託:松井証券:NISA:eMAXIS:Slim先進国株式インデックス（除く日本）"
	entry.Postings[1].Account = "費用:税金:消費税:税率8％"
	if err := Validate(entry); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}

func TestValidateReportsPostingIndex(t *testing.T) {
	t.Parallel()
	entry := validEntry(t)
	entry.Postings[1].Account = "資産::現金"
	err := Validate(entry)
	var postingErr *PostingValidationError
	if !errors.As(err, &postingErr) {
		t.Fatalf("Validate() error = %T, want *PostingValidationError", err)
	}
	if postingErr.Index != 1 || !errors.Is(err, ErrInvalidAccount) {
		t.Fatalf("Validate() error = %v, index=%d", err, postingErr.Index)
	}
}

func TestInferFinalAmount(t *testing.T) {
	t.Parallel()
	entry := validEntry(t)
	entry.Postings = []Posting{
		{Account: "費用:食費", Amount: &Amount{Value: mustDecimal(t, "100.00"), Commodity: "JPY"}},
		{Account: "費用:日用品", Amount: &Amount{Value: mustDecimal(t, "7.5"), Commodity: "JPY"}},
		{Account: "資産:現金"},
	}

	got, err := InferFinalAmount(entry)
	if err != nil {
		t.Fatalf("InferFinalAmount() error = %v", err)
	}
	if got.Value.String() != "-107.50" || got.Commodity != "JPY" {
		t.Fatalf("InferFinalAmount() = %s %s, want -107.50 JPY", got.Value.String(), got.Commodity)
	}
	if entry.Postings[2].Amount != nil {
		t.Fatal("InferFinalAmount() mutated the omitted posting")
	}
}

func TestInferFinalAmountAllowsTemporaryWideSum(t *testing.T) {
	t.Parallel()
	entry := validEntry(t)
	entry.Postings = []Posting{
		{Account: "資産:一時A", Amount: &Amount{Value: mustDecimal(t, "79228162514264337593543950335"), Commodity: "JPY"}},
		{Account: "資産:一時B", Amount: &Amount{Value: mustDecimal(t, "1"), Commodity: "JPY"}},
		{Account: "資産:一時C", Amount: &Amount{Value: mustDecimal(t, "-79228162514264337593543950335"), Commodity: "JPY"}},
		{Account: "資産:調整"},
	}

	got, err := InferFinalAmount(entry)
	if err != nil {
		t.Fatalf("InferFinalAmount() error = %v", err)
	}
	if got.Value.String() != "-1" {
		t.Fatalf("InferFinalAmount() = %s, want -1", got.Value.String())
	}
}

func TestInferFinalAmountRejectsExplicitFinalPosting(t *testing.T) {
	t.Parallel()
	if _, err := InferFinalAmount(validEntry(t)); !errors.Is(err, ErrNoOmittedAmount) {
		t.Fatalf("InferFinalAmount() error = %v, want %v", err, ErrNoOmittedAmount)
	}
}

func validEntry(t *testing.T) JournalEntry {
	t.Helper()
	return JournalEntry{
		Date:        mustEntryTime(t, "2026-08-09"),
		Description: "サンプル店舗",
		Comments:    []string{"source: receipt/example.jpg"},
		Postings: []Posting{
			{
				Account: "費用:食費",
				Amount:  &Amount{Value: mustDecimal(t, "100.00"), Commodity: "JPY"},
				Comment: "サンプル商品",
			},
			{
				Account: "資産:現金",
				Amount:  &Amount{Value: mustDecimal(t, "-100.0"), Commodity: "JPY"},
			},
		},
	}
}

func mustEntryTime(t *testing.T, text string) EntryTime {
	t.Helper()
	value, err := ParseEntryTime(text)
	if err != nil {
		t.Fatalf("ParseEntryTime(%q) error = %v", text, err)
	}
	return value
}

package ingest

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/hirokinko/bokiccio/internal/tacklerfmt"
)

func TestProcessMixedOutcomes(t *testing.T) {
	t.Parallel()
	file, err := os.Open("testdata/mixed-outcomes-v1.json")
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer file.Close()
	batch, err := Decode(file)
	if err != nil {
		t.Fatalf("Decode() error = %v", err)
	}

	result := Process(batch, nil)
	wantStatuses := []OutcomeStatus{OutcomeSuccess, OutcomeWarning, OutcomeError, OutcomeDuplicate}
	if len(result.Outcomes) != len(wantStatuses) {
		t.Fatalf("len(Outcomes) = %d, want %d", len(result.Outcomes), len(wantStatuses))
	}
	for index, want := range wantStatuses {
		outcome := result.Outcomes[index]
		if outcome.RecordIndex != index || outcome.Status != want || outcome.Identity != batch.Records[index].Identity {
			t.Errorf("outcome %d = %+v, want status %q", index, outcome, want)
		}
	}
	if len(result.Entries) != 2 {
		t.Fatalf("len(Entries) = %d, want 2", len(result.Entries))
	}
	if result.Outcomes[0].Entry == nil || result.Outcomes[1].Entry == nil || result.Outcomes[2].Entry != nil || result.Outcomes[3].Entry != nil {
		t.Fatal("outcome entries do not match accepted statuses")
	}

	warning := result.Outcomes[1].Diagnostics
	if len(warning) != 2 || warning[0].Code != "receipt.total_mismatch" || warning[0].Severity != SeverityWarning || warning[1].Code != "receipt.store_uncertain" {
		t.Fatalf("warning diagnostics = %+v", warning)
	}
	if warning[0].Identity != batch.Records[1].Identity || warning[0].FieldPath != "postings[0].amount" || warning[0].PostingIndex == nil || *warning[0].PostingIndex != 0 {
		t.Errorf("warning diagnostic location = %+v", warning[0])
	}
	if got := result.Entries[1].Postings[0].Comment; got != "サンプル商品 / WARN [receipt.total_mismatch]: 合計を確認してください" {
		t.Errorf("warning posting comment = %q", got)
	}
	if got := result.Entries[1].Comments[1]; got != "WARN [receipt.store_uncertain]: 店舗名を確認してください" {
		t.Errorf("warning transaction comment = %q", got)
	}

	domainError := result.Outcomes[2].Diagnostics
	if len(domainError) != 1 || domainError[0].Code != DiagnosticInvalidAccount || domainError[0].Severity != SeverityError {
		t.Fatalf("error diagnostics = %+v", domainError)
	}
	if domainError[0].FieldPath != "postings[0].account" || domainError[0].PostingIndex == nil || *domainError[0].PostingIndex != 0 {
		t.Errorf("error diagnostic location = %+v", domainError[0])
	}

	duplicate := result.Outcomes[3].Diagnostics
	if len(duplicate) != 1 || duplicate[0].Code != DiagnosticDuplicateInBatch || duplicate[0].Severity != SeverityInfo {
		t.Fatalf("duplicate diagnostics = %+v", duplicate)
	}
	if duplicate[0].Identity != batch.Records[3].Identity {
		t.Errorf("duplicate identity = %+v", duplicate[0].Identity)
	}

	exported, err := tacklerfmt.Export(result.Entries, tacklerfmt.Options{OmittedAmounts: tacklerfmt.PreserveOmitted})
	if err != nil {
		t.Fatalf("Export() error = %v", err)
	}
	for _, text := range []string{
		"    ; source: receipts/sample-success.jpg\n",
		"    ; source: receipts/sample-warning.jpg\n",
		"WARN [receipt.total_mismatch]: 合計を確認してください",
		"    ; WARN [receipt.store_uncertain]: 店舗名を確認してください\n",
	} {
		if !bytes.Contains(exported, []byte(text)) {
			t.Errorf("exported journal does not contain %q:\n%s", text, exported)
		}
	}
	if bytes.Contains(exported, []byte("sample-error")) || bytes.Contains(exported, []byte("moved-success")) {
		t.Errorf("exported journal contains rejected or duplicate source:\n%s", exported)
	}

	if batch.Records[1].Postings[0].Comment != "サンプル商品" || batch.Records[1].Warnings[0].PostingIndex == nil || *batch.Records[1].Warnings[0].PostingIndex != 0 {
		t.Fatal("Process() mutated its input batch")
	}
	result.Outcomes[0].Entry.Comments[0] = "changed"
	if result.Entries[0].Comments[0] != "source: receipts/sample-success.jpg" {
		t.Fatal("Outcome.Entry shares mutable comments with ProcessResult.Entries")
	}
	result.Entries[0].Postings[0].Amount.Commodity = "USD"
	if batch.Records[0].Postings[0].Amount.Commodity != "JPY" || result.Outcomes[0].Entry.Postings[0].Amount.Commodity != "JPY" {
		t.Fatal("processed entries share mutable amounts with input or outcomes")
	}
}

func TestProcessCommittedIdentityAsDuplicate(t *testing.T) {
	t.Parallel()
	batch, err := Decode(strings.NewReader(`{
  "schema_version": 1,
  "records": [{
    "source": {"namespace": "manual", "display": "entries/sample.json"},
    "occurred_at": "2026-08-12",
    "description": "サンプル店舗",
    "postings": [
      {"account": "費用:食費", "amount": "100", "commodity": "JPY"},
      {"account": "資産:現金", "amount": "-100", "commodity": "JPY"}
    ]
  }]
}`))
	if err != nil {
		t.Fatalf("Decode() error = %v", err)
	}

	result := Process(batch, []RecordIdentity{batch.Records[0].Identity})
	if len(result.Outcomes) != 1 || result.Outcomes[0].Status != OutcomeDuplicate || len(result.Entries) != 0 {
		t.Fatalf("Process() = %+v", result)
	}
	if diagnostics := result.Outcomes[0].Diagnostics; len(diagnostics) != 1 || diagnostics[0].Code != DiagnosticAlreadyCommitted {
		t.Fatalf("duplicate diagnostics = %+v", diagnostics)
	}
}

func TestProcessProjectsTotalPrice(t *testing.T) {
	t.Parallel()
	batch, err := Decode(strings.NewReader(`{
  "schema_version": 2,
  "records": [{
    "source": {"namespace": "tackler", "display": "uploaded.txn"},
    "occurred_at": "2026-08-12",
    "description": "匿名投資取引",
    "postings": [
      {"account": "資産:投資信託", "amount": "350", "commodity": "口", "total_price": {"amount": "675", "commodity": "JPY"}},
      {"account": "資産:購入予定"}
    ]
  }]
}`))
	if err != nil {
		t.Fatalf("Decode() error = %v", err)
	}

	result := Process(batch, nil)
	if len(result.Entries) != 1 || result.Entries[0].Postings[0].TotalPrice == nil {
		t.Fatalf("Process() = %+v", result)
	}
	result.Entries[0].Postings[0].TotalPrice.Commodity = "USD"
	if batch.Records[0].Postings[0].TotalPrice.Commodity != "JPY" || result.Outcomes[0].Entry.Postings[0].TotalPrice.Commodity != "JPY" {
		t.Fatal("processed entries share mutable total prices")
	}
}

func TestProcessReportsCommodityMismatchLocation(t *testing.T) {
	t.Parallel()
	record := replaceJSON(validRecordJSON(), `"commodity":"JPY"}`, `"commodity":"USD"}`)
	batch, err := Decode(strings.NewReader(batchJSON(record, "")))
	if err != nil {
		t.Fatalf("Decode() error = %v", err)
	}
	result := Process(batch, nil)
	if len(result.Outcomes) != 1 || result.Outcomes[0].Status != OutcomeError || len(result.Entries) != 0 {
		t.Fatalf("Process() = %+v", result)
	}
	diagnostic := result.Outcomes[0].Diagnostics[0]
	if diagnostic.Code != DiagnosticCommodityMismatch || diagnostic.FieldPath != "postings[1].commodity" || diagnostic.PostingIndex == nil || *diagnostic.PostingIndex != 1 {
		t.Fatalf("commodity mismatch diagnostic = %+v", diagnostic)
	}
}

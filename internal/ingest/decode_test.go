package ingest

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/hirokinko/bokiccio/internal/ledger"
)

func TestDecodeValidV1Fixture(t *testing.T) {
	t.Parallel()
	file, err := os.Open("testdata/valid-v1.json")
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer file.Close()

	batch, err := Decode(file)
	if err != nil {
		t.Fatalf("Decode() error = %v", err)
	}
	if batch.SchemaVersion != SchemaVersion || len(batch.Records) != 2 {
		t.Fatalf("Decode() = version %d with %d records", batch.SchemaVersion, len(batch.Records))
	}

	first := batch.Records[0]
	if first.Source.Namespace != "receipt" || first.Source.Display != "receipts/sample-001.jpg" || first.Source.ExternalID != "sample-001" {
		t.Errorf("first source = %+v", first.Source)
	}
	if first.OccurredAt.Precision() != ledger.EntryDate || first.OccurredAt.String() != "2026-08-09" {
		t.Errorf("first occurred_at = %q (%v)", first.OccurredAt.String(), first.OccurredAt.Precision())
	}
	if first.Postings[0].Amount.Value.String() != "207.00" || first.Postings[0].Amount.Value.Scale() != 2 {
		t.Errorf("first amount = %s scale=%d", first.Postings[0].Amount.Value.String(), first.Postings[0].Amount.Value.Scale())
	}
	if first.Identity.Kind != IdentityExternalID || first.Identity.AlgorithmVersion != IdentityAlgorithmVersion {
		t.Errorf("first identity = %+v", first.Identity)
	}
	if got, want := first.Identity.HexDigest(), "ec483c71f1d4186e1984e2ab1cf13c451135576c22e207e8356d525ddfa03932"; got != want {
		t.Errorf("first identity digest = %s, want %s", got, want)
	}

	second := batch.Records[1]
	if second.OccurredAt.Precision() != ledger.EntryDateTime || second.OccurredAt.String() != "2026-08-10T14:30:00.1234+09:00" {
		t.Errorf("second occurred_at = %q (%v)", second.OccurredAt.String(), second.OccurredAt.Precision())
	}
	if second.Postings[1].Amount != nil || second.Postings[1].Comment != "自動均衡" {
		t.Errorf("second final posting = %+v", second.Postings[1])
	}
	if second.Identity.Kind != IdentityFingerprint || len(second.Identity.HexDigest()) != 64 {
		t.Errorf("second identity = %+v", second.Identity)
	}
	if got, want := second.Identity.HexDigest(), "eb09594511dab0f6fdc94d669643f7f1f0cadcbf0dabe9ff588977158cf72404"; got != want {
		t.Errorf("second identity digest = %s, want %s", got, want)
	}
}

func TestDecodeRejectsInvalidInputWithPath(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		input    string
		wantPath string
		wantErr  error
	}{
		{name: "unknown version", input: `{"schema_version":2,"records":[]}`, wantPath: "$.schema_version", wantErr: ErrUnsupportedSchemaVersion},
		{name: "missing records", input: `{"schema_version":1}`, wantPath: "$.records", wantErr: ErrInvalidInput},
		{name: "unknown top-level field", input: `{"schema_version":1,"records":[],"extra":true}`, wantPath: "$", wantErr: ErrInvalidInput},
		{name: "unknown record field", input: batchJSON(validRecordJSON(), `,"extra":true`), wantPath: "$.records[0]", wantErr: ErrInvalidInput},
		{name: "invalid timestamp", input: batchJSON(replaceJSON(validRecordJSON(), `"occurred_at":"2026-08-10"`, `"occurred_at":"2026-08-10T14:30:00"`), ""), wantPath: "$.records[0].occurred_at", wantErr: ErrInvalidInput},
		{name: "invalid decimal", input: batchJSON(recordWithPostings(`{"account":"費用:食費","amount":"1e2","commodity":"JPY"},{"account":"資産:現金","amount":"-100","commodity":"JPY"}`), ""), wantPath: "$.records[0].postings[0].amount", wantErr: ledger.ErrInvalidDecimal},
		{name: "empty account", input: batchJSON(recordWithPostings(`{"account":"","amount":"100","commodity":"JPY"},{"account":"資産:現金","amount":"-100","commodity":"JPY"}`), ""), wantPath: "$.records[0].postings[0].account", wantErr: ErrInvalidInput},
		{name: "empty commodity", input: batchJSON(recordWithPostings(`{"account":"費用:食費","amount":"100","commodity":""},{"account":"資産:現金","amount":"-100","commodity":"JPY"}`), ""), wantPath: "$.records[0].postings[0].commodity", wantErr: ErrInvalidInput},
		{name: "unknown posting field", input: batchJSON(recordWithPostings(`{"account":"費用:食費","amount":"100","commodity":"JPY","extra":true},{"account":"資産:現金","amount":"-100","commodity":"JPY"}`), ""), wantPath: "$.records[0].postings[0]", wantErr: ErrInvalidInput},
		{name: "invalid warning code", input: batchJSON(addRecordField(validRecordJSON(), `"warnings":[{"code":"Receipt Warning","message":"確認してください"}]`), ""), wantPath: "$.records[0].warnings[0].code", wantErr: ErrInvalidInput},
		{name: "invalid warning posting index", input: batchJSON(addRecordField(validRecordJSON(), `"warnings":[{"code":"receipt.warning","message":"確認してください","posting_index":2}]`), ""), wantPath: "$.records[0].warnings[0].posting_index", wantErr: ErrInvalidInput},
		{name: "one posting", input: batchJSON(recordWithPostings(`{"account":"資産:現金"}`), ""), wantPath: "$.records[0].postings", wantErr: ErrInvalidInput},
		{name: "partial amount", input: batchJSON(recordWithPostings(`{"account":"費用:食費","amount":"100"},{"account":"資産:現金","amount":"-100","commodity":"JPY"}`), ""), wantPath: "$.records[0].postings[0]", wantErr: ErrInvalidInput},
		{name: "non-final omission", input: batchJSON(recordWithPostings(`{"account":"費用:食費"},{"account":"資産:現金","amount":"-100","commodity":"JPY"}`), ""), wantPath: "$.records[0].postings[0].amount", wantErr: ErrInvalidInput},
		{name: "absolute display source", input: batchJSON(replaceJSON(validRecordJSON(), `"source":{"namespace":"manual","display":"entries/sample-001.json"}`, `"source":{"namespace":"receipt","display":"/private/sample.jpg"}`), ""), wantPath: "$.records[0].source.display", wantErr: ErrInvalidInput},
		{name: "Windows absolute display source", input: batchJSON(replaceJSON(validRecordJSON(), `"source":{"namespace":"manual","display":"entries/sample-001.json"}`, `"source":{"namespace":"receipt","display":"C:\\private\\sample.jpg"}`), ""), wantPath: "$.records[0].source.display", wantErr: ErrInvalidInput},
		{name: "file URI display source", input: batchJSON(replaceJSON(validRecordJSON(), `"source":{"namespace":"manual","display":"entries/sample-001.json"}`, `"source":{"namespace":"receipt","display":"file:///private/sample.jpg"}`), ""), wantPath: "$.records[0].source.display", wantErr: ErrInvalidInput},
		{name: "query in display source", input: batchJSON(replaceJSON(validRecordJSON(), `"source":{"namespace":"manual","display":"entries/sample-001.json"}`, `"source":{"namespace":"mail","display":"https://example.invalid/message?id=secret"}`), ""), wantPath: "$.records[0].source.display", wantErr: ErrInvalidInput},
		{name: "trailing JSON", input: `{"schema_version":1,"records":[]} {}`, wantPath: "$", wantErr: ErrInvalidInput},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			_, err := Decode(strings.NewReader(test.input))
			if !errors.Is(err, ErrInvalidInput) {
				t.Fatalf("Decode() error = %v, want %v", err, ErrInvalidInput)
			}
			if !errors.Is(err, test.wantErr) {
				t.Fatalf("Decode() error = %v, want %v", err, test.wantErr)
			}
			var inputErr *InputError
			if !errors.As(err, &inputErr) {
				t.Fatalf("Decode() error = %T, want *InputError", err)
			}
			if inputErr.Path != test.wantPath {
				t.Fatalf("InputError.Path = %q, want %q (error: %v)", inputErr.Path, test.wantPath, err)
			}
		})
	}
}

func TestDecodeIdentityRules(t *testing.T) {
	t.Parallel()
	t.Run("external ID ignores accounting fields", func(t *testing.T) {
		t.Parallel()
		left := replaceJSON(validRecordJSON(), `"source":{"namespace":"manual","display":"entries/sample-001.json"}`, `"source":{"namespace":"receipt","display":"a.jpg","external_id":"same"}`)
		right := replaceJSON(validRecordJSON(), `"source":{"namespace":"manual","display":"entries/sample-001.json"}`, `"source":{"namespace":"receipt","display":"b.jpg","external_id":"same"}`)
		right = replaceJSON(right, `"description":"サンプル店舗"`, `"description":"変更後"`)
		identities := decodeIdentities(t, left, right)
		if identities[0] != identities[1] {
			t.Fatalf("external identities differ: %s != %s", identities[0].HexDigest(), identities[1].HexDigest())
		}
	})

	t.Run("fingerprint canonicalizes time decimal and excluded fields", func(t *testing.T) {
		t.Parallel()
		left := replaceJSON(validRecordJSON(), `"source":{"namespace":"manual","display":"entries/sample-001.json"}`, `"source":{"namespace":"mail","display":"a.eml"}`)
		left = replaceJSON(left, `"occurred_at":"2026-08-10"`, `"occurred_at":"2026-08-10T14:30:00+09:00"`)
		left = replaceJSON(left, `"description":"サンプル店舗"`, `"description":" サンプル店舗 "`)
		left = addRecordField(left, `"comments":["left"]`)
		left = addRecordField(left, `"warnings":[{"code":"source.left","message":"left","posting_index":0}]`)
		right := replaceJSON(validRecordJSON(), `"source":{"namespace":"manual","display":"entries/sample-001.json"}`, `"source":{"namespace":"mail","display":"b.eml"}`)
		right = replaceJSON(right, `"occurred_at":"2026-08-10"`, `"occurred_at":"2026-08-10T05:30:00Z"`)
		right = addRecordField(right, `"comments":["right"]`)
		right = addRecordField(right, `"warnings":[{"code":"source.right","message":"right","posting_index":1}]`)
		right = replaceJSON(right, `"comment":"sample"`, `"comment":"changed"`)
		right = strings.ReplaceAll(right, `"100.00"`, `"100"`)
		right = strings.ReplaceAll(right, `"-100.00"`, `"-100"`)
		identities := decodeIdentities(t, left, right)
		if identities[0] != identities[1] {
			t.Fatalf("fingerprints differ: %s != %s", identities[0].HexDigest(), identities[1].HexDigest())
		}
	})

	t.Run("date differs from midnight timestamp", func(t *testing.T) {
		t.Parallel()
		identities := decodeIdentities(t,
			validRecordJSON(),
			replaceJSON(validRecordJSON(), `"occurred_at":"2026-08-10"`, `"occurred_at":"2026-08-10T00:00:00Z"`),
		)
		if identities[0] == identities[1] {
			t.Fatal("date and midnight timestamp have the same fingerprint")
		}
	})

	t.Run("same-batch duplicate remains available to processor", func(t *testing.T) {
		t.Parallel()
		record := validRecordJSON()
		identities := decodeIdentities(t, record, record)
		if identities[0] != identities[1] {
			t.Fatal("identical records have different identities")
		}
	})
}

func TestCanonicalDecimalNormalizesNegativeZero(t *testing.T) {
	t.Parallel()
	negativeZero, err := ledger.ParseDecimal("-0.000")
	if err != nil {
		t.Fatalf("ParseDecimal() error = %v", err)
	}
	if got := canonicalDecimal(negativeZero); got != "0" {
		t.Fatalf("canonicalDecimal() = %q, want 0", got)
	}
}

func decodeIdentities(t *testing.T, records ...string) []RecordIdentity {
	t.Helper()
	batch, err := Decode(strings.NewReader(fmt.Sprintf(`{"schema_version":1,"records":[%s]}`, strings.Join(records, ","))))
	if err != nil {
		t.Fatalf("Decode() error = %v", err)
	}
	identities := make([]RecordIdentity, len(batch.Records))
	for index, record := range batch.Records {
		identities[index] = record.Identity
	}
	return identities
}

func batchJSON(record, recordSuffix string) string {
	if recordSuffix != "" {
		record = strings.TrimSuffix(record, "}") + recordSuffix + "}"
	}
	return fmt.Sprintf(`{"schema_version":1,"records":[%s]}`, record)
}

func validRecordJSON() string {
	return `{"source":{"namespace":"manual","display":"entries/sample-001.json"},"occurred_at":"2026-08-10","description":"サンプル店舗","postings":[{"account":"費用:食費","amount":"100.00","commodity":"JPY","comment":"sample"},{"account":"資産:現金","amount":"-100.00","commodity":"JPY"}]}`
}

func replaceJSON(input, old, replacement string) string {
	return strings.Replace(input, old, replacement, 1)
}

func addRecordField(input, field string) string {
	return strings.TrimSuffix(input, "}") + "," + field + "}"
}

func recordWithPostings(postings string) string {
	return fmt.Sprintf(`{"source":{"namespace":"manual","display":"entries/sample-001.json"},"occurred_at":"2026-08-10","description":"サンプル店舗","postings":[%s]}`, postings)
}

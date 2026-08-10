package ingest

import (
	"bytes"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/hirokinko/bokiccio/internal/tacklerfmt"
)

func TestBuildIsDeterministicAndAdvancesState(t *testing.T) {
	t.Parallel()
	input := readFixture(t, "testdata/mixed-outcomes-v1.json")
	options := tacklerfmt.Options{OmittedAmounts: tacklerfmt.PreserveOmitted}

	first, err := Build(input, EmptyState(), options)
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	second, err := Build(input, EmptyState(), options)
	if err != nil {
		t.Fatalf("Build() second error = %v", err)
	}
	if first.RunIdentity != second.RunIdentity || !bytes.Equal(first.ReportBytes, second.ReportBytes) || !bytes.Equal(first.Journal, second.Journal) || !bytes.Equal(first.StateBytes, second.StateBytes) {
		t.Fatal("Build() is not byte-deterministic")
	}
	if !first.HasErrors {
		t.Fatal("Build() did not retain the error outcome")
	}
	if first.NextState.Generation != 1 || len(first.NextState.Identities) != 2 {
		t.Fatalf("NextState = %+v, want generation 1 with 2 identities", first.NextState)
	}

	report, err := DecodeReport(bytes.NewReader(first.ReportBytes))
	if err != nil {
		t.Fatalf("DecodeReport() error = %v", err)
	}
	if report.RunIdentity != first.RunIdentity || report.PreStateGeneration != 0 || len(report.Outcomes) != 4 {
		t.Fatalf("decoded report = %+v", report)
	}
	if report.Outcomes[1].Diagnostics[0].Identity != report.Outcomes[1].Identity {
		t.Fatal("decoded diagnostic did not recover its record identity")
	}
	state, err := DecodeState(bytes.NewReader(first.StateBytes))
	if err != nil {
		t.Fatalf("DecodeState() error = %v", err)
	}
	if state.Generation != first.NextState.Generation || !equalIdentities(state.Identities, first.NextState.Identities) {
		t.Fatalf("decoded state = %+v, want %+v", state, first.NextState)
	}
}

func TestBuildUsesCommittedStateForCrossRunDuplicates(t *testing.T) {
	t.Parallel()
	input := readFixture(t, "testdata/mixed-outcomes-v1.json")
	options := tacklerfmt.Options{OmittedAmounts: tacklerfmt.PreserveOmitted}
	first, err := Build(input, EmptyState(), options)
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	retry, err := Build(input, first.NextState, options)
	if err != nil {
		t.Fatalf("Build() retry error = %v", err)
	}
	want := []OutcomeStatus{OutcomeDuplicate, OutcomeDuplicate, OutcomeError, OutcomeDuplicate}
	for index, status := range want {
		if retry.Report.Outcomes[index].Status != status {
			t.Errorf("retry outcome %d = %q, want %q", index, retry.Report.Outcomes[index].Status, status)
		}
	}
	if len(retry.Journal) != 0 {
		t.Fatalf("retry journal = %q, want no journal", retry.Journal)
	}
	if retry.NextState.Generation != first.NextState.Generation || !equalIdentities(retry.NextState.Identities, first.NextState.Identities) {
		t.Fatalf("retry state = %+v, want unchanged %+v", retry.NextState, first.NextState)
	}
	if retry.RunIdentity == first.RunIdentity {
		t.Fatal("run identity did not include pre-run state generation")
	}
}

func TestStateCodecRejectsUnknownVersionAndNonCanonicalState(t *testing.T) {
	t.Parallel()
	if _, err := DecodeState(strings.NewReader(`{"schema_version":2,"generation":0,"identities":[]}`)); !errors.Is(err, ErrUnsupportedStateVersion) {
		t.Fatalf("DecodeState() error = %v, want ErrUnsupportedStateVersion", err)
	}

	input := readFixture(t, "testdata/valid-v1.json")
	batch, err := Decode(bytes.NewReader(input))
	if err != nil {
		t.Fatalf("Decode() error = %v", err)
	}
	identity := toWireIdentity(batch.Records[0].Identity)
	data, err := marshalCanonical(wireState{SchemaVersion: StateSchemaVersion, Identities: []wireIdentity{identity, identity}})
	if err != nil {
		t.Fatalf("marshalCanonical() error = %v", err)
	}
	if _, err := DecodeState(bytes.NewReader(data)); !errors.Is(err, ErrInvalidState) {
		t.Fatalf("DecodeState() duplicate error = %v, want ErrInvalidState", err)
	}
}

func TestReportCodecRejectsUnknownVersionAndField(t *testing.T) {
	t.Parallel()
	if _, err := DecodeReport(strings.NewReader(`{"schema_version":2}`)); !errors.Is(err, ErrUnsupportedReportVersion) {
		t.Fatalf("DecodeReport() error = %v, want ErrUnsupportedReportVersion", err)
	}
	if _, err := DecodeReport(strings.NewReader(`{"schema_version":1,"extra":true}`)); !errors.Is(err, ErrInvalidReport) {
		t.Fatalf("DecodeReport() unknown field error = %v, want ErrInvalidReport", err)
	}
}

func readFixture(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	return data
}

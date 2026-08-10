package ingest

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"slices"
	"strconv"
	"strings"

	"github.com/hirokinko/bokiccio/internal/tacklerfmt"
)

const (
	ReportSchemaVersion = 1
	StateSchemaVersion  = 1
	RunIdentityVersion  = 1
)

var (
	ErrInvalidReport            = errors.New("invalid run report")
	ErrInvalidState             = errors.New("invalid deduplication state")
	ErrUnsupportedReportVersion = errors.New("unsupported report schema version")
	ErrUnsupportedStateVersion  = errors.New("unsupported state schema version")
)

type RunIdentity struct {
	AlgorithmVersion int
	Digest           [sha256.Size]byte
}

func (identity RunIdentity) HexDigest() string {
	return fmt.Sprintf("%x", identity.Digest)
}

type State struct {
	SchemaVersion int
	Generation    uint64
	Identities    []RecordIdentity
}

func EmptyState() State {
	return State{SchemaVersion: StateSchemaVersion, Identities: []RecordIdentity{}}
}

type Report struct {
	SchemaVersion      int
	RunIdentity        RunIdentity
	InputDigest        [sha256.Size]byte
	PreStateGeneration uint64
	Outcomes           []Outcome
}

type Artifact struct {
	RunIdentity RunIdentity
	Report      Report
	ReportBytes []byte
	Journal     []byte
	NextState   State
	StateBytes  []byte
	HasErrors   bool
}

// Build deterministically decodes and processes input using the supplied
// committed state. It performs no file I/O.
func Build(input []byte, state State, options tacklerfmt.Options) (Artifact, error) {
	state = canonicalState(state)
	if err := validateState(state); err != nil {
		return Artifact{}, err
	}
	batch, err := Decode(bytes.NewReader(input))
	if err != nil {
		return Artifact{}, err
	}
	if options.OmittedAmounts != tacklerfmt.PreserveOmitted && options.OmittedAmounts != tacklerfmt.FillOmitted {
		return Artifact{}, fmt.Errorf("build journal: %w", tacklerfmt.ErrInvalidOptions)
	}

	processed := Process(batch, state.Identities)
	var journal []byte
	if len(processed.Entries) > 0 {
		journal, err = tacklerfmt.Export(processed.Entries, options)
		if err != nil {
			return Artifact{}, fmt.Errorf("build journal: %w", err)
		}
	}

	inputDigest := sha256.Sum256(input)
	runIdentity := resolveRunIdentity(inputDigest, state.Generation, options)
	report := Report{
		SchemaVersion:      ReportSchemaVersion,
		RunIdentity:        runIdentity,
		InputDigest:        inputDigest,
		PreStateGeneration: state.Generation,
		Outcomes:           processed.Outcomes,
	}
	reportBytes, err := EncodeReport(report)
	if err != nil {
		return Artifact{}, err
	}

	nextState := state
	for _, outcome := range processed.Outcomes {
		if outcome.Status == OutcomeSuccess || outcome.Status == OutcomeWarning {
			nextState.Identities = append(nextState.Identities, outcome.Identity)
		}
	}
	nextState = canonicalState(nextState)
	if len(nextState.Identities) > len(state.Identities) {
		nextState.Generation++
	}
	stateBytes, err := EncodeState(nextState)
	if err != nil {
		return Artifact{}, err
	}

	artifact := Artifact{
		RunIdentity: runIdentity,
		Report:      report,
		ReportBytes: reportBytes,
		Journal:     journal,
		NextState:   nextState,
		StateBytes:  stateBytes,
	}
	for _, outcome := range processed.Outcomes {
		if outcome.Status == OutcomeError {
			artifact.HasErrors = true
			break
		}
	}
	return artifact, nil
}

func resolveRunIdentity(inputDigest [sha256.Size]byte, generation uint64, options tacklerfmt.Options) RunIdentity {
	record := hashIdentity(IdentityFingerprint,
		"bokiccio.run-identity",
		"v1",
		fmt.Sprintf("%x", inputDigest),
		strconv.FormatUint(generation, 10),
		strconv.Itoa(int(options.OmittedAmounts)),
	)
	return RunIdentity{AlgorithmVersion: RunIdentityVersion, Digest: record.Digest}
}

type wireIdentity struct {
	Kind             IdentityKind `json:"kind"`
	AlgorithmVersion int          `json:"algorithm_version"`
	Digest           string       `json:"digest"`
}

type wireRunIdentity struct {
	AlgorithmVersion int    `json:"algorithm_version"`
	Digest           string `json:"digest"`
}

type wireDiagnostic struct {
	Code         string             `json:"code"`
	Severity     DiagnosticSeverity `json:"severity"`
	Message      string             `json:"message"`
	Identity     wireIdentity       `json:"identity"`
	FieldPath    string             `json:"field_path,omitempty"`
	PostingIndex *int               `json:"posting_index,omitempty"`
}

type wireOutcome struct {
	RecordIndex int              `json:"record_index"`
	Status      OutcomeStatus    `json:"status"`
	Identity    wireIdentity     `json:"identity"`
	Source      reportWireSource `json:"source"`
	Diagnostics []wireDiagnostic `json:"diagnostics"`
}

type reportWireSource struct {
	Namespace string `json:"namespace"`
	Display   string `json:"display"`
}

type wireReport struct {
	SchemaVersion      int             `json:"schema_version"`
	RunIdentity        wireRunIdentity `json:"run_identity"`
	InputDigest        string          `json:"input_digest"`
	PreStateGeneration uint64          `json:"pre_state_generation"`
	Outcomes           []wireOutcome   `json:"outcomes"`
}

type wireState struct {
	SchemaVersion int            `json:"schema_version"`
	Generation    uint64         `json:"generation"`
	Identities    []wireIdentity `json:"identities"`
}

func EncodeReport(report Report) ([]byte, error) {
	if report.SchemaVersion != ReportSchemaVersion {
		return nil, fmt.Errorf("%w: %d", ErrUnsupportedReportVersion, report.SchemaVersion)
	}
	wire := wireReport{
		SchemaVersion:      report.SchemaVersion,
		RunIdentity:        toWireRunIdentity(report.RunIdentity),
		InputDigest:        fmt.Sprintf("%x", report.InputDigest),
		PreStateGeneration: report.PreStateGeneration,
		Outcomes:           make([]wireOutcome, len(report.Outcomes)),
	}
	for index, outcome := range report.Outcomes {
		wire.Outcomes[index] = toWireOutcome(outcome)
	}
	return marshalCanonical(wire)
}

func DecodeReport(reader io.Reader) (Report, error) {
	var wire wireReport
	if err := decodeStrict(reader, &wire); err != nil {
		return Report{}, fmt.Errorf("%w: %v", ErrInvalidReport, err)
	}
	if wire.SchemaVersion != ReportSchemaVersion {
		return Report{}, fmt.Errorf("%w: %d", ErrUnsupportedReportVersion, wire.SchemaVersion)
	}
	runIdentity, err := parseRunIdentity(wire.RunIdentity)
	if err != nil {
		return Report{}, fmt.Errorf("%w: run_identity: %v", ErrInvalidReport, err)
	}
	inputDigest, err := parseDigest(wire.InputDigest)
	if err != nil {
		return Report{}, fmt.Errorf("%w: input_digest: %v", ErrInvalidReport, err)
	}
	report := Report{
		SchemaVersion:      wire.SchemaVersion,
		RunIdentity:        runIdentity,
		InputDigest:        inputDigest,
		PreStateGeneration: wire.PreStateGeneration,
		Outcomes:           make([]Outcome, len(wire.Outcomes)),
	}
	for index, outcome := range wire.Outcomes {
		converted, err := fromWireOutcome(outcome)
		if err != nil {
			return Report{}, fmt.Errorf("%w: outcomes[%d]: %v", ErrInvalidReport, index, err)
		}
		report.Outcomes[index] = converted
	}
	return report, nil
}

func EncodeState(state State) ([]byte, error) {
	state = canonicalState(state)
	if err := validateState(state); err != nil {
		return nil, err
	}
	wire := wireState{SchemaVersion: state.SchemaVersion, Generation: state.Generation, Identities: make([]wireIdentity, len(state.Identities))}
	for index, identity := range state.Identities {
		wire.Identities[index] = toWireIdentity(identity)
	}
	return marshalCanonical(wire)
}

func DecodeState(reader io.Reader) (State, error) {
	var wire wireState
	if err := decodeStrict(reader, &wire); err != nil {
		return State{}, fmt.Errorf("%w: %v", ErrInvalidState, err)
	}
	if wire.SchemaVersion != StateSchemaVersion {
		return State{}, fmt.Errorf("%w: %d", ErrUnsupportedStateVersion, wire.SchemaVersion)
	}
	state := State{SchemaVersion: wire.SchemaVersion, Generation: wire.Generation, Identities: make([]RecordIdentity, len(wire.Identities))}
	for index, identity := range wire.Identities {
		converted, err := fromWireIdentity(identity)
		if err != nil {
			return State{}, fmt.Errorf("%w: identities[%d]: %v", ErrInvalidState, index, err)
		}
		state.Identities[index] = converted
	}
	canonical := canonicalState(state)
	if !equalIdentities(state.Identities, canonical.Identities) {
		return State{}, fmt.Errorf("%w: identities must be unique and canonically ordered", ErrInvalidState)
	}
	return state, nil
}

func toWireRunIdentity(identity RunIdentity) wireRunIdentity {
	return wireRunIdentity{AlgorithmVersion: identity.AlgorithmVersion, Digest: identity.HexDigest()}
}

func parseRunIdentity(wire wireRunIdentity) (RunIdentity, error) {
	if wire.AlgorithmVersion != RunIdentityVersion {
		return RunIdentity{}, fmt.Errorf("unsupported algorithm version: %d", wire.AlgorithmVersion)
	}
	digest, err := parseDigest(wire.Digest)
	if err != nil {
		return RunIdentity{}, err
	}
	return RunIdentity{AlgorithmVersion: wire.AlgorithmVersion, Digest: digest}, nil
}

func toWireIdentity(identity RecordIdentity) wireIdentity {
	return wireIdentity{Kind: identity.Kind, AlgorithmVersion: identity.AlgorithmVersion, Digest: identity.HexDigest()}
}

func fromWireIdentity(wire wireIdentity) (RecordIdentity, error) {
	if wire.Kind != IdentityExternalID && wire.Kind != IdentityFingerprint {
		return RecordIdentity{}, fmt.Errorf("unsupported identity kind: %q", wire.Kind)
	}
	if wire.AlgorithmVersion != IdentityAlgorithmVersion {
		return RecordIdentity{}, fmt.Errorf("unsupported algorithm version: %d", wire.AlgorithmVersion)
	}
	digest, err := parseDigest(wire.Digest)
	if err != nil {
		return RecordIdentity{}, err
	}
	return RecordIdentity{Kind: wire.Kind, AlgorithmVersion: wire.AlgorithmVersion, Digest: digest}, nil
}

func toWireOutcome(outcome Outcome) wireOutcome {
	wire := wireOutcome{
		RecordIndex: outcome.RecordIndex,
		Status:      outcome.Status,
		Identity:    toWireIdentity(outcome.Identity),
		Source:      reportWireSource{Namespace: outcome.Source.Namespace, Display: outcome.Source.Display},
		Diagnostics: make([]wireDiagnostic, len(outcome.Diagnostics)),
	}
	for index, diagnostic := range outcome.Diagnostics {
		wire.Diagnostics[index] = wireDiagnostic{
			Code:         diagnostic.Code,
			Severity:     diagnostic.Severity,
			Message:      diagnostic.Message,
			Identity:     toWireIdentity(diagnostic.Identity),
			FieldPath:    diagnostic.FieldPath,
			PostingIndex: cloneIndex(diagnostic.PostingIndex),
		}
	}
	return wire
}

func fromWireOutcome(wire wireOutcome) (Outcome, error) {
	if wire.RecordIndex < 0 {
		return Outcome{}, errors.New("record_index must not be negative")
	}
	if wire.Status != OutcomeSuccess && wire.Status != OutcomeWarning && wire.Status != OutcomeError && wire.Status != OutcomeDuplicate {
		return Outcome{}, fmt.Errorf("unsupported status: %q", wire.Status)
	}
	identity, err := fromWireIdentity(wire.Identity)
	if err != nil {
		return Outcome{}, fmt.Errorf("identity: %w", err)
	}
	outcome := Outcome{
		RecordIndex: wire.RecordIndex,
		Status:      wire.Status,
		Identity:    identity,
		Source:      Source{Namespace: wire.Source.Namespace, Display: wire.Source.Display},
		Diagnostics: make([]Diagnostic, len(wire.Diagnostics)),
	}
	if err := validateSourceText(outcome.Source.Namespace); err != nil {
		return Outcome{}, fmt.Errorf("source.namespace: %w", err)
	}
	if err := validateDisplaySource(outcome.Source.Display); err != nil {
		return Outcome{}, fmt.Errorf("source.display: %w", err)
	}
	for index, diagnostic := range wire.Diagnostics {
		if !diagnosticCode.MatchString(diagnostic.Code) {
			return Outcome{}, fmt.Errorf("diagnostics[%d].code is invalid", index)
		}
		if diagnostic.Severity != SeverityInfo && diagnostic.Severity != SeverityWarning && diagnostic.Severity != SeverityError {
			return Outcome{}, fmt.Errorf("diagnostics[%d].severity is invalid", index)
		}
		diagnosticIdentity, err := fromWireIdentity(diagnostic.Identity)
		if err != nil {
			return Outcome{}, fmt.Errorf("diagnostics[%d].identity: %w", index, err)
		}
		if diagnosticIdentity != identity {
			return Outcome{}, fmt.Errorf("diagnostics[%d].identity does not match outcome identity", index)
		}
		if strings.TrimSpace(diagnostic.Message) == "" || containsLineBreak(diagnostic.Message) {
			return Outcome{}, fmt.Errorf("diagnostics[%d].message is invalid", index)
		}
		outcome.Diagnostics[index] = Diagnostic{
			Code:         diagnostic.Code,
			Severity:     diagnostic.Severity,
			Message:      diagnostic.Message,
			Identity:     diagnosticIdentity,
			FieldPath:    diagnostic.FieldPath,
			PostingIndex: cloneIndex(diagnostic.PostingIndex),
		}
	}
	return outcome, nil
}

func parseDigest(text string) ([sha256.Size]byte, error) {
	var digest [sha256.Size]byte
	if len(text) != sha256.Size*2 {
		return digest, fmt.Errorf("digest must contain %d lowercase hexadecimal characters", sha256.Size*2)
	}
	for _, character := range text {
		if !((character >= '0' && character <= '9') || (character >= 'a' && character <= 'f')) {
			return digest, fmt.Errorf("digest must contain %d lowercase hexadecimal characters", sha256.Size*2)
		}
	}
	for index := range digest {
		high := hexNibble(text[index*2])
		low := hexNibble(text[index*2+1])
		digest[index] = high<<4 | low
	}
	return digest, nil
}

func hexNibble(character byte) byte {
	if character <= '9' {
		return character - '0'
	}
	return character - 'a' + 10
}

func canonicalState(state State) State {
	if state.SchemaVersion == 0 {
		state.SchemaVersion = StateSchemaVersion
	}
	state.Identities = append([]RecordIdentity(nil), state.Identities...)
	sortIdentities(state.Identities)
	write := 0
	for _, identity := range state.Identities {
		if write > 0 && state.Identities[write-1] == identity {
			continue
		}
		state.Identities[write] = identity
		write++
	}
	state.Identities = state.Identities[:write]
	if state.Identities == nil {
		state.Identities = []RecordIdentity{}
	}
	return state
}

func sortIdentities(identities []RecordIdentity) {
	slices.SortFunc(identities, compareIdentity)
}

func compareIdentity(left, right RecordIdentity) int {
	if left.Kind < right.Kind {
		return -1
	}
	if left.Kind > right.Kind {
		return 1
	}
	if left.AlgorithmVersion < right.AlgorithmVersion {
		return -1
	}
	if left.AlgorithmVersion > right.AlgorithmVersion {
		return 1
	}
	return bytes.Compare(left.Digest[:], right.Digest[:])
}

func validateState(state State) error {
	if state.SchemaVersion != StateSchemaVersion {
		return fmt.Errorf("%w: %d", ErrUnsupportedStateVersion, state.SchemaVersion)
	}
	for index, identity := range state.Identities {
		if identity.Kind != IdentityExternalID && identity.Kind != IdentityFingerprint {
			return fmt.Errorf("%w: identities[%d]: unsupported kind %q", ErrInvalidState, index, identity.Kind)
		}
		if identity.AlgorithmVersion != IdentityAlgorithmVersion {
			return fmt.Errorf("%w: identities[%d]: unsupported algorithm version %d", ErrInvalidState, index, identity.AlgorithmVersion)
		}
	}
	return nil
}

func equalIdentities(left, right []RecordIdentity) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}

func marshalCanonical(value any) ([]byte, error) {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(data, '\n'), nil
}

package ingest

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"path/filepath"
	"strings"
	"unicode"

	"github.com/hirokinko/bokiccio/internal/ledger"
)

var (
	ErrInvalidInput             = errors.New("invalid normalized input")
	ErrUnsupportedSchemaVersion = errors.New("unsupported schema version")
)

type InputError struct {
	Path string
	Err  error
}

func (err *InputError) Error() string {
	return fmt.Sprintf("%s: %v", err.Path, err.Err)
}

func (err *InputError) Unwrap() error {
	return err.Err
}

type wireBatch struct {
	SchemaVersion *int               `json:"schema_version"`
	Records       *[]json.RawMessage `json:"records"`
}

type wireRecord struct {
	Source      json.RawMessage    `json:"source"`
	OccurredAt  wireString         `json:"occurred_at"`
	Description wireString         `json:"description"`
	Comments    []string           `json:"comments"`
	Postings    *[]json.RawMessage `json:"postings"`
}

type wireSource struct {
	Namespace  wireString `json:"namespace"`
	Display    wireString `json:"display"`
	ExternalID wireString `json:"external_id"`
}

type wirePosting struct {
	Account   wireString `json:"account"`
	Amount    wireString `json:"amount"`
	Commodity wireString `json:"commodity"`
	Comment   wireString `json:"comment"`
}

type wireString struct {
	Value string
	Set   bool
}

func (value *wireString) UnmarshalJSON(data []byte) error {
	value.Set = true
	if bytes.Equal(data, []byte("null")) {
		return errors.New("must be a string, not null")
	}
	return json.Unmarshal(data, &value.Value)
}

func Decode(reader io.Reader) (Batch, error) {
	var input wireBatch
	if err := decodeStrict(reader, &input); err != nil {
		return Batch{}, invalidInput("$", err)
	}
	if input.SchemaVersion == nil {
		return Batch{}, invalidInput("$.schema_version", errors.New("required field is missing"))
	}
	if *input.SchemaVersion != SchemaVersion {
		return Batch{}, invalidInput("$.schema_version", fmt.Errorf("%w: %d", ErrUnsupportedSchemaVersion, *input.SchemaVersion))
	}
	if input.Records == nil {
		return Batch{}, invalidInput("$.records", errors.New("required field is missing"))
	}

	batch := Batch{SchemaVersion: SchemaVersion, Records: make([]Record, 0, len(*input.Records))}
	for index, raw := range *input.Records {
		path := fmt.Sprintf("$.records[%d]", index)
		record, err := decodeRecord(raw, path)
		if err != nil {
			return Batch{}, err
		}
		record.Identity = resolveIdentity(SchemaVersion, record)
		batch.Records = append(batch.Records, record)
	}
	return batch, nil
}

func decodeRecord(raw json.RawMessage, path string) (Record, error) {
	var input wireRecord
	if err := decodeStrict(bytes.NewReader(raw), &input); err != nil {
		return Record{}, invalidInput(path, err)
	}
	if len(input.Source) == 0 || bytes.Equal(input.Source, []byte("null")) {
		return Record{}, invalidInput(path+".source", errors.New("required field is missing"))
	}
	source, err := decodeSource(input.Source, path+".source")
	if err != nil {
		return Record{}, err
	}
	if !input.OccurredAt.Set {
		return Record{}, invalidInput(path+".occurred_at", errors.New("required field is missing"))
	}
	occurredAt, err := ledger.ParseEntryTime(input.OccurredAt.Value)
	if err != nil {
		return Record{}, invalidInput(path+".occurred_at", err)
	}
	if !input.Description.Set {
		return Record{}, invalidInput(path+".description", errors.New("required field is missing"))
	}
	if strings.TrimSpace(input.Description.Value) == "" || containsLineBreak(input.Description.Value) {
		return Record{}, invalidInput(path+".description", errors.New("must be non-empty and single-line"))
	}
	for index, comment := range input.Comments {
		if containsLineBreak(comment) {
			return Record{}, invalidInput(fmt.Sprintf("%s.comments[%d]", path, index), errors.New("must be single-line"))
		}
	}
	if input.Postings == nil {
		return Record{}, invalidInput(path+".postings", errors.New("required field is missing"))
	}
	if len(*input.Postings) < 2 {
		return Record{}, invalidInput(path+".postings", errors.New("must contain at least two postings"))
	}

	postings := make([]CandidatePosting, 0, len(*input.Postings))
	for index, rawPosting := range *input.Postings {
		posting, err := decodePosting(rawPosting, fmt.Sprintf("%s.postings[%d]", path, index), index == len(*input.Postings)-1)
		if err != nil {
			return Record{}, err
		}
		postings = append(postings, posting)
	}
	return Record{
		Source:      source,
		OccurredAt:  occurredAt,
		Description: input.Description.Value,
		Comments:    append([]string(nil), input.Comments...),
		Postings:    postings,
	}, nil
}

func decodeSource(raw json.RawMessage, path string) (Source, error) {
	var input wireSource
	if err := decodeStrict(bytes.NewReader(raw), &input); err != nil {
		return Source{}, invalidInput(path, err)
	}
	if !input.Namespace.Set {
		return Source{}, invalidInput(path+".namespace", errors.New("required field is missing"))
	}
	if err := validateSourceText(input.Namespace.Value); err != nil {
		return Source{}, invalidInput(path+".namespace", err)
	}
	if !input.Display.Set {
		return Source{}, invalidInput(path+".display", errors.New("required field is missing"))
	}
	if err := validateDisplaySource(input.Display.Value); err != nil {
		return Source{}, invalidInput(path+".display", err)
	}
	if input.ExternalID.Set {
		if err := validateSourceText(input.ExternalID.Value); err != nil {
			return Source{}, invalidInput(path+".external_id", err)
		}
	}
	return Source{Namespace: input.Namespace.Value, Display: input.Display.Value, ExternalID: input.ExternalID.Value}, nil
}

func decodePosting(raw json.RawMessage, path string, final bool) (CandidatePosting, error) {
	var input wirePosting
	if err := decodeStrict(bytes.NewReader(raw), &input); err != nil {
		return CandidatePosting{}, invalidInput(path, err)
	}
	if !input.Account.Set {
		return CandidatePosting{}, invalidInput(path+".account", errors.New("required field is missing"))
	}
	if strings.TrimSpace(input.Account.Value) == "" || containsLineBreak(input.Account.Value) {
		return CandidatePosting{}, invalidInput(path+".account", errors.New("must be non-empty and single-line"))
	}
	if input.Comment.Set && containsLineBreak(input.Comment.Value) {
		return CandidatePosting{}, invalidInput(path+".comment", errors.New("must be single-line"))
	}
	if input.Amount.Set != input.Commodity.Set {
		return CandidatePosting{}, invalidInput(path, errors.New("amount and commodity must either both be present or both be omitted"))
	}
	if !input.Amount.Set {
		if !final {
			return CandidatePosting{}, invalidInput(path+".amount", errors.New("only the final posting may omit its amount"))
		}
		return CandidatePosting{Account: input.Account.Value, Comment: input.Comment.Value}, nil
	}
	decimal, err := ledger.ParseDecimal(input.Amount.Value)
	if err != nil {
		return CandidatePosting{}, invalidInput(path+".amount", err)
	}
	if strings.TrimSpace(input.Commodity.Value) == "" || containsLineBreak(input.Commodity.Value) {
		return CandidatePosting{}, invalidInput(path+".commodity", errors.New("must be non-empty and single-line"))
	}
	return CandidatePosting{
		Account: input.Account.Value,
		Amount:  &ledger.Amount{Value: decimal, Commodity: ledger.Commodity(input.Commodity.Value)},
		Comment: input.Comment.Value,
	}, nil
}

func decodeStrict(reader io.Reader, destination any) error {
	decoder := json.NewDecoder(reader)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		return err
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		if err == nil {
			return errors.New("multiple JSON values are not allowed")
		}
		return err
	}
	return nil
}

func validateSourceText(value string) error {
	if strings.TrimSpace(value) == "" || strings.TrimSpace(value) != value {
		return errors.New("must be non-empty without surrounding whitespace")
	}
	for _, character := range value {
		if unicode.IsControl(character) {
			return errors.New("must not contain control characters")
		}
	}
	return nil
}

func validateDisplaySource(value string) error {
	if err := validateSourceText(value); err != nil {
		return err
	}
	if filepath.IsAbs(value) {
		return errors.New("must not be an absolute local path")
	}
	if len(value) >= 3 && ((value[0] >= 'A' && value[0] <= 'Z') || (value[0] >= 'a' && value[0] <= 'z')) && value[1] == ':' && (value[2] == '/' || value[2] == '\\') {
		return errors.New("must not be an absolute local path")
	}
	parsed, err := url.Parse(value)
	if err != nil {
		return errors.New("must be a relative path or URI")
	}
	if strings.EqualFold(parsed.Scheme, "file") {
		return errors.New("must not be an absolute local path")
	}
	if parsed.User != nil || parsed.RawQuery != "" {
		return errors.New("must not contain credentials or a query string")
	}
	return nil
}

func containsLineBreak(value string) bool {
	return strings.ContainsAny(value, "\r\n")
}

func invalidInput(path string, cause error) error {
	return inputError(path, fmt.Errorf("%w: %w", ErrInvalidInput, cause))
}

func inputError(path string, cause error) error {
	return &InputError{Path: path, Err: cause}
}

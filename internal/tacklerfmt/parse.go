package tacklerfmt

import (
	"errors"
	"fmt"
	"strings"

	"github.com/hirokinko/bokiccio/internal/ledger"
)

var ErrInvalidInput = errors.New("invalid Tackler input")

type InputError struct {
	Line    int
	Entry   int
	Code    string
	Message string
	Err     error
}

func (err *InputError) Error() string {
	location := "input"
	if err.Line > 0 {
		location = fmt.Sprintf("line %d", err.Line)
	} else if err.Entry > 0 {
		location = fmt.Sprintf("entry %d", err.Entry)
	}
	return fmt.Sprintf("%v: %s: %s", ErrInvalidInput, location, err.Message)
}

func (err *InputError) Unwrap() error {
	return err.Err
}

func (err *InputError) Is(target error) bool {
	return target == ErrInvalidInput || errors.Is(err.Err, target)
}

func SafeDiagnostic(err error) string {
	var inputError *InputError
	if errors.As(err, &inputError) {
		return inputError.Error()
	}
	return ErrInvalidInput.Error()
}

func DetailedDiagnostic(err error) string {
	var inputError *InputError
	if !errors.As(err, &inputError) {
		return ErrInvalidInput.Error()
	}
	location := "input"
	if inputError.Line > 0 {
		location = fmt.Sprintf("line %d", inputError.Line)
	} else if inputError.Entry > 0 {
		location = fmt.Sprintf("entry %d", inputError.Entry)
	}
	if inputError.Err == nil {
		return fmt.Sprintf("%v: %s: %s", ErrInvalidInput, location, inputError.Message)
	}
	return fmt.Sprintf("%v: %s: %s (%v)", ErrInvalidInput, location, inputError.Message, inputError.Err)
}

// Parse reads Bokiccio's deliberately limited Tackler-compatible subset.
// It validates every parsed entry and never returns partial entries on error.
func Parse(input []byte) ([]ledger.JournalEntry, error) {
	return parse(input, true)
}

// ParseUnvalidated reads the same syntax as Parse without domain validation.
// It is intended for revision editing flows that must preserve invalid
// revisions for later diagnostics.
func ParseUnvalidated(input []byte) ([]ledger.JournalEntry, error) {
	return parse(input, false)
}

func parse(input []byte, validate bool) ([]ledger.JournalEntry, error) {
	lines := splitInputLines(string(input))
	entries := []ledger.JournalEntry{}
	var current *ledger.JournalEntry

	for index, line := range lines {
		lineNumber := index + 1
		if strings.TrimSpace(line) == "" {
			continue
		}
		if isHeaderLine(line) {
			if current != nil {
				entry := *current
				if validate {
					var err error
					entry, err = validateParsedEntry(*current, len(entries))
					if err != nil {
						return nil, err
					}
				}
				entries = append(entries, entry)
			}
			entry, err := parseHeader(line)
			if err != nil {
				return nil, parseError(lineNumber, err)
			}
			current = &entry
			continue
		}
		if current == nil {
			return nil, lineError(lineNumber, "missing_header", "entry header is required before comments or postings", errors.New("entry header is required before comments or postings"))
		}
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, ";") {
			comment := strings.TrimSpace(strings.TrimPrefix(trimmed, ";"))
			if comment != "" {
				current.Comments = append(current.Comments, comment)
			}
			continue
		}
		posting, err := parsePosting(line)
		if err != nil {
			return nil, parseError(lineNumber, err)
		}
		current.Postings = append(current.Postings, posting)
	}

	if current != nil {
		entry := *current
		if validate {
			var err error
			entry, err = validateParsedEntry(*current, len(entries))
			if err != nil {
				return nil, err
			}
		}
		entries = append(entries, entry)
	}
	if len(entries) == 0 {
		return nil, inputError("empty_input", "at least one entry is required", errors.New("at least one entry is required"))
	}
	return entries, nil
}

func splitInputLines(input string) []string {
	input = strings.ReplaceAll(input, "\r\n", "\n")
	input = strings.ReplaceAll(input, "\r", "\n")
	return strings.Split(input, "\n")
}

func isHeaderLine(line string) bool {
	trimmed := strings.TrimLeft(line, " \t")
	if !strings.Contains(trimmed, "'") || strings.HasPrefix(trimmed, ";") {
		return false
	}
	fields := strings.Fields(trimmed)
	if len(fields) == 0 {
		return false
	}
	return looksLikeEntryTime(fields[0])
}

func looksLikeEntryTime(value string) bool {
	if len(value) < len("2006-01-02") {
		return false
	}
	for index, character := range "0000-00-00" {
		switch character {
		case '-':
			if value[index] != '-' {
				return false
			}
		default:
			if value[index] < '0' || value[index] > '9' {
				return false
			}
		}
	}
	return len(value) == len("2006-01-02") || value[len("2006-01-02")] == 'T'
}

func parseHeader(line string) (ledger.JournalEntry, error) {
	left, description, ok := strings.Cut(strings.TrimSpace(line), "'")
	if !ok {
		return ledger.JournalEntry{}, errors.New("header description is required")
	}
	occurredAt := strings.TrimSpace(left)
	description = strings.TrimSpace(description)
	if occurredAt == "" || description == "" {
		return ledger.JournalEntry{}, errors.New("header date and description are required")
	}
	date, err := ledger.ParseEntryTime(occurredAt)
	if err != nil {
		return ledger.JournalEntry{}, fmt.Errorf("invalid entry date or timestamp: %w", err)
	}
	return ledger.JournalEntry{Date: date, Description: description, Comments: []string{}, Postings: []ledger.Posting{}}, nil
}

func parsePosting(line string) (ledger.Posting, error) {
	body, comment, _ := strings.Cut(strings.TrimSpace(line), ";")
	fields := strings.Fields(body)
	if len(fields) != 1 && len(fields) != 3 {
		return ledger.Posting{}, errors.New("posting must be account or account amount commodity")
	}
	posting := ledger.Posting{Account: fields[0], Comment: strings.TrimSpace(comment)}
	if len(fields) == 1 {
		return posting, nil
	}
	amountText := strings.TrimPrefix(fields[1], "+")
	value, err := ledger.ParseDecimal(amountText)
	if err != nil {
		return ledger.Posting{}, fmt.Errorf("invalid posting amount: %w", err)
	}
	posting.Amount = &ledger.Amount{Value: value, Commodity: ledger.Commodity(fields[2])}
	return posting, nil
}

func validateParsedEntry(entry ledger.JournalEntry, index int) (ledger.JournalEntry, error) {
	if err := ledger.Validate(entry); err != nil {
		return ledger.JournalEntry{}, entryError(index+1, validationCode(err), validationMessage(err), err)
	}
	return entry, nil
}

func parseError(lineNumber int, err error) error {
	return lineError(lineNumber, parseCode(err), parseMessage(err), err)
}

func inputError(code, message string, err error) error {
	return &InputError{Code: code, Message: message, Err: err}
}

func lineError(lineNumber int, code, message string, err error) error {
	return &InputError{Line: lineNumber, Code: code, Message: message, Err: err}
}

func entryError(entryNumber int, code, message string, err error) error {
	return &InputError{Entry: entryNumber, Code: code, Message: message, Err: err}
}

func parseCode(err error) string {
	switch {
	case errors.Is(err, ledger.ErrInvalidEntryTime):
		return "invalid_header_date"
	case errors.Is(err, ledger.ErrInvalidDecimal):
		return "invalid_amount"
	case errors.Is(err, ledger.ErrDecimalOverflow):
		return "amount_overflow"
	}
	if strings.Contains(err.Error(), "description") {
		return "invalid_header"
	}
	if strings.Contains(err.Error(), "amount commodity") {
		return "invalid_posting_shape"
	}
	return "invalid_syntax"
}

func parseMessage(err error) string {
	switch parseCode(err) {
	case "invalid_header_date":
		return "entry header must start with YYYY-MM-DD or timezone-qualified RFC 3339 timestamp"
	case "invalid_amount":
		return "posting amount must be an integer or fixed-point decimal"
	case "amount_overflow":
		return "posting amount exceeds the supported decimal range"
	case "invalid_header":
		return "entry header must be date followed by single-quote description"
	case "invalid_posting_shape":
		return "posting must be account or account amount commodity"
	default:
		return "line is outside the supported Tackler subset"
	}
}

func validationCode(err error) string {
	switch {
	case errors.Is(err, ledger.ErrInvalidEntry):
		return "invalid_entry"
	case errors.Is(err, ledger.ErrInvalidAccount):
		return "invalid_account"
	case errors.Is(err, ledger.ErrInvalidCommodity):
		return "invalid_commodity"
	case errors.Is(err, ledger.ErrCommodityMismatch):
		return "commodity_mismatch"
	case errors.Is(err, ledger.ErrUnbalancedEntry):
		return "unbalanced_entry"
	case errors.Is(err, ledger.ErrInvalidOmitted):
		return "invalid_omitted_amount"
	}
	return "invalid_entry"
}

func validationMessage(err error) string {
	switch validationCode(err) {
	case "invalid_account":
		return "posting account is outside the supported account syntax"
	case "invalid_commodity":
		return "posting commodity is outside the supported commodity syntax"
	case "commodity_mismatch":
		return "all explicit posting amounts must use one commodity"
	case "unbalanced_entry":
		return "explicit posting amounts must balance or the final posting must omit its amount"
	case "invalid_omitted_amount":
		return "only the final posting may omit its amount"
	default:
		return "entry failed domain validation"
	}
}

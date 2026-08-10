package ingest

import (
	"errors"
	"fmt"

	"github.com/hirokinko/bokiccio/internal/ledger"
)

const (
	DiagnosticDuplicateInBatch  = "record.duplicate_in_batch"
	DiagnosticAlreadyCommitted  = "record.already_committed"
	DiagnosticInvalidEntry      = "ledger.invalid_entry"
	DiagnosticInvalidPosting    = "ledger.invalid_posting"
	DiagnosticInvalidAccount    = "ledger.invalid_account"
	DiagnosticInvalidCommodity  = "ledger.invalid_commodity"
	DiagnosticInvalidOmission   = "ledger.invalid_omission"
	DiagnosticDecimalOverflow   = "ledger.decimal_overflow"
	DiagnosticCommodityMismatch = "ledger.commodity_mismatch"
	DiagnosticUnbalanced        = "ledger.unbalanced_entry"
)

// Process converts decoded records to validated journal entries. committed
// contains identities from earlier runs; same-batch duplicates are detected
// independently in input order.
func Process(batch Batch, committed []RecordIdentity) ProcessResult {
	committedSet := make(map[RecordIdentity]struct{}, len(committed))
	for _, identity := range committed {
		committedSet[identity] = struct{}{}
	}
	seen := make(map[RecordIdentity]struct{}, len(batch.Records))
	result := ProcessResult{
		Outcomes: make([]Outcome, 0, len(batch.Records)),
		Entries:  make([]ledger.JournalEntry, 0, len(batch.Records)),
	}

	for index, record := range batch.Records {
		outcome := Outcome{RecordIndex: index, Identity: record.Identity, Source: record.Source}
		if _, exists := committedSet[record.Identity]; exists {
			outcome.Status = OutcomeDuplicate
			outcome.Diagnostics = []Diagnostic{duplicateDiagnostic(record.Identity, DiagnosticAlreadyCommitted, "record identity was committed by an earlier run")}
			result.Outcomes = append(result.Outcomes, outcome)
			continue
		}
		if _, exists := seen[record.Identity]; exists {
			outcome.Status = OutcomeDuplicate
			outcome.Diagnostics = []Diagnostic{duplicateDiagnostic(record.Identity, DiagnosticDuplicateInBatch, "record identity already occurred in this batch")}
			result.Outcomes = append(result.Outcomes, outcome)
			continue
		}
		seen[record.Identity] = struct{}{}

		entry, diagnostics := projectRecord(record)
		if err := ledger.Validate(entry); err != nil {
			outcome.Status = OutcomeError
			outcome.Diagnostics = append(diagnostics, domainDiagnostic(record.Identity, err))
			result.Outcomes = append(result.Outcomes, outcome)
			continue
		}

		if len(diagnostics) > 0 {
			outcome.Status = OutcomeWarning
		} else {
			outcome.Status = OutcomeSuccess
		}
		outcome.Diagnostics = diagnostics
		outcome.Entry = cloneJournalEntry(entry)
		result.Outcomes = append(result.Outcomes, outcome)
		result.Entries = append(result.Entries, entry)
	}
	return result
}

func projectRecord(record Record) (ledger.JournalEntry, []Diagnostic) {
	entry := ledger.JournalEntry{
		Date:        record.OccurredAt,
		Description: record.Description,
		Comments:    make([]string, 0, 1+len(record.Comments)+len(record.Warnings)),
		Postings:    make([]ledger.Posting, len(record.Postings)),
	}
	entry.Comments = append(entry.Comments, "source: "+record.Source.Display)
	entry.Comments = append(entry.Comments, record.Comments...)
	for index, posting := range record.Postings {
		entry.Postings[index] = ledger.Posting{
			Account: posting.Account,
			Amount:  cloneAmount(posting.Amount),
			Comment: posting.Comment,
		}
	}

	diagnostics := make([]Diagnostic, 0, len(record.Warnings))
	for _, warning := range record.Warnings {
		diagnostic := Diagnostic{
			Code:         warning.Code,
			Severity:     SeverityWarning,
			Message:      warning.Message,
			Identity:     record.Identity,
			FieldPath:    warning.FieldPath,
			PostingIndex: cloneIndex(warning.PostingIndex),
		}
		diagnostics = append(diagnostics, diagnostic)
		comment := fmt.Sprintf("WARN [%s]: %s", warning.Code, warning.Message)
		if warning.PostingIndex != nil && *warning.PostingIndex >= 0 && *warning.PostingIndex < len(entry.Postings) {
			posting := &entry.Postings[*warning.PostingIndex]
			if posting.Comment == "" {
				posting.Comment = comment
			} else {
				posting.Comment += " / " + comment
			}
			continue
		}
		entry.Comments = append(entry.Comments, comment)
	}
	return entry, diagnostics
}

func cloneAmount(amount *ledger.Amount) *ledger.Amount {
	if amount == nil {
		return nil
	}
	copy := *amount
	return &copy
}

func cloneJournalEntry(entry ledger.JournalEntry) *ledger.JournalEntry {
	copy := entry
	copy.Comments = append([]string(nil), entry.Comments...)
	copy.Postings = make([]ledger.Posting, len(entry.Postings))
	for index, posting := range entry.Postings {
		copy.Postings[index] = posting
		copy.Postings[index].Amount = cloneAmount(posting.Amount)
	}
	return &copy
}

func cloneIndex(index *int) *int {
	if index == nil {
		return nil
	}
	copy := *index
	return &copy
}

func duplicateDiagnostic(identity RecordIdentity, code, message string) Diagnostic {
	return Diagnostic{Code: code, Severity: SeverityInfo, Message: message, Identity: identity}
}

func domainDiagnostic(identity RecordIdentity, err error) Diagnostic {
	diagnostic := Diagnostic{
		Code:     DiagnosticInvalidEntry,
		Severity: SeverityError,
		Message:  err.Error(),
		Identity: identity,
	}
	var postingErr *ledger.PostingValidationError
	if errors.As(err, &postingErr) {
		diagnostic.PostingIndex = cloneIndex(&postingErr.Index)
	}

	switch {
	case errors.Is(err, ledger.ErrInvalidAccount):
		diagnostic.Code = DiagnosticInvalidAccount
		diagnostic.FieldPath = postingField(diagnostic.PostingIndex, "account")
	case errors.Is(err, ledger.ErrInvalidCommodity):
		diagnostic.Code = DiagnosticInvalidCommodity
		diagnostic.FieldPath = postingField(diagnostic.PostingIndex, "commodity")
	case errors.Is(err, ledger.ErrInvalidOmitted):
		diagnostic.Code = DiagnosticInvalidOmission
		diagnostic.FieldPath = postingField(diagnostic.PostingIndex, "amount")
	case errors.Is(err, ledger.ErrDecimalOverflow):
		diagnostic.Code = DiagnosticDecimalOverflow
		diagnostic.FieldPath = postingField(diagnostic.PostingIndex, "amount")
	case errors.Is(err, ledger.ErrCommodityMismatch):
		diagnostic.Code = DiagnosticCommodityMismatch
		diagnostic.FieldPath = postingField(diagnostic.PostingIndex, "commodity")
	case errors.Is(err, ledger.ErrUnbalancedEntry):
		diagnostic.Code = DiagnosticUnbalanced
	case errors.Is(err, ledger.ErrInvalidPosting):
		diagnostic.Code = DiagnosticInvalidPosting
	}
	return diagnostic
}

func postingField(index *int, field string) string {
	if index == nil {
		return ""
	}
	return fmt.Sprintf("postings[%d].%s", *index, field)
}

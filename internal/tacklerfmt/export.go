package tacklerfmt

import (
	"bytes"
	"errors"
	"fmt"

	"github.com/hirokinko/bokiccio/internal/ledger"
)

var (
	ErrInvalidOptions = errors.New("invalid Tackler export options")
)

// OmittedAmountMode controls how an omitted amount on the final posting is rendered.
type OmittedAmountMode uint8

const (
	PreserveOmitted OmittedAmountMode = iota + 1
	FillOmitted
)

type Options struct {
	OmittedAmounts OmittedAmountMode
}

// Export validates entries and returns their deterministic UTF-8 Tackler
// representation. It never returns partial output when validation fails.
func Export(entries []ledger.JournalEntry, options Options) ([]byte, error) {
	if options.OmittedAmounts != PreserveOmitted && options.OmittedAmounts != FillOmitted {
		return nil, fmt.Errorf("%w: OmittedAmounts must be PreserveOmitted or FillOmitted", ErrInvalidOptions)
	}

	inferred := make([]*ledger.Amount, len(entries))
	for entryIndex, entry := range entries {
		if err := ledger.Validate(entry); err != nil {
			return nil, fmt.Errorf("entry %d: %w", entryIndex, err)
		}
		if entry.Postings[len(entry.Postings)-1].Amount == nil && options.OmittedAmounts == FillOmitted {
			amount, err := ledger.InferFinalAmount(entry)
			if err != nil {
				return nil, fmt.Errorf("entry %d: %w", entryIndex, err)
			}
			inferred[entryIndex] = &amount
		}
	}

	var output bytes.Buffer
	for entryIndex, entry := range entries {
		if entryIndex > 0 {
			output.WriteByte('\n')
		}
		fmt.Fprintf(&output, "%s  '%s\n", entry.Date.String(), entry.Description)
		for _, comment := range entry.Comments {
			fmt.Fprintf(&output, "    ; %s\n", comment)
		}
		for _, posting := range entry.Postings {
			amount := posting.Amount
			if amount == nil {
				amount = inferred[entryIndex]
			}
			fmt.Fprintf(&output, "    %s", posting.Account)
			if amount != nil {
				fmt.Fprintf(&output, "  %s %s", amount.Value.String(), amount.Commodity)
			}
			if posting.TotalPrice != nil {
				fmt.Fprintf(&output, " = %s %s", posting.TotalPrice.Value.String(), posting.TotalPrice.Commodity)
			}
			if posting.Comment != "" {
				fmt.Fprintf(&output, "     ; %s", posting.Comment)
			}
			output.WriteByte('\n')
		}
	}
	return output.Bytes(), nil
}

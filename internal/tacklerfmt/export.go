package tacklerfmt

import (
	"bytes"
	"errors"
	"fmt"

	"new-accountbook/internal/ledger"
)

var (
	ErrInvalidOptions           = errors.New("invalid Tackler export options")
	ErrOmittedAmountUnsupported = errors.New("omitted posting amounts are not supported yet")
)

// OmittedAmountMode controls how an omitted amount on the final posting is
// rendered. Support for omitted amounts is completed in the next slice; the
// option is defined now to keep the Export API stable.
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

	for entryIndex, entry := range entries {
		if err := ledger.Validate(entry); err != nil {
			return nil, fmt.Errorf("entry %d: %w", entryIndex, err)
		}
		for postingIndex, posting := range entry.Postings {
			if posting.Amount == nil {
				return nil, fmt.Errorf("entry %d, posting %d: %w", entryIndex, postingIndex, ErrOmittedAmountUnsupported)
			}
		}
	}

	var output bytes.Buffer
	for entryIndex, entry := range entries {
		if entryIndex > 0 {
			output.WriteByte('\n')
		}
		fmt.Fprintf(&output, "%s  '%s\n", entry.Date.Format("2006-01-02"), entry.Description)
		for _, comment := range entry.Comments {
			fmt.Fprintf(&output, "    ; %s\n", comment)
		}
		for _, posting := range entry.Postings {
			amount := posting.Amount
			fmt.Fprintf(&output, "    %s  %s %s", posting.Account, amount.Value.String(), amount.Commodity)
			if posting.Comment != "" {
				fmt.Fprintf(&output, "     ; %s", posting.Comment)
			}
			output.WriteByte('\n')
		}
	}
	return output.Bytes(), nil
}

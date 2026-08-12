package ledger

import (
	"errors"
	"fmt"
	"math/big"
	"strings"
)

var (
	ErrInvalidEntry      = errors.New("invalid journal entry")
	ErrInvalidPosting    = errors.New("invalid posting")
	ErrInvalidAccount    = errors.New("invalid account")
	ErrInvalidCommodity  = errors.New("invalid commodity")
	ErrInvalidTotalPrice = errors.New("invalid total price")
	ErrCommodityMismatch = errors.New("posting commodities do not match")
	ErrUnbalancedEntry   = errors.New("journal entry is not balanced")
	ErrInvalidOmitted    = errors.New("only the final posting may omit its amount")
	ErrNoOmittedAmount   = errors.New("final posting does not omit its amount")
)

type Commodity string

type Amount struct {
	Value     Decimal
	Commodity Commodity
}

type Posting struct {
	Account    string
	Amount     *Amount
	TotalPrice *Amount
	Comment    string
}

type JournalEntry struct {
	Date        EntryTime
	Description string
	Comments    []string
	Postings    []Posting
}

// PostingValidationError identifies the posting whose domain validation
// failed while preserving the underlying sentinel error.
type PostingValidationError struct {
	Index int
	Err   error
}

func (err *PostingValidationError) Error() string {
	return fmt.Sprintf("posting %d: %v", err.Index, err.Err)
}

func (err *PostingValidationError) Unwrap() error {
	return err.Err
}

// Validate verifies the domain invariants required by the supported Tackler
// journal subset. It does not mutate the entry.
func Validate(entry JournalEntry) error {
	if err := entry.Date.validate(); err != nil {
		return fmt.Errorf("%w: %w", ErrInvalidEntry, err)
	}
	if strings.TrimSpace(entry.Description) == "" {
		return fmt.Errorf("%w: description is empty", ErrInvalidEntry)
	}
	if hasLineBreak(entry.Description) {
		return fmt.Errorf("%w: description contains a line break", ErrInvalidEntry)
	}
	for i, comment := range entry.Comments {
		if hasLineBreak(comment) {
			return fmt.Errorf("%w: comment %d contains a line break", ErrInvalidEntry, i)
		}
	}
	if len(entry.Postings) < 2 {
		return fmt.Errorf("%w: at least two postings are required", ErrInvalidEntry)
	}

	var commodity Commodity
	omitted := -1
	values := make([]Decimal, 0, len(entry.Postings))
	for i, posting := range entry.Postings {
		if err := validatePosting(posting); err != nil {
			return &PostingValidationError{Index: i, Err: err}
		}
		if posting.Amount == nil {
			if omitted >= 0 || i != len(entry.Postings)-1 {
				return &PostingValidationError{Index: i, Err: ErrInvalidOmitted}
			}
			omitted = i
			continue
		}
		effective := effectiveAmount(posting)
		if commodity == "" {
			commodity = effective.Commodity
		} else if commodity != effective.Commodity {
			return &PostingValidationError{Index: i, Err: fmt.Errorf("%w: %q and %q", ErrCommodityMismatch, commodity, effective.Commodity)}
		}
		values = append(values, effective.Value)
	}

	sum := exactSum(values)
	if omitted >= 0 {
		if _, err := decimalFromCoefficient(new(big.Int).Neg(new(big.Int).Set(sum.coefficient)), sum.scale); err != nil {
			return &PostingValidationError{Index: omitted, Err: fmt.Errorf("inferred amount: %w", err)}
		}
		return nil
	}
	if sum.coefficient.Sign() != 0 {
		return fmt.Errorf("%w: difference is %s %s", ErrUnbalancedEntry, formatFixedPoint(sum.coefficient, sum.scale), commodity)
	}
	return nil
}

// InferFinalAmount returns the exact balancing amount and commodity for an
// entry whose final posting omits its amount. The entry is validated first.
func InferFinalAmount(entry JournalEntry) (Amount, error) {
	if err := Validate(entry); err != nil {
		return Amount{}, err
	}
	if entry.Postings[len(entry.Postings)-1].Amount != nil {
		return Amount{}, ErrNoOmittedAmount
	}

	values := make([]Decimal, 0, len(entry.Postings)-1)
	commodity := effectiveAmount(entry.Postings[0]).Commodity
	for _, posting := range entry.Postings[:len(entry.Postings)-1] {
		values = append(values, effectiveAmount(posting).Value)
	}
	sum := exactSum(values)
	value, err := decimalFromCoefficient(new(big.Int).Neg(new(big.Int).Set(sum.coefficient)), sum.scale)
	if err != nil {
		return Amount{}, fmt.Errorf("inferred amount: %w", err)
	}
	return Amount{Value: value, Commodity: commodity}, nil
}

func validatePosting(posting Posting) error {
	if err := validateAccount(posting.Account); err != nil {
		return fmt.Errorf("%w: %w", ErrInvalidPosting, err)
	}
	if hasLineBreak(posting.Comment) {
		return fmt.Errorf("%w: comment contains a line break", ErrInvalidPosting)
	}
	if posting.Amount != nil {
		if err := validateCommodity(posting.Amount.Commodity); err != nil {
			return fmt.Errorf("%w: %w", ErrInvalidPosting, err)
		}
	}
	if posting.TotalPrice != nil {
		if posting.Amount == nil {
			return fmt.Errorf("%w: %w: amount is omitted", ErrInvalidPosting, ErrInvalidTotalPrice)
		}
		if err := validateCommodity(posting.TotalPrice.Commodity); err != nil {
			return fmt.Errorf("%w: %w: %w", ErrInvalidPosting, ErrInvalidTotalPrice, err)
		}
		if posting.Amount.Commodity == posting.TotalPrice.Commodity {
			return fmt.Errorf("%w: %w: posting and total price commodities must differ", ErrInvalidPosting, ErrInvalidTotalPrice)
		}
		if posting.Amount.Value.Sign() != posting.TotalPrice.Value.Sign() {
			return fmt.Errorf("%w: %w: posting and total price must have the same sign", ErrInvalidPosting, ErrInvalidTotalPrice)
		}
	}
	return nil
}

func effectiveAmount(posting Posting) *Amount {
	if posting.TotalPrice != nil {
		return posting.TotalPrice
	}
	return posting.Amount
}

type decimalSum struct {
	coefficient *big.Int
	scale       uint8
}

func exactSum(values []Decimal) decimalSum {
	var scale uint8
	for _, value := range values {
		scale = max(scale, value.scale)
	}
	sum := new(big.Int)
	for _, value := range values {
		sum.Add(sum, scaledCoefficient(value, scale))
	}
	// Retain arbitrary precision while summing so a temporary total cannot
	// overflow. An inferred final posting is range-checked separately.
	return decimalSum{coefficient: sum, scale: scale}
}

func validateAccount(account string) error {
	parts := strings.Split(account, ":")
	if len(parts) == 0 {
		return ErrInvalidAccount
	}
	for i, part := range parts {
		if !validIdentifierPart(part, i > 0) {
			return fmt.Errorf("%w: %q", ErrInvalidAccount, account)
		}
	}
	return nil
}

func validateCommodity(commodity Commodity) error {
	if !validIdentifierPart(string(commodity), false) {
		return fmt.Errorf("%w: %q", ErrInvalidCommodity, commodity)
	}
	return nil
}

func validIdentifierPart(value string, mayStartWithDigit bool) bool {
	runes := []rune(value)
	if len(runes) == 0 {
		return false
	}
	if !isNameStart(runes[0]) && !(mayStartWithDigit && isASCIIDigitRune(runes[0])) {
		return false
	}
	for _, r := range runes[1:] {
		if !isNameChar(r) {
			return false
		}
	}
	return true
}

func isNameChar(r rune) bool {
	return isNameStart(r) || isASCIIDigitRune(r) || r == '_' || r == '-' || r == '\u00B7' ||
		(r >= '\u0300' && r <= '\u036F') || (r >= '\u203F' && r <= '\u2040')
}

func isNameStart(r rune) bool {
	switch r {
	case '$', '¢', '£', '¤', '¥', 'µ', '¹', '²', '³', '°', '¼', '½', '¾':
		return true
	}
	return (r >= 'A' && r <= 'Z') ||
		(r >= 'a' && r <= 'z') ||
		(r >= '\u00C0' && r <= '\u00D6') ||
		(r >= '\u00D8' && r <= '\u00F6') ||
		(r >= '\u00F8' && r <= '\u02FF') ||
		(r >= '\u0370' && r <= '\u037D') ||
		(r >= '\u037F' && r <= '\u1FFF') ||
		(r >= '\u200C' && r <= '\u200D') ||
		(r >= '\u2070' && r <= '\u218F') ||
		(r >= '\u2C00' && r <= '\u2FEF') ||
		(r >= '\u3001' && r <= '\uD7FF') ||
		(r >= '\uF900' && r <= '\uFDCF') ||
		(r >= '\uFDF0' && r <= '\uFFFD')
}

func isASCIIDigitRune(r rune) bool {
	return r >= '0' && r <= '9'
}

func hasLineBreak(value string) bool {
	return strings.ContainsAny(value, "\r\n")
}

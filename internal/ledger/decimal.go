package ledger

import (
	"errors"
	"fmt"
	"math/big"
	"strings"
)

const MaxDecimalScale uint8 = 28

var (
	ErrInvalidDecimal  = errors.New("invalid decimal")
	ErrDecimalOverflow = errors.New("decimal exceeds 96-bit coefficient range")

	maxDecimalCoefficient = func() *big.Int {
		limit := new(big.Int).Lsh(big.NewInt(1), 96)
		return limit.Sub(limit, big.NewInt(1))
	}()
)

// Decimal is an immutable base-10 fixed-point value compatible with the
// coefficient and scale limits of the decimal implementation used by Tackler.
// Its zero value represents 0 with scale 0.
type Decimal struct {
	coefficient *big.Int
	scale       uint8
}

// ParseDecimal parses a non-exponent decimal with an optional leading minus.
// Trailing fractional zeroes and their scale are preserved.
func ParseDecimal(value string) (Decimal, error) {
	if value == "" {
		return Decimal{}, fmt.Errorf("%w: empty value", ErrInvalidDecimal)
	}

	negative := value[0] == '-'
	body := value
	if negative {
		body = value[1:]
		if body == "" {
			return Decimal{}, fmt.Errorf("%w: %q", ErrInvalidDecimal, value)
		}
	}

	if strings.Count(body, ".") > 1 {
		return Decimal{}, fmt.Errorf("%w: %q", ErrInvalidDecimal, value)
	}

	integerPart := body
	fractionPart := ""
	if dot := strings.IndexByte(body, '.'); dot >= 0 {
		integerPart = body[:dot]
		fractionPart = body[dot+1:]
		if fractionPart == "" {
			return Decimal{}, fmt.Errorf("%w: %q", ErrInvalidDecimal, value)
		}
	}
	if integerPart == "" || !isASCIIDigits(integerPart) || !isASCIIDigits(fractionPart) {
		return Decimal{}, fmt.Errorf("%w: %q", ErrInvalidDecimal, value)
	}
	if len(fractionPart) > int(MaxDecimalScale) {
		return Decimal{}, fmt.Errorf("%w: scale %d exceeds %d", ErrDecimalOverflow, len(fractionPart), MaxDecimalScale)
	}

	digits := strings.TrimLeft(integerPart+fractionPart, "0")
	if digits == "" {
		digits = "0"
	}
	// A 96-bit unsigned coefficient has at most 29 decimal digits. Checking
	// this before SetString also bounds work for clearly overflowing values.
	if len(digits) > len(maxDecimalCoefficient.String()) {
		return Decimal{}, fmt.Errorf("%w: %q", ErrDecimalOverflow, value)
	}

	coefficient, ok := new(big.Int).SetString(digits, 10)
	if !ok {
		return Decimal{}, fmt.Errorf("%w: %q", ErrInvalidDecimal, value)
	}
	if coefficient.Cmp(maxDecimalCoefficient) > 0 {
		return Decimal{}, fmt.Errorf("%w: %q", ErrDecimalOverflow, value)
	}
	if negative && coefficient.Sign() != 0 {
		coefficient.Neg(coefficient)
	}

	return Decimal{coefficient: coefficient, scale: uint8(len(fractionPart))}, nil
}

func isASCIIDigits(value string) bool {
	for i := 0; i < len(value); i++ {
		if value[i] < '0' || value[i] > '9' {
			return false
		}
	}
	return true
}

// Scale reports the number of decimal places retained by this value.
func (d Decimal) Scale() uint8 {
	return d.scale
}

// Sign returns -1, 0, or 1 according to the sign of the value.
func (d Decimal) Sign() int {
	return d.coefficientValue().Sign()
}

// String returns the exact fixed-point representation without exponent notation.
func (d Decimal) String() string {
	return formatFixedPoint(d.coefficientValue(), d.scale)
}

func formatFixedPoint(coefficient *big.Int, scale uint8) string {
	coefficient = new(big.Int).Set(coefficient)
	negative := coefficient.Sign() < 0
	coefficient.Abs(coefficient)
	digits := coefficient.String()

	if scale > 0 {
		decimalPlaces := int(scale)
		if len(digits) <= decimalPlaces {
			digits = strings.Repeat("0", decimalPlaces-len(digits)+1) + digits
		}
		split := len(digits) - decimalPlaces
		digits = digits[:split] + "." + digits[split:]
	}
	if negative {
		return "-" + digits
	}
	return digits
}

// Equal reports numerical equality, ignoring differences in retained scale.
func (d Decimal) Equal(other Decimal) bool {
	return d.Cmp(other) == 0
}

// Cmp compares two Decimal values numerically.
func (d Decimal) Cmp(other Decimal) int {
	scale := max(d.scale, other.scale)
	return scaledCoefficient(d, scale).Cmp(scaledCoefficient(other, scale))
}

// Add returns the exact sum with the larger operand scale. It does not round.
func (d Decimal) Add(other Decimal) (Decimal, error) {
	return d.calculate(other, false)
}

// Sub returns the exact difference with the larger operand scale. It does not round.
func (d Decimal) Sub(other Decimal) (Decimal, error) {
	return d.calculate(other, true)
}

func (d Decimal) calculate(other Decimal, subtract bool) (Decimal, error) {
	scale := max(d.scale, other.scale)
	left := scaledCoefficient(d, scale)
	right := scaledCoefficient(other, scale)
	if subtract {
		left.Sub(left, right)
	} else {
		left.Add(left, right)
	}
	return decimalFromCoefficient(left, scale)
}

// Neg returns a value with the opposite sign and the same scale.
func (d Decimal) Neg() Decimal {
	coefficient := d.coefficientValue()
	coefficient.Neg(coefficient)
	return Decimal{coefficient: coefficient, scale: d.scale}
}

func (d Decimal) coefficientValue() *big.Int {
	if d.coefficient == nil {
		return new(big.Int)
	}
	return new(big.Int).Set(d.coefficient)
}

func scaledCoefficient(d Decimal, scale uint8) *big.Int {
	coefficient := d.coefficientValue()
	if difference := scale - d.scale; difference > 0 {
		coefficient.Mul(coefficient, pow10(difference))
	}
	return coefficient
}

func pow10(exponent uint8) *big.Int {
	return new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(exponent)), nil)
}

func decimalFromCoefficient(coefficient *big.Int, scale uint8) (Decimal, error) {
	absolute := new(big.Int).Abs(new(big.Int).Set(coefficient))
	if scale > MaxDecimalScale || absolute.Cmp(maxDecimalCoefficient) > 0 {
		return Decimal{}, ErrDecimalOverflow
	}
	return Decimal{coefficient: new(big.Int).Set(coefficient), scale: scale}, nil
}

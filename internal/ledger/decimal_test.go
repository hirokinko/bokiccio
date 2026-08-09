package ledger

import (
	"errors"
	"testing"
)

func TestParseDecimalAndString(t *testing.T) {
	t.Parallel()
	tests := []struct {
		input string
		want  string
		scale uint8
	}{
		{input: "0", want: "0", scale: 0},
		{input: "123", want: "123", scale: 0},
		{input: "-123", want: "-123", scale: 0},
		{input: "1.20", want: "1.20", scale: 2},
		{input: "0.001", want: "0.001", scale: 3},
		{input: "-0.50", want: "-0.50", scale: 2},
		{input: "0.0000000000000000000000000000", want: "0.0000000000000000000000000000", scale: 28},
		{input: "0001.20", want: "1.20", scale: 2},
		{input: "79228162514264337593543950335", want: "79228162514264337593543950335", scale: 0},
		{input: "-79228162514264337593543950335", want: "-79228162514264337593543950335", scale: 0},
		{input: "0.1234567890123456789012345678", want: "0.1234567890123456789012345678", scale: 28},
	}
	for _, test := range tests {
		t.Run(test.input, func(t *testing.T) {
			got, err := ParseDecimal(test.input)
			if err != nil {
				t.Fatalf("ParseDecimal() error = %v", err)
			}
			if got.String() != test.want {
				t.Fatalf("String() = %q, want %q", got.String(), test.want)
			}
			if got.Scale() != test.scale {
				t.Fatalf("Scale() = %d, want %d", got.Scale(), test.scale)
			}
		})
	}
}

func TestParseDecimalRejectsInvalidOrOutOfRangeValues(t *testing.T) {
	t.Parallel()
	tests := []struct {
		input string
		err   error
	}{
		{input: "", err: ErrInvalidDecimal},
		{input: "-", err: ErrInvalidDecimal},
		{input: "+1", err: ErrInvalidDecimal},
		{input: ".1", err: ErrInvalidDecimal},
		{input: "1.", err: ErrInvalidDecimal},
		{input: "1e3", err: ErrInvalidDecimal},
		{input: "1.2.3", err: ErrInvalidDecimal},
		{input: "１", err: ErrInvalidDecimal},
		{input: "79228162514264337593543950336", err: ErrDecimalOverflow},
		{input: "0.12345678901234567890123456789", err: ErrDecimalOverflow},
	}
	for _, test := range tests {
		t.Run(test.input, func(t *testing.T) {
			_, err := ParseDecimal(test.input)
			if !errors.Is(err, test.err) {
				t.Fatalf("ParseDecimal() error = %v, want %v", err, test.err)
			}
		})
	}
}

func TestDecimalArithmeticIsExactAndPreservesOperands(t *testing.T) {
	t.Parallel()
	left := mustDecimal(t, "1.20")
	right := mustDecimal(t, "2.3")

	sum, err := left.Add(right)
	if err != nil {
		t.Fatalf("Add() error = %v", err)
	}
	if sum.String() != "3.50" {
		t.Fatalf("Add() = %s, want 3.50", sum.String())
	}
	difference, err := left.Sub(right)
	if err != nil {
		t.Fatalf("Sub() error = %v", err)
	}
	if difference.String() != "-1.10" {
		t.Fatalf("Sub() = %s, want -1.10", difference.String())
	}
	if left.String() != "1.20" || right.String() != "2.3" {
		t.Fatalf("operands mutated: left=%s right=%s", left.String(), right.String())
	}
	if !mustDecimal(t, "1.0").Equal(mustDecimal(t, "1.00")) {
		t.Fatal("Equal() rejected numerically equal values with different scales")
	}
	if left.Neg().String() != "-1.20" || left.String() != "1.20" {
		t.Fatal("Neg() did not preserve scale or mutated its receiver")
	}
}

func TestDecimalArithmeticReportsOverflow(t *testing.T) {
	t.Parallel()
	maximum := mustDecimal(t, "79228162514264337593543950335")
	_, err := maximum.Add(mustDecimal(t, "1"))
	if !errors.Is(err, ErrDecimalOverflow) {
		t.Fatalf("Add() error = %v, want ErrDecimalOverflow", err)
	}
}

func TestDecimalZeroValue(t *testing.T) {
	t.Parallel()
	var zero Decimal
	if zero.String() != "0" || zero.Scale() != 0 || zero.Sign() != 0 {
		t.Fatalf("zero value = %s scale=%d sign=%d", zero.String(), zero.Scale(), zero.Sign())
	}
}

func mustDecimal(t *testing.T, value string) Decimal {
	t.Helper()
	decimal, err := ParseDecimal(value)
	if err != nil {
		t.Fatalf("ParseDecimal(%q): %v", value, err)
	}
	return decimal
}

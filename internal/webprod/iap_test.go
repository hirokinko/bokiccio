package webprod

import (
	"encoding/base64"
	"testing"
)

func TestRequireES256(t *testing.T) {
	encode := func(value string) string { return base64.RawURLEncoding.EncodeToString([]byte(value)) }
	tests := []struct {
		token string
		ok    bool
	}{
		{token: encode(`{"alg":"ES256","kid":"key"}`) + ".payload.signature", ok: true},
		{token: encode(`{"alg":"RS256","kid":"key"}`) + ".payload.signature"},
		{token: encode(`{"alg":"none"}`) + ".payload.signature"},
		{token: "malformed"},
	}
	for _, test := range tests {
		err := requireES256(test.token)
		if (err == nil) != test.ok {
			t.Fatalf("requireES256(%q) error = %v, ok=%t", test.token, err, test.ok)
		}
	}
}

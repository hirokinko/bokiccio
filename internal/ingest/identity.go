package ingest

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"hash"
	"strconv"
	"strings"
	"time"

	"github.com/hirokinko/bokiccio/internal/ledger"
)

const IdentityAlgorithmVersion = 1

type IdentityKind string

const (
	IdentityExternalID  IdentityKind = "external_id"
	IdentityFingerprint IdentityKind = "fingerprint"
)

type RecordIdentity struct {
	Kind             IdentityKind
	AlgorithmVersion int
	Digest           [sha256.Size]byte
}

func (identity RecordIdentity) HexDigest() string {
	return hex.EncodeToString(identity.Digest[:])
}

func resolveIdentity(schemaVersion int, record Record) RecordIdentity {
	if record.Source.ExternalID != "" {
		return hashIdentity(IdentityExternalID,
			"bokiccio.record-identity.external-id",
			"v1",
			record.Source.Namespace,
			record.Source.ExternalID,
		)
	}

	fields := []string{
		"bokiccio.record-identity.fingerprint",
		"v1",
		canonicalInteger(schemaVersion),
		record.Source.Namespace,
		canonicalEntryTime(record.OccurredAt),
		strings.TrimSpace(record.Description),
	}
	for _, posting := range record.Postings {
		fields = append(fields, "posting", posting.Account)
		if posting.Amount == nil {
			fields = append(fields, "omitted", "", "")
			continue
		}
		fields = append(fields,
			"explicit",
			canonicalDecimal(posting.Amount.Value),
			string(posting.Amount.Commodity),
		)
		if schemaVersion >= SchemaVersion {
			if posting.TotalPrice == nil {
				fields = append(fields, "no_total_price", "", "")
			} else {
				fields = append(fields,
					"total_price",
					canonicalDecimal(posting.TotalPrice.Value),
					string(posting.TotalPrice.Commodity),
				)
			}
		}
	}
	return hashIdentity(IdentityFingerprint, fields...)
}

func hashIdentity(kind IdentityKind, fields ...string) RecordIdentity {
	digest := sha256.New()
	for _, field := range fields {
		writeIdentityField(digest, field)
	}
	var value [sha256.Size]byte
	copy(value[:], digest.Sum(nil))
	return RecordIdentity{Kind: kind, AlgorithmVersion: IdentityAlgorithmVersion, Digest: value}
}

func writeIdentityField(destination hash.Hash, field string) {
	var length [8]byte
	binary.BigEndian.PutUint64(length[:], uint64(len(field)))
	_, _ = destination.Write(length[:])
	_, _ = destination.Write([]byte(field))
}

func canonicalEntryTime(value ledger.EntryTime) string {
	if value.Precision() == ledger.EntryDate {
		return "date:" + value.String()
	}
	return "datetime:" + value.Time().UTC().Format(time.RFC3339Nano)
}

func canonicalDecimal(value ledger.Decimal) string {
	if value.Sign() == 0 {
		return "0"
	}
	text := value.String()
	if strings.Contains(text, ".") {
		text = strings.TrimRight(text, "0")
		text = strings.TrimSuffix(text, ".")
	}
	return text
}

func canonicalInteger(value int) string {
	return strconv.Itoa(value)
}

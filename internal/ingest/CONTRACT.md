# Normalized input v1

`ingest.Decode` accepts one UTF-8 JSON object. Unknown fields, trailing JSON
values, missing required fields, and unsupported schema versions are rejected.
The decoder uses only the Go standard library and the `ledger` domain package.

```json
{
  "schema_version": 1,
  "records": [
    {
      "source": {
        "namespace": "receipt",
        "display": "receipts/sample-001.jpg",
        "external_id": "sample-001"
      },
      "occurred_at": "2026-08-10T14:30:00+09:00",
      "description": "サンプル店舗",
      "comments": ["要確認"],
      "postings": [
        {
          "account": "費用:食費",
          "amount": "500.00",
          "commodity": "JPY",
          "comment": "サンプル商品"
        },
        {
          "account": "資産:現金",
          "amount": "-500.00",
          "commodity": "JPY"
        }
      ]
    }
  ]
}
```

## Fields

- `schema_version` must be the integer `1`.
- `records` is required and may be empty.
- `source.namespace` and `source.display` are required single-line strings.
  Display sources are relative paths or URIs without credentials or query
  strings. Absolute local paths are rejected.
- `source.external_id` is optional. When present, it must be a non-empty
  single-line string without surrounding whitespace.
- `occurred_at` is either `YYYY-MM-DD` or a timezone-qualified RFC 3339
  timestamp.
- `description` is a required non-empty single-line string. `comments` is an
  optional array of single-line strings.
- `postings` contains at least two postings in accounting order.
- `account` is required. `amount` and `commodity` must either both be present
  as strings or both be omitted from the final posting. `comment` is optional.
- Decimal amounts retain their input scale and never pass through a JSON
  number or floating-point value.

Account, commodity, balance, and inferred-final-amount domain validation is
performed by the record processor in the next boundary, not by the wire DTO.

## Stable identity

Each decoded record receives a SHA-256 identity with algorithm version `1`.
Fields are hashed in the documented order using an unsigned 64-bit big-endian
byte length followed by the UTF-8 field bytes.

- With an external ID, the projection is
  `bokiccio.record-identity.external-id`, `v1`, source namespace, and external
  ID.
- Without an external ID, the projection is
  `bokiccio.record-identity.fingerprint`, `v1`, schema major version, source
  namespace, canonical entry time, trimmed description, and the ordered
  accounting fields of every posting.
- Fingerprint decimals discard insignificant trailing fractional zeroes and
  normalize negative zero to zero. Datetimes are canonicalized to UTC, while
  date-only values remain distinct from midnight datetimes.
- Display source, record order, comments, warnings, and diagnostics are not
  part of the identity.

Equal identities are retained in the decoded batch. The processor classifies
same-batch and previously committed identities as `duplicate` outcomes.

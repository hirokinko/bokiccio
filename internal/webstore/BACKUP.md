# Logical backup format v1

Bokiccioのproduction backupはTursoのdatabase fileやSQL dumpではなく、driver非依存のlogical JSONを使う。
backupは仕訳、source、diagnostic、import report、revision、approvalを含むprivate dataであり、暗号化されていない。
保存先のaccess controlや暗号化は運用環境で行う。

## Envelope

format version 1のtop-level fieldは次のとおりである。

- `format`: `bokiccio-logical-backup`
- `format_version`: `1`
- `schema_version`: backup元のWeb storage schema version
- `created_at`: RFC 3339 timestamp
- `payload_sha256`: canonical payload JSONのlowercase SHA-256
- `row_counts`: payload sectionごとの件数
- `payload`: schema v2またはv3 application data

payloadは全tableを依存順、各table内をprimary key順で保持する。SQL BLOBはJSONのbase64 stringとして
losslessにencodeする。`workflow_state`と`sqlite_sequence`の対象counterも保持する。
schema metadataそのもの、Turso credential、database URL、IAP audience、owner emailは含めない。

formatは手編集を想定しない。restoreはunknown field、trailing JSON、field欠落、format/schema version不一致、
checksum・row count不一致を拒否する。

## Backup

```console
TURSO_DATABASE_URL='libsql://database-name.turso.io' \
TURSO_AUTH_TOKEN='secret-manager-injected-token' \
go run ./cmd/bokiccio backup --output ./bokiccio-backup.json
```

backupは同一directoryのtemporary fileを完成・syncしてから指定pathへatomicに公開する。permissionは`0600`で、
既存fileを上書きしない。backup内容やpathをstdout/stderrへ出さない。

## Restore

restore先は`bokiccio migrate`でcurrent schemaへ移行済み、かつapplication dataと対象sequenceが空でなければ
ならない。

```console
TURSO_DATABASE_URL='libsql://new-empty-database.turso.io' \
TURSO_AUTH_TOKEN='secret-manager-injected-token' \
go run ./cmd/bokiccio migrate

TURSO_DATABASE_URL='libsql://new-empty-database.turso.io' \
TURSO_AUTH_TOKEN='secret-manager-injected-token' \
go run ./cmd/bokiccio restore --input ./bokiccio-backup.json
```

restoreはmerge、既存dataのreplace、schema migrationを行わない。全rowを1 transactionでinsertし、row count、
foreign key、report metadata、identity、entry/revision domain validation、approval relationshipを検証してから
commitする。いずれかが失敗した場合、target dataを変更しない。

format version `1`はDB schema version `2`と`3`を受け付ける。schema v3はpostingとrevision postingへ
optionalなtotal-price amount・scale・commodityを追加する。schema v2 backupにはこれらのfieldがなく、current
schemaへrestoreするとNULLとして保持される。これ以外のschema versionは明示的な変換pathがないため拒否する。

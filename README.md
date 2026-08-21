# Bokiccio

**ぶきっちょでも、複式簿記。**

Bokiccio（ボキッチョ）は、複数の明細・メール・レシートから仕訳を生成し、小規模な会計SaaSへ発展させるためのGoプロジェクトです。プレーンテキストを中心に、キーボードで素早く入力・確認・修正できる会計workflowを目指します。

名前は、メモ帳のように扱える「簿記帖」と、会計が得意でなくても使える「ぶきっちょ」に由来します。

現在は初期基盤の段階です。Tacklerから独立した仕訳ドメイン、Tackler journal formatの互換subset exporter、ローカル取込workflow、Tursoへ永続化するsingle-user Web APIと日本語・英語対応のWeb画面を実装しています。外部サービス連携はまだありません。

## 実装済み

- `JournalEntry`、`Posting`、`Amount`、`Commodity`による仕訳モデル
- 96-bit係数、scale 0〜28の10進固定小数点
- 単一commodity仕訳の検証と正確な貸借集計
- posting数量と総額を分けた複数commodity換算の検証と貸借集計
- 最終postingの金額省略と残額推論
- 決定的なTackler互換subset入出力
- source、WARN、割引などの単一行コメント
- version付き正規化JSONからの仕訳候補decodeとstable identity生成
- record単位のsuccess・warning・error・duplicate処理結果
- version付きreport・deduplication stateと決定的なrun identity
- immutable run bundleとstate manifestを最後にcommitする安全なローカル公開
- `bokiccio import`による外部service不要のローカル取込
- Turso互換schemaとJSON APIによるlocal Web vertical slice
- Turso Cloud remote driver、明示migration、Cloud Run向けHTTP server
- Cloud Run direct IAP JWT、IAP-authorized user、same-origin mutationの検証
- templによる型付きserver-side renderingの日本語・英語対応仕訳検索・取込履歴閲覧画面
- Web UIからのnormalized JSON upload
- immutableな仕訳revision、domain再validation、append-onlyな承認履歴
- 最新revisionを対象にした日付・勘定科目・摘要・状態・source検索
- 現在承認済みの仕訳だけを対象にしたTackler/JSON export
- 明示的な5区分・会計年度・期首残高方式を履歴管理するreporting設定
- 承認済み最新仕訳をcommodity別・勘定科目階層別に集計する月次・年度試算表
- 試算表と単月損益計算書のaccount集計値から承認済み仕訳・寄与postingへ戻るdrill-down
- 参照日時点の資産・負債・純資産と、独立して選択した月の費用を分けて確認する現在overview
- 年度別の期首貸借対照表、当期損益を表示補正する期末貸借対照表、単月損益計算書、12か月の全勘定残高推移
- reporting設定を含むchecksum付きlogical backupと空database限定のtransactional restore
- 匿名化fixtureによるgolden test
- Tackler 26.1.2以降を使った任意実行の互換性test

## 設計原則

- ドメインではTackler固有の`txn`という名称を使わず、`JournalEntry`を使用する。
- 金額を`float32`または`float64`へ変換せず、固定小数点として正確に扱う。
- Tackler固有の表現はexporterへ閉じ込め、完全互換ではなく対応subsetを明示する。
- ドメインをHTTP、DB、Google API、ファイルシステムから独立させる。
- Tree-sitterによる構文解析とLSPによる意味検証は、会計ドメインから分離する。

現在の構成は次のとおりです。

```text
internal/ledger
  ├─ Decimal
  ├─ JournalEntry / Posting / Amount / Commodity
  └─ validation and balance inference

internal/reporting
  ├─ classification and fiscal calendar
  ├─ opening balance policy and exact aggregation
  └─ commodity-separated reports, entry provenance, and account drill-down

internal/tacklerfmt
  ├─ Tackler-compatible subset parser and exporter
  ├─ golden tests
  └─ compatibility fixtures and integration test

internal/ingest
  ├─ normalized input v1/v2 decoder
  ├─ application candidate values
  ├─ source-based identity and accounting fingerprint
  ├─ record processor, outcome, and structured diagnostic
  └─ deterministic report, state, and safe run publication

cmd/bokiccio
  ├─ local import command
  ├─ Turso migration command
  ├─ Turso logical backup/restore commands
  └─ IAP-protected production server

internal/webapp / internal/webstore / internal/webprod
  ├─ HTTP handler and database/sql persistence
  ├─ IAP-authorized user and origin boundary
  ├─ revision, approval, search, and approved export
  ├─ reporting configuration、financial reports、logical backup/restore
  └─ Turso Cloud production composition

internal/webui
  ├─ templによる型付きserver-side rendering
  ├─ normalized JSON upload、仕訳検索、取込履歴の日本語・英語画面
  └─ 同梱したCSS、htmx asset、development-only Biome設定
```

正規化入力のfieldとidentity規約は[normalized input v1/v2 contract](internal/ingest/CONTRACT.md)、outcomeとdiagnosticの規約は[record processing contract](internal/ingest/PROCESSING.md)、report・state・公開手順は[run artifact and publication contract](internal/ingest/RUNS.md)にまとめています。

## 開発と検証

必要なもの：

- Go 1.26以降
- Node.js/npm（CSS/JavaScriptのlint・formatを実行する場合だけ）
- Tackler 26.1.2以降（互換性testを実行する場合だけ）

通常のtestは外部サービスやTackler CLIを必要としません。

```sh
go test ./...
go test -race ./...
go vet ./...
```

Web UIのfirst-party CSS/JavaScriptはBiomeでformat・lintします。Node/npmはdevelopment用であり、Go build、Cloud Run runtime、templ生成、asset配信には使用しません。

```sh
npm ci
npm run format
npm run check
```

Tackler CLIを含む互換性testはbuild tagを付けて実行します。

```sh
go test -tags=tackler_integration ./internal/tacklerfmt
```

## ローカルimport

version 1または2の正規化JSONを、明示したoutput rootへ安全に取り込みます。

```sh
go run ./cmd/bokiccio import \
  --input ./internal/ingest/testdata/valid-v1.json \
  --output ./bokiccio-output
```

成果物は`bokiccio-output/runs/<run-identity>/`のimmutable bundleとして作成され、deduplication stateは`bokiccio-output/state-v1.json`へ保存されます。採用可能な仕訳があるbundleには`journal.txn`、すべてのrunには`report.json`が入ります。

終了codeは、error outcomeなしが`0`、runが完了してreportを公開したもののrecord単位のerrorが残る場合が`1`、usage・schema・I/Oなどrun全体の失敗が`2`です。処理件数と相対bundle pathはstderrへ要約されます。

入力formatは[normalized input v1/v2 contract](internal/ingest/CONTRACT.md)、出力配置と再実行規約は[run artifact and publication contract](internal/ingest/RUNS.md)を参照してください。

## Turso migrationとproduction server

Turso Cloudのschema migrationはserver起動と分離して明示実行します。

```sh
TURSO_DATABASE_URL=libsql://database-name.turso.io \
TURSO_AUTH_TOKEN='secret-manager-injected-token' \
go run ./cmd/bokiccio migrate
```

production serverはCloud Run direct IAPを前提とします。Cloud Run側でもunauthenticated invocationを禁止し、
許可するGoogle AccountへIAP accessを付与してください。applicationはsigned IAP JWTの署名、issuer、audience、subject、email、時刻を検証し、
application全体へのaccess allowlistはIAP IAM policyへ委譲します。IAPで許可された未登録userは共有dataを閲覧・検索・exportできますが、
file upload、revision作成、approval、reporting設定変更は、検証済みemailがdatabaseの`data_write_principals` allowlistへ
登録されている場合だけ許可します。Web UI/APIに削除機能はありません。

```sh
TURSO_DATABASE_URL=libsql://database-name.turso.io \
TURSO_AUTH_TOKEN='secret-manager-injected-token' \
BOKICCIO_IAP_AUDIENCE='/projects/123456789/locations/asia-northeast1/services/bokiccio' \
BOKICCIO_EXTERNAL_ORIGIN='https://bokiccio.example.com' \
PORT=8080 \
go run ./cmd/bokiccio serve
```

`TURSO_AUTH_TOKEN`はCloud RunのSecret Manager environment injectionで渡し、image、source、通常の環境設定へ保存しません。`BOKICCIO_EXTERNAL_ORIGIN`は利用者がアクセスするHTTPS originそのものとし、末尾pathを含めません。複数のCloud Run URLやcustom domainを併用する場合は、comma-separated HTTPS originsとして指定します。serverはmigrationを暗黙実行せず、schemaがcurrentでなければ起動しません。

route、JSON、認証境界、remote integration testは[Web API v1](internal/webapp/API.md)、画面routeとassetの構成は[Web UI](internal/webui/UI.md)を参照してください。

data変更可否はemail allowlist、file upload可否はさらにdatabase-wideのtyped settingで管理します。global settingを無効化すると日本語・英語UIの
normalized JSONとTackler `.txn`のupload formが非表示になり、UI/APIのdirect importも`403 upload_disabled`で拒否されます。
global settingが有効でも、検証済みemailがallowlistになければdirect importは`403 upload_forbidden`、その他のmutationは
`403 write_forbidden`で拒否されます。allowlistは当面operatorがdatabaseで手動管理し、Web UI/APIからは変更できません。

```sh
TURSO_DATABASE_URL=libsql://database-name.turso.io \
TURSO_AUTH_TOKEN='secret-manager-injected-token' \
go run ./cmd/bokiccio settings set --file-upload-enabled=false
```

成功時は新しいboolean値だけを出力します。Web UI、JSON API、Terraformからこの設定は変更できません。

## Turso backupとrestore

backupは仕訳、source、diagnostic、revision、approval、reporting設定履歴、file upload設定、data変更許可emailを含む暗号化されていないprivate dataである。
既存fileを上書きせず、permission `0600`のlogical JSONとして作成する。

```sh
TURSO_DATABASE_URL=libsql://database-name.turso.io \
TURSO_AUTH_TOKEN='secret-manager-injected-token' \
go run ./cmd/bokiccio backup --output ./bokiccio-backup.json
```

restore先はcurrent schemaへmigration済みの空databaseに限定され、mergeや既存dataのreplaceは行わない。

```sh
TURSO_DATABASE_URL=libsql://new-empty-database.turso.io \
TURSO_AUTH_TOKEN='secret-manager-injected-token' \
go run ./cmd/bokiccio restore --input ./bokiccio-backup.json
```

format、checksum、transactional validationの詳細は
[logical backup format v1](internal/webstore/BACKUP.md)を参照してください。

### Production operation checklist

backup/restoreはWeb UIから実行せず、operator CLIで行います。restore前にtarget databaseへ`bokiccio migrate`を
実行し、application dataが空であることを前提にしてください。restore後はproduction serverを起動し、
IAPで許可したaccountから仕訳検索、entry詳細、承認済みexportを確認します。

Cloud Runの複数URLやcustom domainを併用する場合、`BOKICCIO_EXTERNAL_ORIGIN`には利用するHTTPS originを
comma-separatedで設定します。shellや`gcloud --update-env-vars`ではcommaを区切り文字として扱うため、
必要に応じて`gcloud topic escaping`またはflags fileを使ってください。

## Tackler互換subset

parserとexporterは日付、timezone付きRFC 3339日時、摘要、取引・postingコメント、勘定科目、固定小数点金額、明示commodity、`=`によるtotal-price value position、最終postingの金額省略を扱います。timezoneなしの日時、transaction code、metadata、`@`によるunit price、opening position、原価、price database directiveなどは対象外です。

対応構文、固定version、fixture構成の詳細は[互換性契約](internal/tacklerfmt/COMPATIBILITY.md)を参照してください。

## 現在の非対応範囲

- テナント管理
- production deployment manifestと自動migration
- Gmail、Google Drive、Cloud Vision、Vertex AIとの連携
- 定期batchとjob管理
- Tree-sitter grammar、WebAssembly parser、LSP server
- 既存データの自動移行

今後の段階と未決定事項は[ロードマップ](ROADMAP.md)にまとめています。

## ライセンス

このプロジェクトは[Apache License 2.0](LICENSE)の下で公開されています。

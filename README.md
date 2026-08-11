# Bokiccio

**ぶきっちょでも、複式簿記。**

Bokiccio（ボキッチョ）は、複数の明細・メール・レシートから仕訳を生成し、小規模な会計SaaSへ発展させるためのGoプロジェクトです。プレーンテキストを中心に、キーボードで素早く入力・確認・修正できる会計workflowを目指します。

名前は、メモ帳のように扱える「簿記帖」と、会計が得意でなくても使える「ぶきっちょ」に由来します。

現在は初期基盤の段階です。Tacklerから独立した仕訳ドメイン、Tackler journal formatの互換subset exporter、ローカル取込workflow、Tursoへ永続化するsingle-user Web APIを実装しています。Web画面と外部サービス連携はまだありません。

## 実装済み

- `JournalEntry`、`Posting`、`Amount`、`Commodity`による仕訳モデル
- 96-bit係数、scale 0〜28の10進固定小数点
- 単一commodity仕訳の検証と正確な貸借集計
- 最終postingの金額省略と残額推論
- 決定的なTackler互換subset出力
- source、WARN、割引などの単一行コメント
- version付き正規化JSONからの仕訳候補decodeとstable identity生成
- record単位のsuccess・warning・error・duplicate処理結果
- version付きreport・deduplication stateと決定的なrun identity
- immutable run bundleとstate manifestを最後にcommitする安全なローカル公開
- `bokiccio import`による外部service不要のローカル取込
- Turso互換schemaとread-only JSON APIによるlocal Web vertical slice
- Turso Cloud remote driver、明示migration、Cloud Run向けHTTP server
- Cloud Run direct IAP JWT、single-owner、same-origin mutationの検証
- immutableな仕訳revision、domain再validation、append-onlyな承認履歴
- 最新revisionを対象にした日付・勘定科目・摘要・状態・source検索
- 現在承認済みの仕訳だけを対象にしたTackler/JSON export
- 匿名化fixtureによるgolden test
- Tackler 26.1.2を使った任意実行の互換性test

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

internal/tacklerfmt
  ├─ Tackler-compatible subset exporter
  ├─ golden tests
  └─ compatibility fixtures and integration test

internal/ingest
  ├─ normalized input v1 decoder
  ├─ application candidate values
  ├─ source-based identity and accounting fingerprint
  ├─ record processor, outcome, and structured diagnostic
  └─ deterministic report, state, and safe run publication

cmd/bokiccio
  ├─ local import command
  ├─ Turso migration command
  └─ IAP-protected production server

internal/webapp / internal/webstore / internal/webprod
  ├─ HTTP handler and database/sql persistence
  ├─ single-owner IAP and origin boundary
  ├─ revision, approval, search, and approved export
  └─ Turso Cloud production composition
```

正規化入力のfieldとidentity規約は[normalized input v1 contract](internal/ingest/CONTRACT.md)、outcomeとdiagnosticの規約は[record processing contract](internal/ingest/PROCESSING.md)、report・state・公開手順は[run artifact and publication contract](internal/ingest/RUNS.md)にまとめています。

## 開発と検証

必要なもの：

- Go 1.26以降
- Tackler 26.1.2（互換性testを実行する場合だけ）

通常のtestは外部サービスやTackler CLIを必要としません。

```sh
go test ./...
go test -race ./...
go vet ./...
```

Tackler CLIを含む互換性testはbuild tagを付けて実行します。

```sh
go test -tags=tackler_integration ./internal/tacklerfmt
```

## ローカルimport

version 1の正規化JSONを、明示したoutput rootへ安全に取り込みます。

```sh
go run ./cmd/bokiccio import \
  --input ./internal/ingest/testdata/valid-v1.json \
  --output ./bokiccio-output
```

成果物は`bokiccio-output/runs/<run-identity>/`のimmutable bundleとして作成され、deduplication stateは`bokiccio-output/state-v1.json`へ保存されます。採用可能な仕訳があるbundleには`journal.txn`、すべてのrunには`report.json`が入ります。

終了codeは、error outcomeなしが`0`、runが完了してreportを公開したもののrecord単位のerrorが残る場合が`1`、usage・schema・I/Oなどrun全体の失敗が`2`です。処理件数と相対bundle pathはstderrへ要約されます。

入力formatは[normalized input v1 contract](internal/ingest/CONTRACT.md)、出力配置と再実行規約は[run artifact and publication contract](internal/ingest/RUNS.md)を参照してください。

## Turso migrationとproduction server

Turso Cloudのschema migrationはserver起動と分離して明示実行します。

```sh
TURSO_DATABASE_URL=libsql://database-name.turso.io \
TURSO_AUTH_TOKEN='secret-manager-injected-token' \
go run ./cmd/bokiccio migrate
```

production serverはCloud Run direct IAPを前提とします。Cloud Run側でもunauthenticated invocationを禁止し、ownerのGoogle AccountだけへIAP accessを付与してください。applicationはsigned IAP JWTとowner emailを再検証します。

```sh
TURSO_DATABASE_URL=libsql://database-name.turso.io \
TURSO_AUTH_TOKEN='secret-manager-injected-token' \
BOKICCIO_IAP_AUDIENCE='/projects/123456789/locations/asia-northeast1/services/bokiccio' \
BOKICCIO_OWNER_EMAIL='owner@example.com' \
BOKICCIO_EXTERNAL_ORIGIN='https://bokiccio.example.com' \
PORT=8080 \
go run ./cmd/bokiccio serve
```

`TURSO_AUTH_TOKEN`はCloud RunのSecret Manager environment injectionで渡し、image、source、通常の環境設定へ保存しません。`BOKICCIO_EXTERNAL_ORIGIN`は利用者がアクセスするHTTPS originそのものとし、末尾pathを含めません。serverはmigrationを暗黙実行せず、schemaがcurrentでなければ起動しません。

route、JSON、認証境界、remote integration testは[Web API v1](internal/webapp/API.md)を参照してください。

## Tackler互換subset

exporterは日付、timezone付きRFC 3339日時、摘要、取引・postingコメント、勘定科目、固定小数点金額、明示commodity、最終postingの金額省略を扱います。timezoneなしの日時、transaction code、metadata、価格・原価、commodity換算などは対象外です。

対応構文、固定version、fixture構成の詳細は[互換性契約](internal/tacklerfmt/COMPATIBILITY.md)を参照してください。

## 現在の非対応範囲

- Tackler journal parser
- Web UI、テナント管理
- production deployment manifestと自動migration
- Gmail、Google Drive、Cloud Vision、Vertex AIとの連携
- 定期batchとjob管理
- Tree-sitter grammar、WebAssembly parser、LSP server
- 既存データの自動移行

今後の段階と未決定事項は[ロードマップ](ROADMAP.md)にまとめています。

## ライセンス

このプロジェクトは[Apache License 2.0](LICENSE)の下で公開されています。

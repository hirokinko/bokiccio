# Bokiccio

**ぶきっちょでも、複式簿記。**

Bokiccio（ボキッチョ）は、複数の明細・メール・レシートから仕訳を生成し、小規模な会計SaaSへ発展させるためのGoプロジェクトです。プレーンテキストを中心に、キーボードで素早く入力・確認・修正できる会計workflowを目指します。

名前は、メモ帳のように扱える「簿記帖」と、会計が得意でなくても使える「ぶきっちょ」に由来します。

現在は初期基盤の段階です。Tacklerから独立した仕訳ドメインと、Tackler journal formatの互換subset exporterまでを実装しています。Web API、永続化、認証、画面、外部サービス連携はまだありません。

## 実装済み

- `JournalEntry`、`Posting`、`Amount`、`Commodity`による仕訳モデル
- 96-bit係数、scale 0〜28の10進固定小数点
- 単一commodity仕訳の検証と正確な貸借集計
- 最終postingの金額省略と残額推論
- 決定的なTackler互換subset出力
- source、WARN、割引などの単一行コメント
- version付き正規化JSONからの仕訳候補decodeとstable identity生成
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
  └─ source-based identity and accounting fingerprint
```

正規化入力のfieldとidentity規約は[normalized input v1 contract](internal/ingest/CONTRACT.md)にまとめています。

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

## Tackler互換subset

exporterは日付、timezone付きRFC 3339日時、摘要、取引・postingコメント、勘定科目、固定小数点金額、明示commodity、最終postingの金額省略を扱います。timezoneなしの日時、transaction code、metadata、価格・原価、commodity換算などは対象外です。

対応構文、固定version、fixture構成の詳細は[互換性契約](internal/tacklerfmt/COMPATIBILITY.md)を参照してください。

## 現在の非対応範囲

- Tackler journal parser
- HTTP API、Web UI、認証、テナント管理
- DB schemaとmigration
- Gmail、Google Drive、Cloud Vision、Vertex AIとの連携
- 定期batchとjob管理
- Tree-sitter grammar、WebAssembly parser、LSP server
- 既存データの自動移行

今後の段階と未決定事項は[ロードマップ](ROADMAP.md)にまとめています。

## ライセンス

このプロジェクトは[Apache License 2.0](LICENSE)の下で公開されています。

# Repository guidance

## Scope

- このrepositoryは、Bokiccioの正確な仕訳domainとTackler互換subset exporterを育てるGo projectです。
- project概要は`README.md`、公開上の方向性は`ROADMAP.md`、format契約は`internal/tacklerfmt/COMPATIBILITY.md`を必要なときだけ参照してください。
- taskに関係しない機能、layer、依存関係を追加しないでください。

## Architecture boundaries

- `internal/ledger`はdomainです。Tackler、HTTP、DB、Google API、file systemへ依存させないでください。
- `internal/tacklerfmt`はTackler export adapterです。Tackler完全互換やjournal parserへ無断で拡張しないでください。
- domainでは`JournalEntry`を使い、`txn`はTackler fixtureまたはexport fileの文脈だけで使用してください。
- Tree-sitterのCST、editor、LSPの型を`ledger` modelへ結合しないでください。

## Accounting and data rules

- 金額に`float32`、`float64`、暗黙の丸めを使わないでください。
- `Decimal`の96-bit係数、scale 0〜28、非破壊演算という契約を維持してください。
- 複数commodityの換算を、換算ruleなしでbalanceさせないでください。
- fixtureには実在する取引、氏名、credential、private pathを含めず、架空・匿名化した値を使ってください。

## Change discipline

- userの未commit変更を保持し、taskに必要な最小差分にしてください。
- Go runtime dependencyは標準libraryを優先し、新規dependencyが必要なら理由とtrade-offを示してください。
- behaviorまたは対応formatを変える場合は、関連testと契約documentを同じ変更で更新してください。
- commit、push、remote変更はuserが明示的に依頼した場合だけ行ってください。

## Verification

- 変更したGo fileには`gofmt`を実行してください。
- codeまたはtest変更後は`go test ./...`、`go test -race ./...`、`go vet ./...`を実行してください。
- Tackler出力、grammar境界、compatibility fixtureを変えた場合は、追加で次を実行してください。

  ```sh
  go test -tags=tackler_integration ./internal/tacklerfmt
  ```

- documentだけの変更では、内部link、記載command、whitespaceを確認してください。
- 実行できない検証は成功扱いせず、理由を報告してください。

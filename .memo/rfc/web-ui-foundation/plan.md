# Plan: Web UI foundation

## Existing implementation

- `webapp.Handler`は`/api/v1`のJSON APIを提供し、`webapp.Repository`がimport、一覧、詳細を抽象化する。
- `webstore.Store`はnormalized import、filter-bound pagination、entry/run detailをTurso schema v2で提供する。
- `webprod.NewProductionHandler`は`/livez`以外をIAP owner検証し、全POSTへexplicit originを要求する。
- `cmd/bokiccio serve`はAPI handlerをproduction security handlerへ渡す。
- HTML component、asset、multipart upload、browser routeは存在しない。

## Package and composition direction

- `internal/webui`: UI route、typed view model、templ component、embedded asset、form decodeを担当する。
- `internal/webapp`: JSON APIと共有Repository/DTO契約を維持し、templ/htmxへ依存しない。
- `internal/webstore`: schema/queryを変更せず、既存Repository実装をUIでも再利用する。
- `internal/webprod`: IAP/security wrapperをAPIとUIを束ねたapplication handlerの外側へ一度だけ適用する。
- `internal/ledger`、`internal/ingest`、`internal/tacklerfmt`: UIへ依存しない。

composition rootで`webapp.Handler`と`webui.Handler`を明示muxへ登録する。`/api/v1/`はAPI、UI/assetsはwebuiへ
routingし、曖昧なfallbackでAPI error representationをHTMLへ変えない。

## Dependency and generation

templ runtimeとgeneratorを同一versionで固定する。generatorはGo `tool` directiveで管理し、`.templ`と生成された
`*_templ.go`をcommitする。通常の`go build`、Cloud Run source deploy、consumerの`go test`はgeneratorなしで動く。

generation checkは次を基準とする。

```console
go tool templ generate
git diff --exit-code -- '*_templ.go'
```

htmxは公式releaseのversionとchecksumを記録してrepository内assetとして保持し、`go:embed`でbinaryへ含める。
導入versionの更新は明示変更とし、CDN fallbackは持たない。

UI messageは全fieldを必須にするtyped Go structureを`ja`と`en`で構築し、templ componentへlocaleとmessageを
明示的に渡す。unprefixed routeは日本語、`/en` prefixは英語としてroute生成helperを共有する。locale stateを
cookie、query、browser storageへ保存しない。

Biome 2.5.6をexact development dependencyとして`package-lock.json`で固定する。`npm run format`はfirst-party
CSS/JavaScriptへwriteし、`npm run check`は同じ対象を変更せず検査する。vendor htmx、generated Go、`.templ`は
Biome includeから除外する。Node/npmはdevelopment verificationだけに使用し、Go build pathへ追加しない。

## Implementation slices

### Progress

- Slice 0: Implemented、2026-08-11にproduction verification完了
- Slice 1: Implemented
- Slice 2: Implemented
- Slice 3: Implemented、2026-08-12にlocal verificationとproduction smoke確認完了

### Slice 0: typed SSR shell and read-only browse

- templ/tool dependency、generation規約、embedded CSS/htmxを追加する。
- `/`、`/entries/{id}`、`/imports/{id}`のfull-page SSRを追加する。
- default一覧、entry/run detail、status/diagnostic/historyのtyped view modelを追加する。
- API/UI explicit muxとHTML security headerをproduction compositionへ接続する。
- HTML escape、404 representation、IAP保護、`/livez`、既存API回帰をtestする。

完了条件: production compositionと同じroute境界で`/`から一覧・詳細をkeyboard navigationでき、既存API bytesを
変更しない。

### Slice 1: i18n and frontend quality

- typed `ja`/`en` message resourceとtranslation completeness testを追加する。
- 既存unprefixed日本語routeを維持し、`/en` prefixの一覧、entry/run詳細、error routeを追加する。
- language selectorとlocale-aware internal link helperを追加し、accounting valueがlocale間で同一であることをtestする。
- exact-pinned Biome、lockfile、config、first-party CSS/JavaScript用format/check scriptを追加する。
- htmx checksum不変、templ再生成、Go test/race/vet/build、Biome checkを検証する。

完了条件: 日本語・英語のread-only UIをdirect navigationでき、translation欠落とfrontend format/lint違反が
reproducibleなcheckで失敗し、production buildへNode dependencyを追加しない。

### Slice 2: private search and htmx pagination

- `POST /ui/entries/search`のstrict form decodeと既存EntryFilterへのmappingを追加する。
- full-page responseと`HX-Request`時の一覧fragmentを同じtyped componentからrenderする。
- filter stateとcursorをform body/hidden fieldだけで引き継ぎ、URL・cookie・browser storageへ置かない。
- invalid filter/cursor、pagination、htmx/no-htmx、origin拒否、`Vary`/cache headerをtestする。

完了条件: 全filterとpaginationが既存APIと同じ結果を返し、private valueがnavigation URLへ現れない。

### Slice 3: normalized JSON file upload

- streaming multipart decode、単一file、10 MiB file limit、bounded request overheadを実装する。
- file bytesを既存`Repository.Import`へ渡し、run result/detailをHTML表示する。
- success、warning/error partial success、duplicate、invalid JSON/schema、oversize、複数file、storage failureをtestする。
- filename/contentをlog、URL、DB、safe errorへ出さないことをtestする。

完了条件: browser uploadからtransactional import、run detail、entry detailまで外部serviceなしで再現できる。

実装結果: `POST /ui/imports`と`POST /en/ui/imports`でmultipart uploadを受け、既存`Repository.Import`へfile bytesを渡す。
成功時はlocaleを維持してrun detailへ`303 See Other`で遷移する。invalid/oversize/media/storage failureはprivate-safeな
HTML errorを返し、security middleware由来のUI 401/403もHTML error pageへ振り分ける。API routeのsecurity errorは
JSON representationを維持する。

## Testing

- component: typed view model、escape、status/diagnostic、empty state、amount omission、date/timestamp
- handler: route/method/content type、strict form/multipart、body limit、full-page/fragment、private-safe error
- workflow: upload → run detail → entry detail、filter → pagination、duplicate、partial outcome
- security: IAP boundary、Origin、CSP/no-store/nosniff/referrer、no external asset、no private log
- compatibility: JSON API status/body/content type、export bytes、`/livez`
- generation: pinned `go tool templ generate`後にgenerated diffなし
- localization: `ja`/`en` route、`html[lang]`、key completeness、fallback/404、accounting value不変
- frontend quality: `npm run check`、format idempotence、vendor htmx checksum不変
- regression: `go test ./...`、`go test -race ./...`、`go vet ./...`

browser固有のfocus/history behaviorは、可能な範囲をHTML structure testし、Slice 2以降のmanual verificationで
keyboard-onlyとhtmx/no-htmxの両方を確認する。新しいbrowser automation dependencyは初期実装へ導入しない。

## Rollout and rollback

Slice 0はread-only UI routeとしてCloud Runへdeployし、既存APIを同じrevisionで回帰確認する。Slice 1はread-only
locale routeとdevelopment toolchainだけを追加する。Slice 2のsearch POSTとSlice 3のupload POSTは既存same-origin
middleware配下へ追加する。DB migrationとsecret追加は不要である。

rollbackはUI route/assets/dependencyをrevertしてAPI-only compositionへ戻す。UIは新しい永続dataを持たず、uploadで
commit済みのrunは既存API importと同じ正規dataなのでrollback後も保持する。

## Risks and mitigations

- generated code stale: generator version固定とregeneration diff check
- translation drift: typed message structureとlocale completeness test
- locale state leak: prefix route限定、cookie/query/browser storage不使用
- frontend tool drift: exact Biome versionとlockfile、first-party include限定
- htmx依存でno-JS操作不能: standard form/full-pageを先に成立させる
- private filterのURL/log露出: POST body限定、route-template logging、platform logの確認
- HTML injection: templ default escape、unsafe HTML禁止、CSP
- API/UI route collision: explicit muxとrepresentation回帰test
- multipart memory/DoS: streaming decode、file/total size limit、単一file strictness
- UIとAPIの意味ずれ: shared Repository/DTOと同一fixtureの結果比較

## Follow-up

revision編集・approval UIは本RFC完了後のUI Slice候補とする。Tackler `.txn` upload/importは別RFCでparser、subset、
partial failure、identity、Tree-sitterとの関係を決める。

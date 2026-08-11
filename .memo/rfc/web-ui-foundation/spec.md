# RFC: Web UI foundation

**RFC ID:** web-ui-foundation  
**Status:** Implemented

## Summary

既存のsingle-owner Web applicationへ、server-side renderingを基本とするキーボード中心のHTML UIを追加する。
初期Sliceではnormalized input v1 JSONのfile upload、仕訳候補の一覧・private filter検索・pagination、
entry/run詳細の閲覧、日本語・英語UI、first-party CSS/JavaScriptのlint・formatを提供する。revision編集とapproval UIは後続Sliceとする。

## Goals

- browserだけでnormalized inputを登録し、結果と生成仕訳を確認できる。
- APIと同じcurrent candidate、filter、pagination、diagnostic、履歴をHTMLでlosslessに表現する。
- private accounting valueをnavigation URL、application log、third-party requestへ複製しない。
- SSRを基本とし、htmxは応答性を改善するprogressive enhancementとして限定利用する。
- typed view modelとcompileされるHTML componentによってGo側の型境界を維持する。
- application-owned UI textを日本語と英語で提供し、accounting dataを翻訳・再解釈しない。
- first-party CSS/JavaScriptへ再現可能なformat・lint境界を持つ。

## Non-Goals

- revision編集、approval、export設定、backup/restoreのWeb UI
- provider固有CSV、画像、Tackler journalの直接uploadまたは変換
- account master、分類rule、AI suggestion、connector、定期job
- SPA、client-side router、offline/PWA、browser persistent storage
- Tree-sitter、browser editor、LSP
- multi-user、tenant、role、共同編集

Tackler互換subsetの`.txn` upload/importは、parserとpartial failure契約を扱う後続RFCとする。

## Rendering and dependency contract

HTML componentには`templ`を使用し、`.templ`から生成した`*_templ.go`もrepositoryへcommitする。
templ generatorはGo tool dependencyとしてversion固定し、再生成後に差分がないことをCIで検査する。
production runtime、Go/Cloud Run build、frontend asset生成にはNode runtimeとfrontend bundlerを導入しない。

CSSとversion固定したhtmxはGo binaryへembedし、同一originから配信する。CDN、外部font、analytics、
third-party scriptを使用しない。htmxが無効またはloadできない場合も、通常のHTML form submitとfull-page responseで
core workflowを完了できなければならない。`HX-Request`はfragment/full-page representationの選択だけに使用し、
認証、認可、validationの根拠にしない。

development-onlyのformat・lintにはBiome 2.5.6をexact npm dependencyとして使用し、`package.json`、
`package-lock.json`、configをcommitする。Biomeはfirst-party CSSとJavaScriptだけを対象とし、checksum固定のhtmx、
generated Go、templ sourceを変更しない。write commandとCI向けnon-mutating check commandを提供する。

## Localization

supported localeは日本語`ja`と英語`en`とし、日本語をdefaultにする。既存unprefixed UI routeは日本語として
後方互換に維持し、英語は`/en` prefixを使用する。language selectorは通常linkとし、localeのためのcookie、query、
local/session storageを追加しない。

application-owned title、navigation、label、status、empty/error messageをtyped Go message structureからrenderし、
localeごとのfield完全性と空文字をtestする。BCP 47 parsing/matchingが必要な境界だけ`golang.org/x/text/language`を使用する。
accounting value、account、source、diagnostic、Decimal text、commodity、opaque identityはlocale間で同一に保ち、
翻訳またはlocale依存の再formatを行わない。

## Browser routes

全routeは既存Cloud Run direct IAPとapplicationのowner検証下に置く。`/livez`だけを従来どおり例外とする。

- `GET /`: default条件の仕訳候補一覧、検索form、pagination、normalized input upload formを含むfull HTMLを返す。
- `POST /ui/entries/search`: filterとoptional cursorをform bodyで受ける。通常requestにはfull HTML、
  htmx requestには一覧とpaginationのHTML fragmentを返す。
- `GET /entries/{entry-id}`: source、import outcome、diagnostic、original、revision/approval履歴、current candidateを返す。
- `POST /ui/imports`: normalized input v1 JSON fileを1つ受け、既存import workflowで処理する。
- `GET /imports/{run-identity}`: status件数、record順のoutcome、diagnostic、生成entryへのlinkを返す。
- `GET /assets/{asset}`: binaryへembedしたversion固定assetだけを正しいcontent typeで返す。

上記UI routeには英語版の`/en/`、`/en/entries/{entry-id}`、`/en/imports/{run-identity}`を追加する。
Slice 2・3のUI POSTも`/en/ui/...`を持ち、responseとredirectでlocale prefixを維持する。unsupported locale prefixは
日本語へsilent fallbackせずprivate-safeなHTML 404を返す。既存unprefixed routeは日本語を返す。

UI routeのunknown resourceはprivate-safeなHTML 404を返す。既存`/api/v1`は従来どおりJSONを返し、route、schema、
status code、export bytesを変更しない。

## Search and pagination

UI検索は既存`EntryFilter`と同じfilter、AND条件、current candidate、orderingを使用する。filterとcursorは
`application/x-www-form-urlencoded`のrequest bodyで送信し、navigation URL、cookie、local storage、
session storageへ保存しない。response内のform controlとhidden fieldへHTML escapeして戻すことで、
full-page submitとhtmx paginationの両方で状態を引き継ぐ。

`/`のGETはfilterなしのdefault pageだけを返す。`/ui/entries/search`のPOSTはread-only operationだが、既存IAP
middlewareのstate-changing method規則に従い、設定済みexternal originと一致する`Origin`を必須とする。
filter validation、limit、opaque cursor、cursor/filter bindingは既存repository契約を再利用する。

## Upload and import

`POST /ui/imports`は`multipart/form-data`でUTF-8のnormalized input v1 JSONを1fileだけ受け付ける。
file contentは最大10 MiBとし、multipart全体にも小さく明示した追加上限を設けてstreamingで読む。
複数file、file field欠落、size超過、不正multipart、不正normalized inputをDB変更なしで拒否する。

upload filenameとclient指定content typeはidentity、source、format判定に使わず、DB、URL、log、errorへ保存しない。
file bytesを既存`Repository.Import`へそのまま渡し、identity、deduplication、warning/error、partial success、
transaction、Decimal、EntryTime、posting omissionの意味を変更しない。

commitされたrunはrecord-level errorを含んでも成功したuploadとして扱い、opaqueなrun identityを使う
`/imports/{run-identity}`へ遷移する。invalid input、size/media failure、storage failureはそれぞれ区別した
private-safeなHTML errorを返し、file content、filename、SQL、credentialを反映しない。

## HTML and accessibility contract

- selected localeと一致するdocument language、title、main landmark、見出し階層を持つ。
- 全form controlにprogrammatic labelを持たせ、error/diagnosticをcontrolまたは処理結果へ関連付ける。
- upload、検索、pagination、一覧から詳細への移動をkeyboardだけで実行できる。
- linkはnavigation、buttonはactionとして使い分け、色だけでstatusを表現しない。
- htmx fragment置換後にfocusを失わせず、処理結果を適切なlive regionで通知する。
- accounting textはtemplの通常escapeを通し、安全化されていないHTMLとして挿入しない。

## Security and privacy

既存IAP JWT、owner email、same-origin mutation、explicit external originの契約を維持する。application固有の
login、session、cookie、CSRF token storeを追加しない。

private HTML responseには`Cache-Control: no-store`を付ける。少なくともCSP `default-src 'self'`、
`object-src 'none'`、`base-uri 'none'`、`frame-ancestors 'none'`、`form-action 'self'`を設定し、inline executable
scriptを必要としない構成にする。`X-Content-Type-Options: nosniff`とprivate-safe referrer policyを設定する。

application logはmethod、route template、status、duration、request size、run identityなど既存の許可fieldだけを
扱い、query/form value、filename、source、description、diagnostic、account、amountを記録しない。

## Compatibility and data

DB schema v2、既存JSON API v1、local CLI、normalized input v1、report/state、Tackler exportを変更しない。
UI固有のserver-side session stateと永続tableは追加しない。UI handlerは既存repository interfaceを利用し、
会計domainとstorage rowへHTMLの概念を持ち込まない。

## Acceptance

`.memo/changes/web-ui-foundation/requirements.md`のR-001〜R-008、C-001〜C-007、AC-001〜AC-007を満たす。
full-pageとhtmx fragmentの両経路、private filter POST、uploadの成功・invalid・partial・duplicate・oversize、
日本語・英語とlocale route、translation completeness、Biome check、HTML escape、security header、API回帰を
外部serviceなしのtestで確認する。

## Implementation status

2026-08-12時点で、本RFCの初期Web UI foundation範囲は実装済みである。typed SSR shell、ja/en locale route、
frontend quality tooling、private search/htmx pagination、normalized JSON upload、Cloud Run direct IAP配下の
複数external origin許可、UI向けsecurity error HTML representationを含む。

revision編集、approval UI、Tackler journal upload、provider固有format uploadはNon-GoalsまたはFollow-upとして残す。

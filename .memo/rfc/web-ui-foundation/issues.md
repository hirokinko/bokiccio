# Issues: Web UI foundation

## ISSUE-001: SSR component and enhancement model

**Status:** Resolved  
**Blocking:** No

### Resolution

2026-08-11のユーザー回答により、server-side renderingを基本とし、部分更新が有用な箇所だけhtmxを使用する。
typed HTML componentにはtemplを採用し、生成された`*_templ.go`もcommitする。generatorはGo tool dependencyで
version固定し、CIで再生成差分を検査する。Node runtime/buildとSPAは導入しない。

## ISSUE-002: Initial UI boundary

**Status:** Resolved  
**Blocking:** No

### Resolution

2026-08-11のユーザー回答により、初期UIは一覧、検索、pagination、entry/run詳細、file uploadを含める。
revision編集とapproval UIは後続Sliceへ分離する。

## ISSUE-003: Browser entry route

**Status:** Resolved  
**Blocking:** No

### Resolution

`GET /`でdefault仕訳候補一覧を含むfull HTMLを表示する。entry/run detailはopaque IDを持つ直接linkで開ける。
client-side routerは持たない。

## ISSUE-004: Private filter transport

**Status:** Resolved  
**Blocking:** No

### Resolution

private filter valueをnavigation URLへ載せない。検索とpaginationはPOST form bodyで送り、通常requestにはfull page、
htmx requestにはfragmentを返す。stateはHTML form内だけで引き継ぎ、cookieやbrowser storageへ保存しない。
既存GET JSON APIは後方互換のため維持する。

## ISSUE-005: Upload format and limit

**Status:** Resolved  
**Blocking:** No

### Resolution

2026-08-11のユーザー回答により、UTF-8 normalized input v1 JSONを1file、file content最大10 MiBで受け付ける。
filenameとclient content typeをidentity/sourceへ使わず、DB、URL、log、errorへ保存しない。CSV、画像、Tackler
journal等のprovider/source変換は含めない。

## ISSUE-006: Generated templ code ownership

**Status:** Resolved  
**Blocking:** No

### Resolution

generated Goをcommitし、repositoryをtempl CLIなしでbuild可能に保つ。`.templ`を人が編集するsource of truthとし、
generated fileの直接編集は禁止する。CIはpinned generatorで再生成し、差分があれば失敗する。

## ISSUE-007: Tackler journal upload

**Status:** Deferred  
**Blocking:** No

### Resolution

ユーザー要望としてTackler `.txn` upload/importを記録する。本RFCへは含めず、parser方式、accepted subset、
unsupported syntax、file内partial failure、source/identity、Tree-sitter grammarとの関係を決める後続RFCとする。

## ISSUE-008: Supported UI locales

**Status:** Resolved
**Blocking:** No

### Resolution

2026-08-11のユーザー回答により、default localeを日本語`ja`、追加localeを英語`en`とする。application-owned UI textだけを翻訳し、accounting value、source、diagnostic、Decimal text、commodity、opaque identityは翻訳・再解釈しない。

## ISSUE-009: Locale route and persistence

**Status:** Resolved
**Blocking:** No

### Resolution

既存unprefixed routeを日本語として後方互換に維持し、英語は`/en/`、`/en/entries/{id}`、`/en/imports/{id}`で提供する。language selectorは通常linkとし、cookie、query、browser storageを追加しない。後続のsearch/upload POSTもlocale prefixを引き継ぐ。

## ISSUE-010: Frontend lint and format toolchain

**Status:** Resolved
**Blocking:** No

### Resolution

2026-08-11のユーザー回答により、development-onlyのNode/npmとBiome 2.5.6を導入する。Biomeはexact versionとlockfileで固定し、first-party CSS/JavaScriptだけをformat・lintする。Nodeをproduction runtime、Cloud Run build、frontend bundle、templ generationに使用せず、checksum固定のhtmxは対象外とする。

## ISSUE-011: Slice ordering

**Status:** Resolved
**Blocking:** No

### Resolution

i18nとfrontend qualityを次のSlice 1として実装し、既存のprivate search/paginationをSlice 2、normalized JSON uploadをSlice 3へ繰り下げる。

## ISSUE-012: Basic form POST and `Origin: null`

**Status:** Resolved
**Blocking:** No

### Resolution

2026-08-12のproduction確認で、`Referrer-Policy: no-referrer`によりbasic form POSTの`Origin`が`null`になり、
same-origin mutation検証で`POST /ui/imports`が403になることを確認した。private-safe referrer boundaryを保ちながら
same-origin form POSTを壊さないため、UI HTML responseの`Referrer-Policy`を`same-origin`へ変更する。

## ISSUE-013: Cloud Run service URL and generated deployment URL

**Status:** Resolved
**Blocking:** No

### Resolution

Cloud Runで提示される`https://bokiccio-391222912924.asia-northeast1.run.app`と、service status URLの
`https://bokiccio-r2eub5xl4q-an.a.run.app`の両方から利用されるため、`BOKICCIO_EXTERNAL_ORIGIN`はcomma-separated
HTTPS originsを受け付ける。state-changing UI/API requestの`Origin`はいずれかのconfigured originと完全一致する場合だけ許可する。

## ISSUE-014: UI security error representation

**Status:** Resolved
**Blocking:** No

### Resolution

IAP/Origin middlewareはUI handlerより前段で拒否するため、従来はUI form submitで403になった場合もJSONだけが表示された。
APIのJSON error契約を維持するため、security middlewareにerror writer hookを追加し、production compositionで`/api`はJSON、
UI routeは既存templ error pageへ振り分ける。

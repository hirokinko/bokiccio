# Web UI

BokiccioのWeb UIは、Cloud Run direct IAPで保護された単一利用者向けのserver-rendered HTMLです。JSON APIとはrouteを分離し、privateな検索条件や取引内容を外部asset URLへ渡しません。

## Routes

- `GET /`: 最新50件の仕訳候補
- `GET /entries/{id}`: 仕訳候補とrevision・承認履歴
- `GET /imports/{run-identity}`: 取込結果とdiagnostic
- `GET /en/`: 最新50件の仕訳候補（英語UI）
- `GET /en/entries/{id}`: 仕訳候補とrevision・承認履歴（英語UI）
- `GET /en/imports/{run-identity}`: 取込結果とdiagnostic（英語UI）
- `GET /assets/app.css`: 同梱した画面style
- `GET /assets/htmx-2.0.10.min.js`: 同梱したhtmx

`/en`は`/en/`へredirectします。`HEAD`も同じrouteで利用できます。その他のmethodは`405 Method Not Allowed`、存在しないresourceはHTMLの`404 Not Found`を返します。JSON APIは従来どおり`/api/`以下で提供します。

unprefixed routeは日本語UI、`/en/` prefixは英語UIです。localeはURL pathだけで決まり、cookie、query parameter、local storage、session storageは使いません。account、source、diagnostic code/message、Decimal text、commodity、opaque IDはlocale間で翻訳・再formatしません。

すべてのHTMLとassetはproduction handlerのIAP検証を通り、`Cache-Control: no-store`と同一originを前提としたContent Security Policyを付与します。画面のHTMLはtemplから生成し、生成済みの`*_templ.go`もsource treeへcommitします。

## Development commands

templ sourceを変更した場合は生成済みGoを更新します。

```sh
go tool templ generate -path internal/webui
```

first-party CSS/JavaScriptはdevelopment-onlyのBiomeで管理します。checksum固定の`internal/webui/assets/htmx-2.0.10.min.js`、`.templ`、generated GoはBiome対象外です。

```sh
npm ci
npm run format
npm run check
```

この段階の画面はread-onlyです。検索、normalized JSON upload、仕訳の修正・承認は後続のsliceで追加します。Tackler journal formatのfile uploadは別RFCで扱います。

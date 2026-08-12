# Web UI

BokiccioのWeb UIは、Cloud Run direct IAPで保護された単一利用者向けのserver-rendered HTMLです。JSON APIとはrouteを分離し、privateな検索条件や取引内容を外部asset URLへ渡しません。

## Routes

- `GET /`: 最新50件の仕訳候補
- `POST /ui/imports`: normalized input v1 JSON fileのupload
- `POST /ui/imports/tackler`: Tackler互換subset `.txn` fileのupload
- `POST /ui/entries/search`: form bodyによる仕訳候補検索とpagination
- `POST /ui/exports/tackler`: form body filterによる承認済み仕訳のTackler export
- `POST /ui/exports/json`: form body filterによる承認済み仕訳のJSON export
- `POST /ui/entries/{id}/revisions`: 仕訳候補のrevision作成
- `POST /ui/entries/{id}/approvals`: latest revisionの承認
- `GET /entries/{id}`: 仕訳候補とrevision・承認履歴
- `GET /imports/{run-identity}`: 取込結果とdiagnostic
- `GET /en/`: 最新50件の仕訳候補（英語UI）
- `POST /en/ui/imports`: normalized input v1 JSON fileのupload（英語UI）
- `POST /en/ui/imports/tackler`: Tackler互換subset `.txn` fileのupload（英語UI）
- `POST /en/ui/entries/search`: form bodyによる仕訳候補検索とpagination（英語UI）
- `POST /en/ui/exports/tackler`: form body filterによる承認済み仕訳のTackler export（英語UI）
- `POST /en/ui/exports/json`: form body filterによる承認済み仕訳のJSON export（英語UI）
- `POST /en/ui/entries/{id}/revisions`: 仕訳候補のrevision作成（英語UI）
- `POST /en/ui/entries/{id}/approvals`: latest revisionの承認（英語UI）
- `GET /en/entries/{id}`: 仕訳候補とrevision・承認履歴（英語UI）
- `GET /en/imports/{run-identity}`: 取込結果とdiagnostic（英語UI）
- `GET /assets/app.css`: 同梱した画面style
- `GET /assets/htmx-2.0.10.min.js`: 同梱したhtmx

`/en`は`/en/`へredirectします。`HEAD`も同じrouteで利用できます。その他のmethodは`405 Method Not Allowed`、存在しないresourceはHTMLの`404 Not Found`を返します。JSON APIは従来どおり`/api/`以下で提供します。

unprefixed routeは日本語UI、`/en/` prefixは英語UIです。localeはURL pathだけで決まり、cookie、query parameter、local storage、session storageは使いません。account、source、diagnostic code/message、Decimal text、commodity、opaque IDはlocale間で翻訳・再formatしません。

検索条件とpagination cursorは`application/x-www-form-urlencoded`のPOST bodyで送信し、URL、cookie、browser storageへ保存しません。`HX-Request: true`の場合は一覧、export form、paginationだけのHTML fragmentを返し、通常requestにはfull HTMLを返します。

export routeも検索と同じfilterを`application/x-www-form-urlencoded`のPOST bodyで受け付ける。Tackler exportと
JSON exportは既存JSON APIと同じ対象選択、出力bytesまたはschema、content typeを使う。filter付きexportでも
検索条件をURLへ出さない。

entry detailのrevision formはTackler風の1 entryをtextareaで送信する。先頭行は`date  'description`、
以降は4 spaces indentのtransaction comment行またはposting行とする。postingは`account amount commodity ; comment`
形式を受け付け、amountとcommodityを省略した行はomitted amountとして扱う。空行は無視する。textarea内のTabは
first-party JavaScriptで4 spaces入力にする。postingの追加は行追加、削除は行削除で行える。invalid revisionは履歴として保存し、approval routeではvalidation済みのlatest revisionだけを承認する。

normalized input uploadは`multipart/form-data`のPOST bodyで送信します。file fieldは`file`、file contentは最大10 MiB、request全体にも小さなoverhead上限を設けます。filenameとclient側Content-Typeはsource、identity、format判定には使わず、保存、log、HTML responseへの反映もしません。record単位のerrorを含んだ取込runも保存できた場合は成功uploadとして扱い、`303 See Other`で`/imports/{run-identity}`または`/en/imports/{run-identity}`へredirectします。

Tackler `.txn` uploadはnormalized input uploadとは別form/routeで扱う。対応subsetは`internal/tacklerfmt/COMPATIBILITY.md`に従い、parse後のentryをnormalized input v1へ変換して既存import経路へ渡す。sourceはprivate filenameではなく`tackler: uploaded.txn`として記録する。
parseやdomain validationに失敗した場合は、line numberまたはentry numberと原因をHTML errorとserver logへ出す。原因にはparser/domain validationが返すoffending valueを含む場合がある。filename、SQL error、request body全体は表示しない。

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

この段階の画面はnormalized JSON upload、Tackler `.txn` upload、検索、閲覧、revision作成、approval、承認済み仕訳のexportに対応しています。

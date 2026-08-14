# Web UI

BokiccioのWeb UIは、Cloud Run direct IAPで保護された単一利用者向けのserver-rendered HTMLです。JSON APIとはrouteを分離し、privateな検索条件や取引内容を外部asset URLへ渡しません。

## Routes

- `GET /`: 最新50件の仕訳候補
- `POST /ui/imports`: normalized input v1/v2 JSON fileのupload
- `POST /ui/imports/tackler`: Tackler互換subset `.txn` fileのupload
- `POST /ui/entries/search`: form bodyによる仕訳候補検索とpagination
- `POST /ui/exports/tackler`: form body filterによる承認済み仕訳のTackler export
- `POST /ui/exports/json`: form body filterによる承認済み仕訳のJSON export
- `POST /ui/entries/{id}/revisions`: 仕訳候補のrevision作成
- `POST /ui/entries/{id}/approvals`: latest revisionの承認
- `GET /entries/{id}`: 仕訳候補とrevision・承認履歴
- `GET /imports/{run-identity}`: 取込結果とdiagnostic
- `GET /settings/reporting`: reporting calendar、classification、会計年度、期首残高方式の設定
- `POST /ui/settings/reporting`: reporting configuration revisionの作成
- `GET /reports/trial-balance`: 選択した会計年度または月次期間のcommodity別試算表
- `POST /ui/reports/trial-balance`: form bodyで選択した期間へのredirect
- `GET /reports/current`: 参照日時点の現在残高と独立選択した月の費用
- `POST /ui/reports/current`: 選択した参照日または費用月のhtmx fragment、または非htmx redirect
- `GET /reports/balance-sheet`: 選択した会計年度の期首貸借対照表
- `POST /ui/reports/balance-sheet`: form bodyで選択した会計年度へのredirect
- `GET /reports/income-statement`: 選択した月次期間の損益計算書
- `POST /ui/reports/income-statement`: form bodyで選択した月次期間へのredirect
- `GET /reports/balance-trend`: 選択した会計年度の12か月の全勘定残高推移
- `POST /ui/reports/balance-trend`: form bodyで選択した会計年度へのredirect
- `GET /en/`: 最新50件の仕訳候補（英語UI）
- `POST /en/ui/imports`: normalized input v1/v2 JSON fileのupload（英語UI）
- `POST /en/ui/imports/tackler`: Tackler互換subset `.txn` fileのupload（英語UI）
- `POST /en/ui/entries/search`: form bodyによる仕訳候補検索とpagination（英語UI）
- `POST /en/ui/exports/tackler`: form body filterによる承認済み仕訳のTackler export（英語UI）
- `POST /en/ui/exports/json`: form body filterによる承認済み仕訳のJSON export（英語UI）
- `POST /en/ui/entries/{id}/revisions`: 仕訳候補のrevision作成（英語UI）
- `POST /en/ui/entries/{id}/approvals`: latest revisionの承認（英語UI）
- `GET /en/entries/{id}`: 仕訳候補とrevision・承認履歴（英語UI）
- `GET /en/imports/{run-identity}`: 取込結果とdiagnostic（英語UI）
- `GET /en/settings/reporting`: reporting設定（英語UI）
- `POST /en/ui/settings/reporting`: reporting configuration revisionの作成（英語UI）
- `GET /en/reports/trial-balance`: commodity別試算表（英語UI）
- `POST /en/ui/reports/trial-balance`: form bodyで選択した期間へのredirect（英語UI）
- `GET /en/reports/current`: 現在残高と選択月費用（英語UI）
- `POST /en/ui/reports/current`: 選択した参照日または費用月のhtmx fragment、または非htmx redirect（英語UI）
- `GET /en/reports/balance-sheet`: 期首貸借対照表（英語UI）
- `POST /en/ui/reports/balance-sheet`: form bodyで選択した会計年度へのredirect（英語UI）
- `GET /en/reports/income-statement`: 月次損益計算書（英語UI）
- `POST /en/ui/reports/income-statement`: form bodyで選択した月次期間へのredirect（英語UI）
- `GET /en/reports/balance-trend`: 12か月の全勘定残高推移（英語UI）
- `POST /en/ui/reports/balance-trend`: form bodyで選択した会計年度へのredirect（英語UI）
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
または`account quantity commodity = total commodity ; comment`形式を受け付け、amountとcommodityを省略した行は
omitted amountとして扱う。空行は無視する。textarea内のTabは
first-party JavaScriptで4 spaces入力にする。postingの追加は行追加、削除は行削除で行える。invalid revisionは履歴として保存し、approval routeではvalidation済みのlatest revisionだけを承認する。

reporting設定formは`base_revision`を含む全設定を送信し、保存時にimmutableな新revisionを作る。分類は親accountから
descendantへ継承し、会計年度は開始日・終了日、期首残高方式、改行区切りの期首仕訳IDを保持する。calendar変更時は
過去reportも再集計される旨を画面に表示する。stale formは`409 Conflict`、不正な期間・分類・期首仕訳は`400 Bad Request`とし、
既存revisionを変更しない。400 responseは開始月、年度境界、分類の重複、期首仕訳の承認・日付・分類など、利用者が
修正できる原因をlocale別の安全なmessageで表示する。初回表示だけ入力開始用の空行を1件表示し、設定済み画面では保存済みの
分類・会計年度だけを表示する。行の追加・削除はfirst-party JavaScriptの明示buttonで行い、画面を開くだけでは行を増やさない。

試算表画面はconfigured fiscal yearと各月次期間だけを選択肢にし、queryには選択済みの`start_date`と`end_date`だけを持つ。
初期表示は設定内の最後の会計年度全体を決定的に選び、current dateへ依存しない。commodityを別sectionに分け、category、
account階層、小計、期首・発生・期末の借方・貸方をcanonical decimal stringのまま表示する。通常の科目行は配下を含む小計を
表示し、直接計上値が小計と異なる場合だけ補助行へ表示する。未分類accountも金額へ含め、科目欄にWARNINGと設定画面への導線を
表示する。狭い画面では横長tableを科目別cardへ切り替え、6つの金額を2列で表示する。小計と異なる直接計上値は折りたたみ内へ
配置し、横スクロールなしでreport全体を確認できるようにする。

reportの主要導線は現在残高・月間費用画面とし、queryなしでは日本標準時の当日を残高基準日、その日を含む設定済み月次期間を
費用月にする。日付selectorは設定済み会計年度内の任意日を選択でき、残高には参照日より後の承認済み仕訳を含めない。
現在残高には資産・負債・純資産の実残高を表示し、
収益・費用の仮想振替やcategory間の貸借一致を表示条件にしない。このため決算書のB/Sとは明確に区別する。

費用月は残高基準日と独立して設定済み月次期間から選択し、その開始日から終了日までの費用category合計とaccount内訳だけを
表示する。残高基準日のselectorは現在残高section、費用月のselectorは月間費用sectionに配置し、一方を変更しても他方の選択は維持する。
「支払予定」など資産に分類したaccountは現在残高へ残り、
費用への振替は仕訳日を含む選択月の費用集計へ反映される。commodityは混ぜず、
資産・負債・純資産のsummary cardと費用合計をdesktop・smartphoneの両方で確認できる。試算表は全勘定の検証用として維持する。

htmx requestでは、残高基準日の変更は`#current-balances-result`、費用月の変更は
`#current-expenses-result`だけを差し替える。responseの`HX-Push-Url`でcanonical query付きURLを履歴へ反映し、
out-of-band swapでもう一方のformのhidden値を同期する。400と500は対応するresult内へ表示し、500の内部詳細は
開発環境だけに含める。htmxを利用しないPOSTは従来どおり303 redirectし、GETで同じ画面を表示する。

期首貸借対照表は設定済み会計年度と完全一致する期間だけを受け付け、その年度の期首残高方式から資産・負債・純資産を
requestごとに再構成する。月次損益計算書はreporting calendarが生成した単月だけを対象に、収益・費用と月次損益を表示する。
いずれもcommodityを分離し、未分類accountを別groupに残す。金額は1列とし、資産・費用は借方、負債・純資産・収益は貸方を
正として表示する。反対残高は負数、WARNING、実際の借方・貸方を併記する。

残高推移は設定済み会計年度の12月末について、資産・負債・純資産・収益・費用・未分類の全勘定残高を表示する。収益・費用は
年度期首から各月末までの累計であり、月次P/Lとは区別する。desktop・smartphoneとも月単位のcardを縦またはgridに並べ、
横長の12列tableを必須にしない。自動繰越の期首が貸借不一致なら期首B/Sと残高推移は`422`で理由と設定導線を表示し、月次P/Lは
引き続き利用できる。

normalized input uploadは`multipart/form-data`のPOST bodyで送信します。file fieldは`file`、file contentは最大10 MiB、request全体にも小さなoverhead上限を設けます。filenameとclient側Content-Typeはsource、identity、format判定には使わず、保存、log、HTML responseへの反映もしません。record単位のerrorを含んだ取込runも保存できた場合は成功uploadとして扱い、`303 See Other`で`/imports/{run-identity}`または`/en/imports/{run-identity}`へredirectします。

Tackler `.txn` uploadはnormalized input uploadとは別form/routeで扱う。対応subsetは`internal/tacklerfmt/COMPATIBILITY.md`に従い、parse後のentryをnormalized input v2へ変換して既存import経路へ渡す。sourceはprivate filenameではなく`tackler: uploaded.txn`として記録する。
parseやdomain validationに失敗した場合は、line numberまたはentry numberと原因をHTML errorとserver logへ出す。原因にはparser/domain validationが返すoffending valueを含む場合がある。filename、SQL error、request body全体は表示しない。

すべてのHTMLとassetはproduction handlerのIAP検証を通り、`Cache-Control: no-store`と同一originを前提としたContent Security Policyを付与します。画面のHTMLはtemplから生成し、生成済みの`*_templ.go`もsource treeへcommitします。

500 responseは既定で内部errorを表示しない。明示的に`BOKICCIO_ENVIRONMENT=development`を設定した環境だけ、原因となった
error textをHTML escapeしてdevelopment detailとして表示する。この値はprivateなaccount、SQL、pathを含み得るため、owner以外が
到達できる環境やproductionでは設定しない。未設定または`production`では常にprivate-safeな固定messageだけを返す。

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

この段階の画面はnormalized JSON upload、Tackler `.txn` upload、検索、閲覧、revision作成、approval、承認済み仕訳のexport、
reporting設定、現在残高・月間費用、commodity別試算表、期首B/S、月次P/L、全勘定残高推移に対応しています。

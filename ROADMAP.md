# Roadmap

Bokiccioは、さまざまな取引記録を検証可能な仕訳へ変換し、利用者が自分の会計データを管理・確認・持ち出せるサービスを目指しています。

このロードマップは方向性を共有するためのもので、提供時期を確約するものではありません。各段階では、正確性、データの可搬性、入力元への追跡可能性を優先します。

## Available: 会計データの基盤とローカルworkflow

現在は、後続機能が共通利用する仕訳・exportの基盤と、外部serviceなしで再現できるローカル取込workflowを提供しています。

- 正確な固定小数点金額とcommodityを持つ仕訳model
- 貸借、勘定科目、コメント、金額省略のvalidation
- posting数量と`=`総額指定を分けて保持する複数commodity換算
- 入力順とsource・WARNなどのmetadataを保つ決定的なexport
- Tackler journal format互換subsetへの出力
- Tackler 26.1.2以降に対する自動互換性test
- version付きの正規化JSON import境界
- source情報を維持した仕訳候補の生成
- 重複取込の検出と安全な再実行
- record単位のvalidation error・WARNとmachine-readable report
- immutable run bundleとdeduplication state
- `bokiccio import`による検証・export
- 匿名化fixtureを使ったend-to-end test

## Available: Single-user web application

単一利用者が、取込結果と仕訳候補をprivateなWeb/API境界で確認し、修正・承認・export・backupを行える基盤を提供しています。

normalized import、Turso Cloud永続化、取込結果・仕訳候補のJSON API、Cloud Run direct IAPによるsingle-owner公開境界、未登録IAP user向けのread-only閲覧、日本語・英語でnormalized JSON uploadとTackler `.txn` upload、仕訳検索、取込履歴閲覧、修正・承認・exportを行うserver-rendered画面、immutableな修正・承認履歴、最新revisionの検索、承認済み仕訳のexport、checksum付きlogical backupと空databaseへのtransactional restoreまでを実装・検証済みです。

- journalとpostingの永続化
- 取込履歴、処理状態、sourceの表示
- Web UIからのnormalized JSON upload
- Web UIからのTackler互換subset `.txn` upload
- Tackler `.txn`の`=` total-price value positionの取込・編集・再出力
- 日本語・英語のserver-rendered閲覧画面
- signed IAP JWTとowner identityを検証するproduction server
- database allowlistでdata変更をsingle ownerへ限定するread-only viewer境界
- APIによる仕訳候補の修正・承認
- Web UIによる仕訳候補の修正・承認
- 最新revisionを対象にした検索、絞り込み、期間別の閲覧
- 承認済み仕訳のTackler互換形式と機械可読形式でのAPI/UI export
- checksum付きlogical backupと空database限定のtransactional restore

## Available: 会計レポート基盤

承認済みの最新仕訳を、利用者が明示した分類と会計年度に基づいて日常的に検証する基盤を提供しています。

- 資産・負債・純資産・収益・費用への明示的なreporting classification
- 開始月、期首・期末を持つ会計年度と、年度内の月次期間
- 年度別の自動繰越または承認済み期首仕訳による期首残高方式
- 月次・年度を選択でき、勘定科目階層を保ったcommodity別試算表
- 参照日時点の資産・負債・純資産と、独立選択した月の費用合計・内訳を別sectionで確認する日常用overview
- 残高基準日と費用月を対応section内で選択し、URLと選択状態を保ったhtmx部分更新と非htmx fallback
- 年度ごとの期首残高方式から再構成する期首貸借対照表（B/S）
- 会計年度末の実残高と当期損益の表示補正から参照時に再集計する期末貸借対照表（B/S）
- reporting calendarの単月と設定済み会計年度全体を対象にした損益計算書（P/L）
- 5区分と未分類を含む12か月の全勘定残高推移
- 試算表と単月・通期P/Lのaccount集計値から承認済み仕訳・寄与postingへ戻るdrill-down
- 未分類accountと通常とは反対側の残高に対するWARNING
- 日本語・英語とdesktop・smartphoneに対応したreport画面
- reporting設定履歴を含むlogical backup/restore

## Planned: 会計レポートの拡張

承認済みの最新仕訳から、期間や基準日を指定して財務状態と損益を確認できるようにします。取込量を増やす前に、登録した仕訳が期待どおり集計されていることを日常的に検証できる導線を整えます。

- 月末または任意基準日時点の貸借対照表（B/S）
- 会計年度途中までの年度累計損益計算書（P/L）
- 前月・前年同月・前年度との比較
- category・commodity合計、月次損益、その他reportから承認済み仕訳へのdrill-down拡張
- 比較期間と小計・合計の機械可読export
- 現金・現金同等物とcash-flow categoryを定義した後のキャッシュフロー計算書（C/F）

会計年度・月次期間、明示的なreporting classification、commodity別試算表に加え、参照日時点の現在残高・選択月費用overview、年度別の期首B/S・期末B/S、単月・通期P/L、月末全勘定残高推移まで提供済みである。試算表と単月・通期P/Lのaccount行は、同じ集計規則で承認済み仕訳と寄与postingまで追跡できる。期末B/Sは現在の承認済み仕訳から参照時に再集計し、収益・費用の差額を当期損益として純資産側へ表示補正するが、決算仕訳や勘定科目、締め済みsnapshotは作らない。overviewの残高と費用は画面遷移なしに個別更新でき、非htmx環境でも同じ集計結果を表示する。複数commodityは引き続き換算ruleなしに合算せず、表示commodityごとに分けるか、明示的な換算結果だけを使用する。

資産推移は承認済み仕訳から算出した帳簿残高を月末ごとに表示し、資産合計だけでなく負債、純資産、勘定科目階層へ分解できるようにする。市場価格による時価評価、含み損益、価格情報の自動取得はこの集計へ暗黙に混ぜず、価格sourceと評価ruleを定義する後続機能として扱う。

年度・月次はreporting calendarとして定義し、entryの日付を対応する期間へ決定的に割り当てる。現在のreportは参照時に再集計し、月次締め・年度締めによる過去仕訳の編集制限や決算振替の自動生成は別途仕様を決める。

C/FはB/S・P/Lと同じ仕訳を基礎にするが、現金・現金同等物の指定、営業・投資・財務への分類、直接法・間接法の選択が追加で必要になる。そのため会計レポートphaseの後続sliceとし、B/S・P/Lの提供を待たせない。

## Planned: Automated ingestion

利用者が許可した外部sourceから取引記録を継続的に取り込み、手作業を減らします。

- メール、クラウドストレージ、ファイルuploadのconnector
- スマートフォンまたはPCのbrowser cameraで撮影したレシート・請求書からの取引登録
- 画像・文書からのOCRと構造化抽出
- AIによる摘要・勘定科目候補の提案
- 定期実行、retry、rate limit、部分成功の管理
- 原本、抽出結果、生成仕訳のprovenance
- confidenceとWARNに基づくhuman review

外部連携は明示的な許可、最小権限、認証情報の安全な保管を前提とします。AIの推論結果は確定仕訳として無条件に採用せず、検証可能な候補として扱います。

## Future: Editor and interoperability

plain text accountingの可搬性を維持しながら、Web editorと外部toolとの連携を強化します。Tackler形式のimportや編集補助に効く領域ですが、当面は取引登録と日常操作の価値を優先します。

- Tackler互換subsetのTree-sitter grammar
- browser向けincremental parsingとsyntax highlighting
- LSPによる勘定科目補完、commodity検証、貸借診断
- formatter、parser、LSPで共有するcompatibility corpus
- import・export用のversioned API
- formatやstorageに依存しないdata portability

Tree-sitterは構文解析、LSPは意味検証を担当し、会計データmodelとeditor固有の表現は分離します。

## Future: Multi-user service

単一利用者向けworkflowが安定した後、複数利用者が安全に使えるservice機能を検討します。

- tenantごとのデータ分離
- 認証、role、権限管理
- 共同確認と変更履歴
- 監査logとデータexport
- 利用量とquotaの管理
- backup、障害復旧、observability
- securityとprivacyに対する継続的な検証

## Product principles

- **Correctness:** 金額を浮動小数点で扱わず、検証失敗を黙って補正しない。
- **Traceability:** 生成された仕訳から入力sourceと処理結果を確認できるようにする。
- **Human control:** 自動分類やAI提案を利用者が確認・修正できるようにする。
- **Portability:** 特定の画面、DB、providerに閉じず、データをexportできるようにする。
- **Privacy:** 外部連携とAI処理の範囲を明確にし、最小限のデータと権限だけを使う。
- **Interoperability:** Tackler互換性と構文fixtureを維持し、外部toolとの接続を検証する。

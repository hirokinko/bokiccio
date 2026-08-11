# Roadmap

Bokiccioは、さまざまな取引記録を検証可能な仕訳へ変換し、利用者が自分の会計データを管理・確認・持ち出せるサービスを目指しています。

このロードマップは方向性を共有するためのもので、提供時期を確約するものではありません。各段階では、正確性、データの可搬性、入力元への追跡可能性を優先します。

## Available: 会計データの基盤とローカルworkflow

現在は、後続機能が共通利用する仕訳・exportの基盤と、外部serviceなしで再現できるローカル取込workflowを提供しています。

- 正確な固定小数点金額とcommodityを持つ仕訳model
- 貸借、勘定科目、コメント、金額省略のvalidation
- 入力順とsource・WARNなどのmetadataを保つ決定的なexport
- Tackler journal format互換subsetへの出力
- Tackler 26.1.2に対する自動互換性test
- version付きの正規化JSON import境界
- source情報を維持した仕訳候補の生成
- 重複取込の検出と安全な再実行
- record単位のvalidation error・WARNとmachine-readable report
- immutable run bundleとdeduplication state
- `bokiccio import`による検証・export
- 匿名化fixtureを使ったend-to-end test

## In progress: Single-user web application

仕訳の取込から確認、修正、exportまでをWeb上で扱える、単一利用者向けapplicationへ発展させます。

現在はnormalized import、Turso Cloud永続化、取込結果・仕訳候補のJSON API、Cloud Run direct IAPによるsingle-owner公開境界、日本語・英語で仕訳検索と取込履歴閲覧を行うserver-rendered画面、immutableな修正・承認履歴、最新revisionの検索、承認済み仕訳のexport、checksum付きlogical backupと空databaseへのtransactional restoreまでを実装しています。

- journalとpostingの永続化
- 取込履歴、処理状態、sourceの表示
- 日本語・英語のserver-rendered閲覧画面
- signed IAP JWTとowner identityを検証するproduction server
- 仕訳候補の確認・修正・承認
- 勘定科目と分類ruleの管理
- 検索、絞り込み、期間別の閲覧
- Tackler互換形式と機械可読形式でのexport
- backupとrestore

## Planned: Automated ingestion

利用者が許可した外部sourceから取引記録を継続的に取り込み、手作業を減らします。

- メール、クラウドストレージ、ファイルuploadのconnector
- 画像・文書からのOCRと構造化抽出
- AIによる摘要・勘定科目候補の提案
- 定期実行、retry、rate limit、部分成功の管理
- 原本、抽出結果、生成仕訳のprovenance
- confidenceとWARNに基づくhuman review

外部連携は明示的な許可、最小権限、認証情報の安全な保管を前提とします。AIの推論結果は確定仕訳として無条件に採用せず、検証可能な候補として扱います。

## Future: Multi-user service

単一利用者向けworkflowが安定した後、複数利用者が安全に使えるservice機能を検討します。

- tenantごとのデータ分離
- 認証、role、権限管理
- 共同確認と変更履歴
- 監査logとデータexport
- 利用量とquotaの管理
- backup、障害復旧、observability
- securityとprivacyに対する継続的な検証

## Future: Editor and interoperability

plain text accountingの可搬性を維持しながら、Web editorと外部toolとの連携を強化します。

- Tackler互換subsetのTree-sitter grammar
- browser向けincremental parsingとsyntax highlighting
- LSPによる勘定科目補完、commodity検証、貸借診断
- formatter、parser、LSPで共有するcompatibility corpus
- import・export用のversioned API
- formatやstorageに依存しないdata portability

Tree-sitterは構文解析、LSPは意味検証を担当し、会計データmodelとeditor固有の表現は分離します。

## Product principles

- **Correctness:** 金額を浮動小数点で扱わず、検証失敗を黙って補正しない。
- **Traceability:** 生成された仕訳から入力sourceと処理結果を確認できるようにする。
- **Human control:** 自動分類やAI提案を利用者が確認・修正できるようにする。
- **Portability:** 特定の画面、DB、providerに閉じず、データをexportできるようにする。
- **Privacy:** 外部連携とAI処理の範囲を明確にし、最小限のデータと権限だけを使う。
- **Interoperability:** Tackler互換性と構文fixtureを維持し、外部toolとの接続を検証する。

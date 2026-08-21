package webui

import "fmt"

type messages struct {
	LocaleName                                 string
	SiteTagline                                string
	LanguageLabel                              string
	JapaneseLanguage                           string
	EnglishLanguage                            string
	IndexTitle                                 string
	IndexEyebrow                               string
	EmptyEntriesTitle                          string
	EmptyEntriesMessage                        string
	UploadFormLabel                            string
	UploadHeading                              string
	UploadFileLabel                            string
	UploadHelp                                 string
	UploadSubmit                               string
	TacklerUploadFormLabel                     string
	TacklerUploadHeading                       string
	TacklerUploadFileLabel                     string
	TacklerUploadHelp                          string
	TacklerUploadSubmit                        string
	NoMatchingEntriesTitle                     string
	NoMatchingEntriesMessage                   string
	SearchFormLabel                            string
	SearchHeading                              string
	DateFromLabel                              string
	DateToLabel                                string
	AccountFilterLabel                         string
	DescriptionFilterLabel                     string
	ImportStatusFilterLabel                    string
	WorkflowFilterLabel                        string
	SourceNamespaceLabel                       string
	SourceDisplayLabel                         string
	SearchSubmit                               string
	ClearSearch                                string
	AllImportStatuses                          string
	ImportStatusSuccess                        string
	ImportStatusWarning                        string
	AllWorkflowStatuses                        string
	WorkflowStatusUnapproved                   string
	WorkflowStatusInvalid                      string
	WorkflowStatusApproved                     string
	SearchResultsLabel                         string
	ExportHeading                              string
	ExportTackler                              string
	ExportJSON                                 string
	NextPage                                   string
	InvalidUploadTitle                         string
	InvalidUploadMessage                       string
	InvalidTacklerUploadTitle                  string
	InvalidTacklerUploadMessage                string
	UploadTooLargeTitle                        string
	UploadTooLargeMessage                      string
	UnsupportedUploadTitle                     string
	UnsupportedUploadMessage                   string
	UploadFailedTitle                          string
	UploadFailedMessage                        string
	UploadDisabledTitle                        string
	UploadDisabledMessage                      string
	UploadForbiddenTitle                       string
	UploadForbiddenMessage                     string
	WriteForbiddenTitle                        string
	WriteForbiddenMessage                      string
	InvalidSearchTitle                         string
	InvalidSearchMessage                       string
	ImportStatusLabel                          string
	WorkflowStatusLabel                        string
	RevisionLabel                              string
	BackToEntries                              string
	SourceHeading                              string
	RunLabel                                   string
	RecordLabel                                string
	CurrentCandidateHeading                    string
	EditCandidateHeading                       string
	EditCandidateFormLabel                     string
	EntryTextLabel                             string
	EntryTextHelp                              string
	SaveRevision                               string
	ApproveCandidateHeading                    string
	ApproveCandidateFormLabel                  string
	ApproveRevision                            string
	AlreadyApproved                            string
	CannotApproveInvalid                       string
	FormErrorTitle                             string
	InvalidRevisionFormMessage                 string
	RevisionConflictMessage                    string
	RevisionFailedMessage                      string
	InvalidApprovalFormMessage                 string
	ApprovalInvalidRevisionMessage             string
	ApprovalConflictMessage                    string
	ApprovalFailedMessage                      string
	OriginalSnapshotHeading                    string
	ImportDiagnostics                          string
	None                                       string
	HistoryHeading                             string
	CurrentRevisionLabel                       string
	CurrentApprovalLabel                       string
	Unapproved                                 string
	ApprovalHistory                            string
	RunTitle                                   string
	RunEyebrow                                 string
	NeedsReview                                string
	Complete                                   string
	InputDigestLabel                           string
	PreStateGeneration                         string
	OutcomesHeading                            string
	IdentityLabel                              string
	ViewEntry                                  string
	AccountColumn                              string
	AmountColumn                               string
	CommentColumn                              string
	OmittedAmount                              string
	DiagnosticsLabel                           string
	MethodNotAllowedTitle                      string
	MethodNotAllowedMessage                    string
	NotFoundTitle                              string
	NotFoundMessage                            string
	InternalErrorTitle                         string
	InternalErrorMessage                       string
	SecurityUnauthorizedTitle                  string
	SecurityUnauthorizedMessage                string
	SecurityForbiddenTitle                     string
	SecurityForbiddenMessage                   string
	ErrorLabel                                 string
	NavigationEntries                          string
	NavigationSettings                         string
	NavigationReports                          string
	ReportingSettingsTitle                     string
	ReportingSettingsEyebrow                   string
	ReportingSettingsIntro                     string
	ReportingNotConfigured                     string
	ReportingRevisionLabel                     string
	ReportingCalendarHeading                   string
	ReportingStartMonthLabel                   string
	ReportingCalendarWarning                   string
	ReportingClassifications                   string
	ReportingClassificationHelp                string
	ReportingCategoryLabel                     string
	ReportingAddClassification                 string
	ReportingRemoveClassification              string
	ReportingFiscalYears                       string
	ReportingFiscalYearHelp                    string
	ReportingAddFiscalYear                     string
	ReportingRemoveFiscalYear                  string
	ReportingStartDateLabel                    string
	ReportingEndDateLabel                      string
	ReportingOpeningModeLabel                  string
	ReportingOpeningAutomatic                  string
	ReportingOpeningEntries                    string
	ReportingOpeningEntryIDs                   string
	ReportingOpeningEntryIDsHelp               string
	ReportingSave                              string
	ReportingUnclassifiedHeading               string
	ReportingUnclassifiedNone                  string
	ReportingInvalidFormMessage                string
	ReportingInvalidStartMonthMessage          string
	ReportingMissingFiscalYearsMessage         string
	ReportingInvalidClassificationMessage      string
	ReportingOverlappingClassificationsMessage string
	ReportingInvalidFiscalYearMessage          string
	ReportingNoncontiguousYearsMessage         string
	ReportingInvalidOpeningSettingsMessage     string
	ReportingOpeningNotApprovedMessage         string
	ReportingOpeningDateMismatchMessage        string
	ReportingOpeningTemporaryAccountMessage    string
	ReportingConflictMessage                   string
	ReportingSaveFailedMessage                 string
	TrialBalanceTitle                          string
	TrialBalanceEyebrow                        string
	TrialBalanceNotConfigured                  string
	TrialBalanceSetupLink                      string
	TrialBalancePeriodLabel                    string
	TrialBalanceShow                           string
	TrialBalanceInvalidPeriodMessage           string
	TrialBalanceClassificationWarning          string
	TrialBalanceEmpty                          string
	TrialBalanceConfigurationRevision          string
	TrialBalanceCommodity                      string
	TrialBalanceCategory                       string
	TrialBalanceDirect                         string
	TrialBalanceSubtotal                       string
	TrialBalanceOpeningDebit                   string
	TrialBalanceOpeningCredit                  string
	TrialBalanceDebitTurnover                  string
	TrialBalanceCreditTurnover                 string
	TrialBalanceClosingDebit                   string
	TrialBalanceClosingCredit                  string
	TrialBalanceWarnings                       string
	TrialBalanceTableHelp                      string
	ReportNavigationLabel                      string
	ReportNavigationCurrent                    string
	ReportNavigationTrialBalance               string
	ReportNavigationBalanceSheet               string
	ReportNavigationClosingBalanceSheet        string
	ReportNavigationIncomeStatement            string
	ReportNavigationBalanceTrend               string
	StatementNotConfigured                     string
	StatementSetupLink                         string
	StatementFiscalYearLabel                   string
	StatementMonthlyPeriodLabel                string
	StatementShow                              string
	StatementInvalidPeriodMessage              string
	StatementOpeningUnbalancedMessage          string
	StatementClosingUnbalancedMessage          string
	StatementClassificationWarning             string
	StatementConfigurationRevision             string
	StatementCommodity                         string
	StatementAmount                            string
	StatementActualSide                        string
	StatementDebitSide                         string
	StatementCreditSide                        string
	StatementDirect                            string
	StatementSubtotal                          string
	StatementEmpty                             string
	BalanceSheetTitle                          string
	BalanceSheetEyebrow                        string
	BalanceSheetAsOf                           string
	ClosingBalanceSheetTitle                   string
	ClosingBalanceSheetEyebrow                 string
	ClosingBalanceSheetIntro                   string
	ClosingBalanceSheetAsOf                    string
	ClosingBalanceSheetCurrentEarnings         string
	ClosingBalanceSheetBalanceDifference       string
	ClosingBalanceSheetOpeningUnbalanced       string
	IncomeStatementTitle                       string
	IncomeStatementEyebrow                     string
	IncomeStatementNetIncome                   string
	BalanceTrendTitle                          string
	BalanceTrendEyebrow                        string
	BalanceTrendHelp                           string
	CurrentOverviewTitle                       string
	CurrentOverviewEyebrow                     string
	CurrentOverviewIntro                       string
	CurrentOverviewDateLabel                   string
	CurrentOverviewExpenseMonthLabel           string
	CurrentOverviewShow                        string
	CurrentOverviewShowExpenses                string
	CurrentOverviewInvalidDate                 string
	CurrentOverviewAsOf                        string
	CurrentOverviewFiscalYear                  string
	CurrentBalancesHeading                     string
	CurrentBalancesHelp                        string
	CurrentExpenseHeading                      string
	CurrentExpenseHelp                         string
	CurrentExpensePeriod                       string
	CurrentExpenseTotal                        string
	CurrentOverviewEmptyBalances               string
	CurrentOverviewEmptyExpenses               string
	DrillDownAction                            string
	DrillDownTitle                             string
	DrillDownEyebrow                           string
	DrillDownScopeLabel                        string
	DrillDownScopeSubtree                      string
	DrillDownScopeDirect                       string
	DrillDownOpening                           string
	DrillDownMovement                          string
	DrillDownContributions                     string
	DrillDownEmpty                             string
	DrillDownBackToReport                      string
	DrillDownInvalidTitle                      string
	DrillDownInvalidMessage                    string
	DrillDownStaleTitle                        string
	DrillDownStaleMessage                      string
	CategoryAsset                              string
	CategoryLiability                          string
	CategoryEquity                             string
	CategoryRevenue                            string
	CategoryExpense                            string
	CategoryUnclassified                       string
	DevelopmentErrorDetail                     string

	EntriesShown            func(int) string
	CurrentCandidateEyebrow func(int) string
	RevisionHeader          func(int) string
	BaseRevisionSummary     func(base int, createdAt string, valid bool) string
	CurrentApprovalSummary  func(revision int, approvedAt string) string
	ApprovalSummary         func(sequence int64, revision int, approvedAt string) string
	RecordStatusSummary     func(recordIndex int, status string) string
	IdentitySummary         func(kind string, algorithmVersion int, digest string) string
	FiscalYearPeriodLabel   func(startDate, endDate string) string
	MonthlyPeriodLabel      func(month int, startDate, endDate string) string
}

func messagesFor(l locale) messages {
	switch l {
	case localeEN:
		return englishMessages()
	default:
		return japaneseMessages()
	}
}

func japaneseMessages() messages {
	return messages{
		LocaleName:                                 "日本語",
		SiteTagline:                                "ぶきっちょでも、複式簿記。",
		LanguageLabel:                              "表示言語",
		JapaneseLanguage:                           "日本語",
		EnglishLanguage:                            "English",
		IndexTitle:                                 "仕訳候補",
		IndexEyebrow:                               "Journal candidates",
		EmptyEntriesTitle:                          "仕訳候補はまだありません",
		EmptyEntriesMessage:                        "normalized input JSONをuploadすると仕訳候補が表示されます。",
		UploadFormLabel:                            "normalized inputをupload",
		UploadHeading:                              "取込",
		UploadFileLabel:                            "normalized input JSON",
		UploadHelp:                                 "version 1のJSON fileを1つ選択してください。最大10 MiBです。",
		UploadSubmit:                               "upload",
		TacklerUploadFormLabel:                     "Tackler txnをupload",
		TacklerUploadHeading:                       "Tackler import",
		TacklerUploadFileLabel:                     "Tackler .txn",
		TacklerUploadHelp:                          "対応subsetの.txn fileを1つ選択してください。最大10 MiBです。",
		TacklerUploadSubmit:                        "txnをupload",
		NoMatchingEntriesTitle:                     "条件に一致する仕訳候補はありません",
		NoMatchingEntriesMessage:                   "検索条件を変えて再実行してください。",
		SearchFormLabel:                            "仕訳候補を検索",
		SearchHeading:                              "検索",
		DateFromLabel:                              "日付From",
		DateToLabel:                                "日付To",
		AccountFilterLabel:                         "勘定科目",
		DescriptionFilterLabel:                     "摘要",
		ImportStatusFilterLabel:                    "取込状態",
		WorkflowFilterLabel:                        "確認状態",
		SourceNamespaceLabel:                       "Source namespace",
		SourceDisplayLabel:                         "Source display",
		SearchSubmit:                               "検索",
		ClearSearch:                                "条件をクリア",
		AllImportStatuses:                          "すべて",
		ImportStatusSuccess:                        "成功",
		ImportStatusWarning:                        "警告",
		AllWorkflowStatuses:                        "すべて",
		WorkflowStatusUnapproved:                   "未承認",
		WorkflowStatusInvalid:                      "不正",
		WorkflowStatusApproved:                     "承認済み",
		SearchResultsLabel:                         "検索結果",
		ExportHeading:                              "Export",
		ExportTackler:                              "Tackler",
		ExportJSON:                                 "JSON",
		NextPage:                                   "次のページ",
		InvalidUploadTitle:                         "Invalid upload",
		InvalidUploadMessage:                       "upload内容を処理できませんでした。",
		InvalidTacklerUploadTitle:                  "Invalid Tackler upload",
		InvalidTacklerUploadMessage:                "Tackler .txnを処理できませんでした。",
		UploadTooLargeTitle:                        "Upload too large",
		UploadTooLargeMessage:                      "uploadできるfileは10 MiBまでです。",
		UnsupportedUploadTitle:                     "Unsupported upload",
		UnsupportedUploadMessage:                   "multipart/form-dataでJSON fileを1つ送信してください。",
		UploadFailedTitle:                          "Upload failed",
		UploadFailedMessage:                        "取込を完了できませんでした。",
		UploadDisabledTitle:                        "Upload disabled",
		UploadDisabledMessage:                      "現在、この環境ではファイルの取込は無効です。",
		UploadForbiddenTitle:                       "Upload not permitted",
		UploadForbiddenMessage:                     "このアカウントにはファイル取込が許可されていません。",
		WriteForbiddenTitle:                        "変更は許可されていません",
		WriteForbiddenMessage:                      "このアカウントはデータを閲覧できますが、変更は許可されていません。",
		InvalidSearchTitle:                         "Invalid search",
		InvalidSearchMessage:                       "検索条件を処理できませんでした。",
		ImportStatusLabel:                          "取込",
		WorkflowStatusLabel:                        "確認",
		RevisionLabel:                              "Revision",
		BackToEntries:                              "仕訳候補へ戻る",
		SourceHeading:                              "Source",
		RunLabel:                                   "Run",
		RecordLabel:                                "record",
		CurrentCandidateHeading:                    "Current candidate",
		EditCandidateHeading:                       "修正",
		EditCandidateFormLabel:                     "仕訳候補を修正",
		EntryTextLabel:                             "Entry",
		EntryTextHelp:                              "1 entryをTackler風に書きます。先頭行は date  'description、以降は4 spaces indentの1行1postingです。Tabは4 spacesを入力します。",
		SaveRevision:                               "revisionを保存",
		ApproveCandidateHeading:                    "承認",
		ApproveCandidateFormLabel:                  "仕訳候補を承認",
		ApproveRevision:                            "承認",
		AlreadyApproved:                            "現在のrevisionは承認済みです。",
		CannotApproveInvalid:                       "現在のrevisionはvalidation errorがあるため承認できません。",
		FormErrorTitle:                             "操作を完了できませんでした",
		InvalidRevisionFormMessage:                 "修正内容を処理できませんでした。",
		RevisionConflictMessage:                    "仕訳候補が更新されています。内容を確認してからやり直してください。",
		RevisionFailedMessage:                      "revisionを保存できませんでした。",
		InvalidApprovalFormMessage:                 "承認内容を処理できませんでした。",
		ApprovalInvalidRevisionMessage:             "validation errorがあるrevisionは承認できません。",
		ApprovalConflictMessage:                    "仕訳候補が更新されています。内容を確認してから承認してください。",
		ApprovalFailedMessage:                      "承認を完了できませんでした。",
		OriginalSnapshotHeading:                    "Original snapshot",
		ImportDiagnostics:                          "Import diagnostics",
		None:                                       "なし",
		HistoryHeading:                             "履歴",
		CurrentRevisionLabel:                       "Current revision",
		CurrentApprovalLabel:                       "Current approval",
		Unapproved:                                 "未承認",
		ApprovalHistory:                            "Approval history",
		RunTitle:                                   "取込結果",
		RunEyebrow:                                 "Import run",
		NeedsReview:                                "要確認",
		Complete:                                   "完了",
		InputDigestLabel:                           "Input digest",
		PreStateGeneration:                         "Pre-state generation",
		OutcomesHeading:                            "Outcomes",
		IdentityLabel:                              "Identity",
		ViewEntry:                                  "仕訳を表示",
		AccountColumn:                              "Account",
		AmountColumn:                               "Amount",
		CommentColumn:                              "Comment",
		OmittedAmount:                              "省略",
		DiagnosticsLabel:                           "Diagnostics",
		MethodNotAllowedTitle:                      "Method not allowed",
		MethodNotAllowedMessage:                    "この操作には対応していません。",
		NotFoundTitle:                              "Not found",
		NotFoundMessage:                            "指定されたページは見つかりませんでした。",
		InternalErrorTitle:                         "Internal error",
		InternalErrorMessage:                       "ページを表示できませんでした。",
		SecurityUnauthorizedTitle:                  "Authentication required",
		SecurityUnauthorizedMessage:                "認証を確認できませんでした。ページを再読み込みしてやり直してください。",
		SecurityForbiddenTitle:                     "Request blocked",
		SecurityForbiddenMessage:                   "この画面からの送信として確認できませんでした。ページを再読み込みしてやり直してください。",
		ErrorLabel:                                 "Error",
		NavigationEntries:                          "仕訳候補",
		NavigationSettings:                         "レポート設定",
		NavigationReports:                          "レポート",
		ReportingSettingsTitle:                     "レポート設定",
		ReportingSettingsEyebrow:                   "Reporting configuration",
		ReportingSettingsIntro:                     "勘定科目の分類、会計年度、年度ごとの期首残高方式を1つの履歴revisionとして保存します。",
		ReportingNotConfigured:                     "レポート設定はまだありません。初回保存でrevision 1を作成します。",
		ReportingRevisionLabel:                     "Current revision",
		ReportingCalendarHeading:                   "Reporting calendar",
		ReportingStartMonthLabel:                   "会計年度の開始月",
		ReportingCalendarWarning:                   "開始月や年度を変更すると、承認済み仕訳は変更せず、過去期間を含む試算表を次回表示時に再集計します。",
		ReportingClassifications:                   "勘定科目の分類",
		ReportingClassificationHelp:                "親科目の分類は配下へ継承されます。空欄の行は保存されません。",
		ReportingCategoryLabel:                     "区分",
		ReportingFiscalYears:                       "会計年度と期首残高",
		ReportingFiscalYearHelp:                    "会計年度は連続した12か月の日付範囲で指定します。空欄の行は保存されません。",
		ReportingStartDateLabel:                    "開始日",
		ReportingEndDateLabel:                      "終了日",
		ReportingOpeningModeLabel:                  "期首残高方式",
		ReportingOpeningAutomatic:                  "自動繰越",
		ReportingOpeningEntries:                    "期首仕訳",
		ReportingOpeningEntryIDs:                   "期首仕訳ID",
		ReportingOpeningEntryIDsHelp:               "1行に1つ指定します。自動繰越中も指定は保持され、年度内の発生額から除外されます。",
		ReportingAddClassification:                 "勘定科目の分類を追加",
		ReportingRemoveClassification:              "この分類を削除",
		ReportingAddFiscalYear:                     "会計年度を追加",
		ReportingRemoveFiscalYear:                  "この会計年度を削除",
		ReportingSave:                              "設定revisionを保存",
		ReportingUnclassifiedHeading:               "未分類の勘定科目",
		ReportingUnclassifiedNone:                  "設定済み年度の承認済み仕訳に未分類科目はありません。",
		ReportingInvalidFormMessage:                "設定内容を処理できませんでした。日付、分類、期首仕訳を確認してください。",
		ReportingInvalidStartMonthMessage:          "会計年度の開始月は1から12で指定してください。",
		ReportingMissingFiscalYearsMessage:         "会計年度を1件以上指定してください。",
		ReportingInvalidClassificationMessage:      "勘定科目または区分が不正です。勘定科目と5区分の組み合わせを確認してください。",
		ReportingOverlappingClassificationsMessage: "親科目とその配下を重複して分類できません。親科目の分類だけを残してください。",
		ReportingInvalidFiscalYearMessage:          "会計年度の日付が開始月と一致しません。開始月の1日から12か月後の前日までを指定してください。",
		ReportingNoncontiguousYearsMessage:         "複数の会計年度は、前年度の翌日から連続するように指定してください。",
		ReportingInvalidOpeningSettingsMessage:     "期首残高方式または期首仕訳IDが不正です。期首仕訳方式では1件以上のIDが必要です。",
		ReportingOpeningNotApprovedMessage:         "指定した期首仕訳が存在しないか、現在のrevisionが承認済みではありません。",
		ReportingOpeningDateMismatchMessage:        "指定した期首仕訳の日付が会計年度の開始日と一致しません。",
		ReportingOpeningTemporaryAccountMessage:    "期首仕訳には資産・負債・純資産へ分類された勘定科目だけを使用できます。",
		ReportingConflictMessage:                   "レポート設定が更新されています。再読み込みしてからやり直してください。",
		ReportingSaveFailedMessage:                 "レポート設定を保存できませんでした。",
		TrialBalanceTitle:                          "試算表",
		TrialBalanceEyebrow:                        "Trial balance",
		TrialBalanceNotConfigured:                  "試算表を表示するには、先にレポート設定を保存してください。",
		TrialBalanceSetupLink:                      "レポート設定を開く",
		TrialBalancePeriodLabel:                    "期間",
		TrialBalanceShow:                           "試算表を表示",
		TrialBalanceInvalidPeriodMessage:           "設定された会計年度または月次期間を選択してください。",
		TrialBalanceClassificationWarning:          "未分類の勘定科目があります。金額には含まれていますが、分類設定を確認してください。",
		TrialBalanceEmpty:                          "この期間に表示する残高・発生額はありません。",
		TrialBalanceConfigurationRevision:          "Configuration revision",
		TrialBalanceCommodity:                      "Commodity",
		TrialBalanceCategory:                       "区分",
		TrialBalanceDirect:                         "直接計上",
		TrialBalanceSubtotal:                       "小計",
		TrialBalanceOpeningDebit:                   "期首借方",
		TrialBalanceOpeningCredit:                  "期首貸方",
		TrialBalanceDebitTurnover:                  "借方発生",
		TrialBalanceCreditTurnover:                 "貸方発生",
		TrialBalanceClosingDebit:                   "期末借方",
		TrialBalanceClosingCredit:                  "期末貸方",
		TrialBalanceWarnings:                       "Warning",
		TrialBalanceTableHelp:                      "各科目の金額は配下を含む小計です。直接計上額が小計と異なる場合だけ、次の補助行に表示します。",
		ReportNavigationLabel:                      "レポートの種類",
		ReportNavigationCurrent:                    "現在残高・月間費用",
		ReportNavigationTrialBalance:               "検証用試算表",
		ReportNavigationBalanceSheet:               "期首B/S",
		ReportNavigationClosingBalanceSheet:        "期末B/S",
		ReportNavigationIncomeStatement:            "月次P/L",
		ReportNavigationBalanceTrend:               "残高推移",
		StatementNotConfigured:                     "レポートを表示するには、先にレポート設定を保存してください。",
		StatementSetupLink:                         "レポート設定を開く",
		StatementFiscalYearLabel:                   "会計年度",
		StatementMonthlyPeriodLabel:                "月次期間",
		StatementShow:                              "レポートを表示",
		StatementInvalidPeriodMessage:              "設定された対象期間を選択してください。",
		StatementOpeningUnbalancedMessage:          "自動繰越で作成した期首残高の貸借が一致しません。レポート設定と前年度の残高を確認してください。",
		StatementClosingUnbalancedMessage:          "当期損益の表示補正後も期末残高の貸借が一致しません。レポート設定と仕訳を確認してください。",
		StatementClassificationWarning:             "未分類の勘定科目があります。金額には含まれていますが、分類設定を確認してください。",
		StatementConfigurationRevision:             "Configuration revision",
		StatementCommodity:                         "Commodity",
		StatementAmount:                            "金額",
		StatementActualSide:                        "実際の残高側",
		StatementDebitSide:                         "借方",
		StatementCreditSide:                        "貸方",
		StatementDirect:                            "直接計上",
		StatementSubtotal:                          "小計",
		StatementEmpty:                             "この期間に表示する金額はありません。",
		BalanceSheetTitle:                          "期首貸借対照表",
		BalanceSheetEyebrow:                        "Opening balance sheet",
		BalanceSheetAsOf:                           "期首日",
		ClosingBalanceSheetTitle:                   "期末貸借対照表",
		ClosingBalanceSheetEyebrow:                 "Period-end balance sheet",
		ClosingBalanceSheetIntro:                   "選択した会計年度の期末日時点を、現在承認済みの仕訳から再集計します。保存済み・締め済みの値ではありません。当期損益は仕訳や勘定科目を作らず、純資産側への表示補正として示します。",
		ClosingBalanceSheetAsOf:                    "期末日",
		ClosingBalanceSheetCurrentEarnings:         "当期損益（表示補正）",
		ClosingBalanceSheetBalanceDifference:       "貸借差額",
		ClosingBalanceSheetOpeningUnbalanced:       "期首残高の貸借が一致しません。レポート設定と期首残高を確認してください。",
		IncomeStatementTitle:                       "月次損益計算書",
		IncomeStatementEyebrow:                     "Monthly income statement",
		IncomeStatementNetIncome:                   "当月損益",
		BalanceTrendTitle:                          "勘定残高推移",
		BalanceTrendEyebrow:                        "Balance trend",
		BalanceTrendHelp:                           "各月末時点の全勘定残高です。収益・費用は会計年度の期首から累計しています。",
		CurrentOverviewTitle:                       "現在残高・月間費用",
		CurrentOverviewEyebrow:                     "Current balance and expenses",
		CurrentOverviewIntro:                       "参照日時点の資産・負債・純資産と、独立して選択した月に計上した費用を分けて表示します。決算書の貸借対照表ではありません。",
		CurrentOverviewDateLabel:                   "参照日",
		CurrentOverviewExpenseMonthLabel:           "費用月",
		CurrentOverviewShow:                        "残高基準日を変更",
		CurrentOverviewShowExpenses:                "費用月を変更",
		CurrentOverviewInvalidDate:                 "設定された会計年度内の残高基準日と費用月を指定してください。",
		CurrentOverviewAsOf:                        "残高基準日",
		CurrentOverviewFiscalYear:                  "会計年度",
		CurrentBalancesHeading:                     "現在残高",
		CurrentBalancesHelp:                        "資産・負債・純資産の実残高です。収益・費用は含めず、category間の貸借一致を表示条件にしません。",
		CurrentExpenseHeading:                      "月間費用",
		CurrentExpenseHelp:                         "費用に分類した勘定科目だけを選択月の開始日から終了日まで集計しています。",
		CurrentExpensePeriod:                       "集計期間",
		CurrentExpenseTotal:                        "費用合計",
		CurrentOverviewEmptyBalances:               "この日付に表示する資産・負債・純資産残高はありません。",
		CurrentOverviewEmptyExpenses:               "この期間に計上された費用はありません。",
		DrillDownAction:                            "根拠の仕訳",
		DrillDownTitle:                             "集計値の根拠",
		DrillDownEyebrow:                           "Report drill-down",
		DrillDownScopeLabel:                        "対象範囲",
		DrillDownScopeSubtree:                      "配下を含む小計",
		DrillDownScopeDirect:                       "直接計上",
		DrillDownOpening:                           "期首への寄与",
		DrillDownMovement:                          "期間内の発生",
		DrillDownContributions:                     "対象posting",
		DrillDownEmpty:                             "この集計値に寄与した仕訳はありません。",
		DrillDownBackToReport:                      "レポートへ戻る",
		DrillDownInvalidTitle:                      "Drill-downを表示できません",
		DrillDownInvalidMessage:                    "集計値の指定を処理できませんでした。レポートを再表示してやり直してください。",
		DrillDownStaleTitle:                        "レポートが更新されています",
		DrillDownStaleMessage:                      "表示後に承認済み仕訳またはレポート設定が変わりました。レポートを再表示してください。",
		CategoryAsset:                              "資産",
		CategoryLiability:                          "負債",
		CategoryEquity:                             "純資産",
		CategoryRevenue:                            "収益",
		CategoryExpense:                            "費用",
		CategoryUnclassified:                       "未分類",
		DevelopmentErrorDetail:                     "Development error detail",
		EntriesShown: func(count int) string {
			return fmt.Sprintf("%d件を表示", count)
		},
		CurrentCandidateEyebrow: func(revision int) string {
			return fmt.Sprintf("Current candidate · revision %d", revision)
		},
		RevisionHeader: func(revision int) string {
			return fmt.Sprintf("Revision %d", revision)
		},
		BaseRevisionSummary: func(base int, createdAt string, valid bool) string {
			return fmt.Sprintf("Base %d / %s / valid: %t", base, createdAt, valid)
		},
		CurrentApprovalSummary: func(revision int, approvedAt string) string {
			return fmt.Sprintf("revision %d / %s", revision, approvedAt)
		},
		ApprovalSummary: func(sequence int64, revision int, approvedAt string) string {
			return fmt.Sprintf("Sequence %d: revision %d / %s", sequence, revision, approvedAt)
		},
		RecordStatusSummary: func(recordIndex int, status string) string {
			return fmt.Sprintf("Record %d · %s", recordIndex, status)
		},
		IdentitySummary: func(kind string, algorithmVersion int, digest string) string {
			return fmt.Sprintf("%s v%d / %s", kind, algorithmVersion, digest)
		},
		FiscalYearPeriodLabel: func(startDate, endDate string) string {
			return fmt.Sprintf("会計年度 %s – %s", startDate, endDate)
		},
		MonthlyPeriodLabel: func(month int, startDate, endDate string) string {
			return fmt.Sprintf("月次 %d: %s – %s", month, startDate, endDate)
		},
	}
}

func englishMessages() messages {
	return messages{
		LocaleName:                                 "English",
		SiteTagline:                                "Double-entry bookkeeping, even when the source is messy.",
		LanguageLabel:                              "Language",
		JapaneseLanguage:                           "日本語",
		EnglishLanguage:                            "English",
		IndexTitle:                                 "Journal candidates",
		IndexEyebrow:                               "Journal candidates",
		EmptyEntriesTitle:                          "No journal candidates yet",
		EmptyEntriesMessage:                        "Upload normalized input JSON to show journal candidates.",
		UploadFormLabel:                            "Upload normalized input",
		UploadHeading:                              "Import",
		UploadFileLabel:                            "Normalized input JSON",
		UploadHelp:                                 "Choose one version 1 JSON file. The maximum size is 10 MiB.",
		UploadSubmit:                               "Upload",
		TacklerUploadFormLabel:                     "Upload Tackler txn",
		TacklerUploadHeading:                       "Tackler import",
		TacklerUploadFileLabel:                     "Tackler .txn",
		TacklerUploadHelp:                          "Choose one .txn file in the supported subset. The maximum size is 10 MiB.",
		TacklerUploadSubmit:                        "Upload txn",
		NoMatchingEntriesTitle:                     "No matching journal candidates",
		NoMatchingEntriesMessage:                   "Change the search filters and try again.",
		SearchFormLabel:                            "Search journal candidates",
		SearchHeading:                              "Search",
		DateFromLabel:                              "Date from",
		DateToLabel:                                "Date to",
		AccountFilterLabel:                         "Account",
		DescriptionFilterLabel:                     "Description",
		ImportStatusFilterLabel:                    "Import status",
		WorkflowFilterLabel:                        "Review status",
		SourceNamespaceLabel:                       "Source namespace",
		SourceDisplayLabel:                         "Source display",
		SearchSubmit:                               "Search",
		ClearSearch:                                "Clear",
		AllImportStatuses:                          "All",
		ImportStatusSuccess:                        "Success",
		ImportStatusWarning:                        "Warning",
		AllWorkflowStatuses:                        "All",
		WorkflowStatusUnapproved:                   "Unapproved",
		WorkflowStatusInvalid:                      "Invalid",
		WorkflowStatusApproved:                     "Approved",
		SearchResultsLabel:                         "Search results",
		ExportHeading:                              "Export",
		ExportTackler:                              "Tackler",
		ExportJSON:                                 "JSON",
		NextPage:                                   "Next page",
		InvalidUploadTitle:                         "Invalid upload",
		InvalidUploadMessage:                       "The upload could not be processed.",
		InvalidTacklerUploadTitle:                  "Invalid Tackler upload",
		InvalidTacklerUploadMessage:                "The Tackler .txn upload could not be processed.",
		UploadTooLargeTitle:                        "Upload too large",
		UploadTooLargeMessage:                      "Uploaded files must be 10 MiB or smaller.",
		UnsupportedUploadTitle:                     "Unsupported upload",
		UnsupportedUploadMessage:                   "Send one JSON file as multipart/form-data.",
		UploadFailedTitle:                          "Upload failed",
		UploadFailedMessage:                        "The import could not be completed.",
		UploadDisabledTitle:                        "Upload disabled",
		UploadDisabledMessage:                      "File imports are currently disabled in this environment.",
		UploadForbiddenTitle:                       "Upload not permitted",
		UploadForbiddenMessage:                     "This account is not permitted to import files.",
		WriteForbiddenTitle:                        "Changes not permitted",
		WriteForbiddenMessage:                      "This account can view data but is not permitted to make changes.",
		InvalidSearchTitle:                         "Invalid search",
		InvalidSearchMessage:                       "The search filters could not be processed.",
		ImportStatusLabel:                          "Import",
		WorkflowStatusLabel:                        "Review",
		RevisionLabel:                              "Revision",
		BackToEntries:                              "Back to journal candidates",
		SourceHeading:                              "Source",
		RunLabel:                                   "Run",
		RecordLabel:                                "record",
		CurrentCandidateHeading:                    "Current candidate",
		EditCandidateHeading:                       "Edit",
		EditCandidateFormLabel:                     "Edit journal candidate",
		EntryTextLabel:                             "Entry",
		EntryTextHelp:                              "Write one Tackler-style entry. The first line is date  'description, followed by one 4-space-indented posting per line. Tab inserts 4 spaces.",
		SaveRevision:                               "Save revision",
		ApproveCandidateHeading:                    "Approval",
		ApproveCandidateFormLabel:                  "Approve journal candidate",
		ApproveRevision:                            "Approve",
		AlreadyApproved:                            "The current revision is already approved.",
		CannotApproveInvalid:                       "The current revision has validation errors and cannot be approved.",
		FormErrorTitle:                             "The operation could not be completed",
		InvalidRevisionFormMessage:                 "The revision form could not be processed.",
		RevisionConflictMessage:                    "The journal candidate changed. Review it and try again.",
		RevisionFailedMessage:                      "The revision could not be saved.",
		InvalidApprovalFormMessage:                 "The approval form could not be processed.",
		ApprovalInvalidRevisionMessage:             "A revision with validation errors cannot be approved.",
		ApprovalConflictMessage:                    "The journal candidate changed. Review it before approving.",
		ApprovalFailedMessage:                      "The approval could not be completed.",
		OriginalSnapshotHeading:                    "Original snapshot",
		ImportDiagnostics:                          "Import diagnostics",
		None:                                       "None",
		HistoryHeading:                             "History",
		CurrentRevisionLabel:                       "Current revision",
		CurrentApprovalLabel:                       "Current approval",
		Unapproved:                                 "not approved",
		ApprovalHistory:                            "Approval history",
		RunTitle:                                   "Import result",
		RunEyebrow:                                 "Import run",
		NeedsReview:                                "Needs review",
		Complete:                                   "Complete",
		InputDigestLabel:                           "Input digest",
		PreStateGeneration:                         "Pre-state generation",
		OutcomesHeading:                            "Outcomes",
		IdentityLabel:                              "Identity",
		ViewEntry:                                  "View entry",
		AccountColumn:                              "Account",
		AmountColumn:                               "Amount",
		CommentColumn:                              "Comment",
		OmittedAmount:                              "omitted",
		DiagnosticsLabel:                           "Diagnostics",
		MethodNotAllowedTitle:                      "Method not allowed",
		MethodNotAllowedMessage:                    "This operation is not supported.",
		NotFoundTitle:                              "Not found",
		NotFoundMessage:                            "The requested page was not found.",
		InternalErrorTitle:                         "Internal error",
		InternalErrorMessage:                       "The page could not be displayed.",
		SecurityUnauthorizedTitle:                  "Authentication required",
		SecurityUnauthorizedMessage:                "Authentication could not be verified. Reload the page and try again.",
		SecurityForbiddenTitle:                     "Request blocked",
		SecurityForbiddenMessage:                   "The request could not be verified as coming from this page. Reload the page and try again.",
		ErrorLabel:                                 "Error",
		NavigationEntries:                          "Journal candidates",
		NavigationSettings:                         "Reporting settings",
		NavigationReports:                          "Reports",
		ReportingSettingsTitle:                     "Reporting settings",
		ReportingSettingsEyebrow:                   "Reporting configuration",
		ReportingSettingsIntro:                     "Save account classifications, fiscal years, and each year's opening-balance mode as one historical revision.",
		ReportingNotConfigured:                     "Reporting is not configured. The first save creates revision 1.",
		ReportingRevisionLabel:                     "Current revision",
		ReportingCalendarHeading:                   "Reporting calendar",
		ReportingStartMonthLabel:                   "Fiscal year start month",
		ReportingCalendarWarning:                   "Changing the month or fiscal years does not modify approved entries. Past periods are recalculated the next time a report is shown.",
		ReportingClassifications:                   "Account classifications",
		ReportingClassificationHelp:                "A parent classification applies to descendants. Blank rows are not saved.",
		ReportingCategoryLabel:                     "Category",
		ReportingAddClassification:                 "Add account classification",
		ReportingRemoveClassification:              "Remove this classification",
		ReportingFiscalYears:                       "Fiscal years and opening balances",
		ReportingFiscalYearHelp:                    "Enter each fiscal year as a continuous 12-month date range. Blank rows are not saved.",
		ReportingAddFiscalYear:                     "Add fiscal year",
		ReportingRemoveFiscalYear:                  "Remove this fiscal year",
		ReportingStartDateLabel:                    "Start date",
		ReportingEndDateLabel:                      "End date",
		ReportingOpeningModeLabel:                  "Opening-balance mode",
		ReportingOpeningAutomatic:                  "Automatic carry-forward",
		ReportingOpeningEntries:                    "Opening entries",
		ReportingOpeningEntryIDs:                   "Opening entry IDs",
		ReportingOpeningEntryIDsHelp:               "Enter one ID per line. IDs remain retained in automatic mode and are excluded from the year's turnover.",
		ReportingSave:                              "Save configuration revision",
		ReportingUnclassifiedHeading:               "Unclassified accounts",
		ReportingUnclassifiedNone:                  "No unclassified accounts occur in approved entries for the configured fiscal years.",
		ReportingInvalidFormMessage:                "The settings could not be processed. Check dates, classifications, and opening entries.",
		ReportingInvalidStartMonthMessage:          "Enter a fiscal year start month from 1 through 12.",
		ReportingMissingFiscalYearsMessage:         "Enter at least one fiscal year.",
		ReportingInvalidClassificationMessage:      "An account or category is invalid. Check each account and its five-category assignment.",
		ReportingOverlappingClassificationsMessage: "A parent and its descendant cannot both be classified. Keep only the parent classification.",
		ReportingInvalidFiscalYearMessage:          "A fiscal-year range does not match the start month. Use the first day of that month through the day before the same date next year.",
		ReportingNoncontiguousYearsMessage:         "Multiple fiscal years must be continuous, with each year starting the day after the previous one ends.",
		ReportingInvalidOpeningSettingsMessage:     "The opening-balance mode or entry IDs are invalid. Opening-entry mode requires at least one ID.",
		ReportingOpeningNotApprovedMessage:         "An opening entry does not exist or its current revision is not approved.",
		ReportingOpeningDateMismatchMessage:        "An opening entry date does not match the fiscal-year start date.",
		ReportingOpeningTemporaryAccountMessage:    "Opening entries may use only accounts classified as assets, liabilities, or equity.",
		ReportingConflictMessage:                   "The reporting configuration changed. Reload the page and try again.",
		ReportingSaveFailedMessage:                 "The reporting configuration could not be saved.",
		TrialBalanceTitle:                          "Trial balance",
		TrialBalanceEyebrow:                        "Trial balance",
		TrialBalanceNotConfigured:                  "Save reporting settings before viewing a trial balance.",
		TrialBalanceSetupLink:                      "Open reporting settings",
		TrialBalancePeriodLabel:                    "Period",
		TrialBalanceShow:                           "Show trial balance",
		TrialBalanceInvalidPeriodMessage:           "Select a configured fiscal-year or monthly period.",
		TrialBalanceClassificationWarning:          "Some accounts are unclassified. Their amounts are included; review the classification settings.",
		TrialBalanceEmpty:                          "There are no balances or movements to show for this period.",
		TrialBalanceConfigurationRevision:          "Configuration revision",
		TrialBalanceCommodity:                      "Commodity",
		TrialBalanceCategory:                       "Category",
		TrialBalanceDirect:                         "Direct",
		TrialBalanceSubtotal:                       "Subtotal",
		TrialBalanceOpeningDebit:                   "Opening debit",
		TrialBalanceOpeningCredit:                  "Opening credit",
		TrialBalanceDebitTurnover:                  "Debit turnover",
		TrialBalanceCreditTurnover:                 "Credit turnover",
		TrialBalanceClosingDebit:                   "Closing debit",
		TrialBalanceClosingCredit:                  "Closing credit",
		TrialBalanceWarnings:                       "Warning",
		TrialBalanceTableHelp:                      "Account amounts are subtotals including descendants. A detail row appears only when the direct amount differs from the subtotal.",
		ReportNavigationLabel:                      "Report type",
		ReportNavigationCurrent:                    "Current balance & expenses",
		ReportNavigationTrialBalance:               "Verification trial balance",
		ReportNavigationBalanceSheet:               "Opening B/S",
		ReportNavigationClosingBalanceSheet:        "Period-end B/S",
		ReportNavigationIncomeStatement:            "Monthly P/L",
		ReportNavigationBalanceTrend:               "Balance trend",
		StatementNotConfigured:                     "Save reporting settings before viewing a report.",
		StatementSetupLink:                         "Open reporting settings",
		StatementFiscalYearLabel:                   "Fiscal year",
		StatementMonthlyPeriodLabel:                "Monthly period",
		StatementShow:                              "Show report",
		StatementInvalidPeriodMessage:              "Select a configured reporting period.",
		StatementOpeningUnbalancedMessage:          "The opening balance created by automatic carry-forward is not balanced. Review the reporting settings and the prior-year balances.",
		StatementClosingUnbalancedMessage:          "The period-end balance remains unbalanced after the current-earnings presentation adjustment. Review the reporting settings and entries.",
		StatementClassificationWarning:             "Some accounts are unclassified. Their amounts are included; review the classification settings.",
		StatementConfigurationRevision:             "Configuration revision",
		StatementCommodity:                         "Commodity",
		StatementAmount:                            "Amount",
		StatementActualSide:                        "Actual balance side",
		StatementDebitSide:                         "Debit",
		StatementCreditSide:                        "Credit",
		StatementDirect:                            "Direct",
		StatementSubtotal:                          "Subtotal",
		StatementEmpty:                             "There are no amounts to show for this period.",
		BalanceSheetTitle:                          "Opening balance sheet",
		BalanceSheetEyebrow:                        "Opening balance sheet",
		BalanceSheetAsOf:                           "Opening date",
		ClosingBalanceSheetTitle:                   "Period-end balance sheet",
		ClosingBalanceSheetEyebrow:                 "Period-end balance sheet",
		ClosingBalanceSheetIntro:                   "Recalculates the selected fiscal year end from the currently approved entries. This is not a saved or closed snapshot. Current earnings are shown as a presentation adjustment to equity without creating an entry or account.",
		ClosingBalanceSheetAsOf:                    "Fiscal year end",
		ClosingBalanceSheetCurrentEarnings:         "Current earnings (presentation adjustment)",
		ClosingBalanceSheetBalanceDifference:       "Balance difference",
		ClosingBalanceSheetOpeningUnbalanced:       "The opening balance is not balanced. Review the reporting settings and opening balances.",
		IncomeStatementTitle:                       "Monthly income statement",
		IncomeStatementEyebrow:                     "Monthly income statement",
		IncomeStatementNetIncome:                   "Net income for the month",
		BalanceTrendTitle:                          "Account balance trend",
		BalanceTrendEyebrow:                        "Balance trend",
		BalanceTrendHelp:                           "All account balances at each month end. Revenue and expenses accumulate from the start of the fiscal year.",
		CurrentOverviewTitle:                       "Current balance and expenses",
		CurrentOverviewEyebrow:                     "Current balance and expenses",
		CurrentOverviewIntro:                       "Shows asset, liability, and equity balances at the selected date separately from expenses posted in an independently selected month. This is not a statutory balance sheet.",
		CurrentOverviewDateLabel:                   "As-of date",
		CurrentOverviewExpenseMonthLabel:           "Expense month",
		CurrentOverviewShow:                        "Change balance date",
		CurrentOverviewShowExpenses:                "Change expense month",
		CurrentOverviewInvalidDate:                 "Select a balance date and expense month within the configured fiscal years.",
		CurrentOverviewAsOf:                        "Balance date",
		CurrentOverviewFiscalYear:                  "Fiscal year",
		CurrentBalancesHeading:                     "Current balances",
		CurrentBalancesHelp:                        "Actual asset, liability, and equity balances. Revenue and expenses are excluded, and category balance is not required.",
		CurrentExpenseHeading:                      "Monthly expenses",
		CurrentExpenseHelp:                         "Includes only accounts classified as expenses for the complete selected month.",
		CurrentExpensePeriod:                       "Expense period",
		CurrentExpenseTotal:                        "Expense total",
		CurrentOverviewEmptyBalances:               "There are no asset, liability, or equity balances to show at this date.",
		CurrentOverviewEmptyExpenses:               "No expenses were posted in this period.",
		DrillDownAction:                            "View entries",
		DrillDownTitle:                             "Entries behind this amount",
		DrillDownEyebrow:                           "Report drill-down",
		DrillDownScopeLabel:                        "Scope",
		DrillDownScopeSubtree:                      "Subtotal including descendants",
		DrillDownScopeDirect:                       "Direct postings",
		DrillDownOpening:                           "Opening contribution",
		DrillDownMovement:                          "Period movement",
		DrillDownContributions:                     "Matching postings",
		DrillDownEmpty:                             "No approved entries contribute to this amount.",
		DrillDownBackToReport:                      "Back to report",
		DrillDownInvalidTitle:                      "Drill-down unavailable",
		DrillDownInvalidMessage:                    "The report selection could not be processed. Reload the report and try again.",
		DrillDownStaleTitle:                        "The report has changed",
		DrillDownStaleMessage:                      "Approved entries or reporting settings changed after this report was displayed. Reload the report.",
		CategoryAsset:                              "Asset",
		CategoryLiability:                          "Liability",
		CategoryEquity:                             "Equity",
		CategoryRevenue:                            "Revenue",
		CategoryExpense:                            "Expense",
		CategoryUnclassified:                       "Unclassified",
		DevelopmentErrorDetail:                     "Development error detail",
		EntriesShown: func(count int) string {
			if count == 1 {
				return "Showing 1 entry"
			}
			return fmt.Sprintf("Showing %d entries", count)
		},
		CurrentCandidateEyebrow: func(revision int) string {
			return fmt.Sprintf("Current candidate · revision %d", revision)
		},
		RevisionHeader: func(revision int) string {
			return fmt.Sprintf("Revision %d", revision)
		},
		BaseRevisionSummary: func(base int, createdAt string, valid bool) string {
			return fmt.Sprintf("Base %d / %s / valid: %t", base, createdAt, valid)
		},
		CurrentApprovalSummary: func(revision int, approvedAt string) string {
			return fmt.Sprintf("revision %d / %s", revision, approvedAt)
		},
		ApprovalSummary: func(sequence int64, revision int, approvedAt string) string {
			return fmt.Sprintf("Sequence %d: revision %d / %s", sequence, revision, approvedAt)
		},
		RecordStatusSummary: func(recordIndex int, status string) string {
			return fmt.Sprintf("Record %d · %s", recordIndex, status)
		},
		IdentitySummary: func(kind string, algorithmVersion int, digest string) string {
			return fmt.Sprintf("%s v%d / %s", kind, algorithmVersion, digest)
		},
		FiscalYearPeriodLabel: func(startDate, endDate string) string {
			return fmt.Sprintf("Fiscal year %s – %s", startDate, endDate)
		},
		MonthlyPeriodLabel: func(month int, startDate, endDate string) string {
			return fmt.Sprintf("Month %d: %s – %s", month, startDate, endDate)
		},
	}
}

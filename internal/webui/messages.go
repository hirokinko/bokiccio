package webui

import "fmt"

type messages struct {
	LocaleName                  string
	SiteTagline                 string
	LanguageLabel               string
	JapaneseLanguage            string
	EnglishLanguage             string
	IndexTitle                  string
	IndexEyebrow                string
	EmptyEntriesTitle           string
	EmptyEntriesMessage         string
	UploadFormLabel             string
	UploadHeading               string
	UploadFileLabel             string
	UploadHelp                  string
	UploadSubmit                string
	NoMatchingEntriesTitle      string
	NoMatchingEntriesMessage    string
	SearchFormLabel             string
	SearchHeading               string
	DateFromLabel               string
	DateToLabel                 string
	AccountFilterLabel          string
	DescriptionFilterLabel      string
	ImportStatusFilterLabel     string
	WorkflowFilterLabel         string
	SourceNamespaceLabel        string
	SourceDisplayLabel          string
	SearchSubmit                string
	ClearSearch                 string
	AllImportStatuses           string
	ImportStatusSuccess         string
	ImportStatusWarning         string
	AllWorkflowStatuses         string
	WorkflowStatusUnapproved    string
	WorkflowStatusInvalid       string
	WorkflowStatusApproved      string
	SearchResultsLabel          string
	NextPage                    string
	InvalidUploadTitle          string
	InvalidUploadMessage        string
	UploadTooLargeTitle         string
	UploadTooLargeMessage       string
	UnsupportedUploadTitle      string
	UnsupportedUploadMessage    string
	UploadFailedTitle           string
	UploadFailedMessage         string
	InvalidSearchTitle          string
	InvalidSearchMessage        string
	ImportStatusLabel           string
	WorkflowStatusLabel         string
	RevisionLabel               string
	BackToEntries               string
	SourceHeading               string
	RunLabel                    string
	RecordLabel                 string
	CurrentCandidateHeading     string
	OriginalSnapshotHeading     string
	ImportDiagnostics           string
	None                        string
	HistoryHeading              string
	CurrentRevisionLabel        string
	CurrentApprovalLabel        string
	Unapproved                  string
	ApprovalHistory             string
	RunTitle                    string
	RunEyebrow                  string
	NeedsReview                 string
	Complete                    string
	InputDigestLabel            string
	PreStateGeneration          string
	OutcomesHeading             string
	IdentityLabel               string
	ViewEntry                   string
	AccountColumn               string
	AmountColumn                string
	CommentColumn               string
	OmittedAmount               string
	DiagnosticsLabel            string
	MethodNotAllowedTitle       string
	MethodNotAllowedMessage     string
	NotFoundTitle               string
	NotFoundMessage             string
	InternalErrorTitle          string
	InternalErrorMessage        string
	SecurityUnauthorizedTitle   string
	SecurityUnauthorizedMessage string
	SecurityForbiddenTitle      string
	SecurityForbiddenMessage    string
	ErrorLabel                  string

	EntriesShown            func(int) string
	CurrentCandidateEyebrow func(int) string
	RevisionHeader          func(int) string
	BaseRevisionSummary     func(base int, createdAt string, valid bool) string
	CurrentApprovalSummary  func(revision int, approvedAt string) string
	ApprovalSummary         func(sequence int64, revision int, approvedAt string) string
	RecordStatusSummary     func(recordIndex int, status string) string
	IdentitySummary         func(kind string, algorithmVersion int, digest string) string
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
		LocaleName:                  "日本語",
		SiteTagline:                 "ぶきっちょでも、複式簿記。",
		LanguageLabel:               "表示言語",
		JapaneseLanguage:            "日本語",
		EnglishLanguage:             "English",
		IndexTitle:                  "仕訳候補",
		IndexEyebrow:                "Journal candidates",
		EmptyEntriesTitle:           "仕訳候補はまだありません",
		EmptyEntriesMessage:         "normalized input JSONをuploadすると仕訳候補が表示されます。",
		UploadFormLabel:             "normalized inputをupload",
		UploadHeading:               "取込",
		UploadFileLabel:             "normalized input JSON",
		UploadHelp:                  "version 1のJSON fileを1つ選択してください。最大10 MiBです。",
		UploadSubmit:                "upload",
		NoMatchingEntriesTitle:      "条件に一致する仕訳候補はありません",
		NoMatchingEntriesMessage:    "検索条件を変えて再実行してください。",
		SearchFormLabel:             "仕訳候補を検索",
		SearchHeading:               "検索",
		DateFromLabel:               "日付From",
		DateToLabel:                 "日付To",
		AccountFilterLabel:          "勘定科目",
		DescriptionFilterLabel:      "摘要",
		ImportStatusFilterLabel:     "取込状態",
		WorkflowFilterLabel:         "確認状態",
		SourceNamespaceLabel:        "Source namespace",
		SourceDisplayLabel:          "Source display",
		SearchSubmit:                "検索",
		ClearSearch:                 "条件をクリア",
		AllImportStatuses:           "すべて",
		ImportStatusSuccess:         "成功",
		ImportStatusWarning:         "警告",
		AllWorkflowStatuses:         "すべて",
		WorkflowStatusUnapproved:    "未承認",
		WorkflowStatusInvalid:       "不正",
		WorkflowStatusApproved:      "承認済み",
		SearchResultsLabel:          "検索結果",
		NextPage:                    "次のページ",
		InvalidUploadTitle:          "Invalid upload",
		InvalidUploadMessage:        "upload内容を処理できませんでした。",
		UploadTooLargeTitle:         "Upload too large",
		UploadTooLargeMessage:       "uploadできるfileは10 MiBまでです。",
		UnsupportedUploadTitle:      "Unsupported upload",
		UnsupportedUploadMessage:    "multipart/form-dataでJSON fileを1つ送信してください。",
		UploadFailedTitle:           "Upload failed",
		UploadFailedMessage:         "取込を完了できませんでした。",
		InvalidSearchTitle:          "Invalid search",
		InvalidSearchMessage:        "検索条件を処理できませんでした。",
		ImportStatusLabel:           "取込",
		WorkflowStatusLabel:         "確認",
		RevisionLabel:               "Revision",
		BackToEntries:               "仕訳候補へ戻る",
		SourceHeading:               "Source",
		RunLabel:                    "Run",
		RecordLabel:                 "record",
		CurrentCandidateHeading:     "Current candidate",
		OriginalSnapshotHeading:     "Original snapshot",
		ImportDiagnostics:           "Import diagnostics",
		None:                        "なし",
		HistoryHeading:              "履歴",
		CurrentRevisionLabel:        "Current revision",
		CurrentApprovalLabel:        "Current approval",
		Unapproved:                  "未承認",
		ApprovalHistory:             "Approval history",
		RunTitle:                    "取込結果",
		RunEyebrow:                  "Import run",
		NeedsReview:                 "要確認",
		Complete:                    "完了",
		InputDigestLabel:            "Input digest",
		PreStateGeneration:          "Pre-state generation",
		OutcomesHeading:             "Outcomes",
		IdentityLabel:               "Identity",
		ViewEntry:                   "仕訳を表示",
		AccountColumn:               "Account",
		AmountColumn:                "Amount",
		CommentColumn:               "Comment",
		OmittedAmount:               "省略",
		DiagnosticsLabel:            "Diagnostics",
		MethodNotAllowedTitle:       "Method not allowed",
		MethodNotAllowedMessage:     "この操作には対応していません。",
		NotFoundTitle:               "Not found",
		NotFoundMessage:             "指定されたページは見つかりませんでした。",
		InternalErrorTitle:          "Internal error",
		InternalErrorMessage:        "ページを表示できませんでした。",
		SecurityUnauthorizedTitle:   "Authentication required",
		SecurityUnauthorizedMessage: "認証を確認できませんでした。ページを再読み込みしてやり直してください。",
		SecurityForbiddenTitle:      "Request blocked",
		SecurityForbiddenMessage:    "この画面からの送信として確認できませんでした。ページを再読み込みしてやり直してください。",
		ErrorLabel:                  "Error",
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
	}
}

func englishMessages() messages {
	return messages{
		LocaleName:                  "English",
		SiteTagline:                 "Double-entry bookkeeping, even when the source is messy.",
		LanguageLabel:               "Language",
		JapaneseLanguage:            "日本語",
		EnglishLanguage:             "English",
		IndexTitle:                  "Journal candidates",
		IndexEyebrow:                "Journal candidates",
		EmptyEntriesTitle:           "No journal candidates yet",
		EmptyEntriesMessage:         "Upload normalized input JSON to show journal candidates.",
		UploadFormLabel:             "Upload normalized input",
		UploadHeading:               "Import",
		UploadFileLabel:             "Normalized input JSON",
		UploadHelp:                  "Choose one version 1 JSON file. The maximum size is 10 MiB.",
		UploadSubmit:                "Upload",
		NoMatchingEntriesTitle:      "No matching journal candidates",
		NoMatchingEntriesMessage:    "Change the search filters and try again.",
		SearchFormLabel:             "Search journal candidates",
		SearchHeading:               "Search",
		DateFromLabel:               "Date from",
		DateToLabel:                 "Date to",
		AccountFilterLabel:          "Account",
		DescriptionFilterLabel:      "Description",
		ImportStatusFilterLabel:     "Import status",
		WorkflowFilterLabel:         "Review status",
		SourceNamespaceLabel:        "Source namespace",
		SourceDisplayLabel:          "Source display",
		SearchSubmit:                "Search",
		ClearSearch:                 "Clear",
		AllImportStatuses:           "All",
		ImportStatusSuccess:         "Success",
		ImportStatusWarning:         "Warning",
		AllWorkflowStatuses:         "All",
		WorkflowStatusUnapproved:    "Unapproved",
		WorkflowStatusInvalid:       "Invalid",
		WorkflowStatusApproved:      "Approved",
		SearchResultsLabel:          "Search results",
		NextPage:                    "Next page",
		InvalidUploadTitle:          "Invalid upload",
		InvalidUploadMessage:        "The upload could not be processed.",
		UploadTooLargeTitle:         "Upload too large",
		UploadTooLargeMessage:       "Uploaded files must be 10 MiB or smaller.",
		UnsupportedUploadTitle:      "Unsupported upload",
		UnsupportedUploadMessage:    "Send one JSON file as multipart/form-data.",
		UploadFailedTitle:           "Upload failed",
		UploadFailedMessage:         "The import could not be completed.",
		InvalidSearchTitle:          "Invalid search",
		InvalidSearchMessage:        "The search filters could not be processed.",
		ImportStatusLabel:           "Import",
		WorkflowStatusLabel:         "Review",
		RevisionLabel:               "Revision",
		BackToEntries:               "Back to journal candidates",
		SourceHeading:               "Source",
		RunLabel:                    "Run",
		RecordLabel:                 "record",
		CurrentCandidateHeading:     "Current candidate",
		OriginalSnapshotHeading:     "Original snapshot",
		ImportDiagnostics:           "Import diagnostics",
		None:                        "None",
		HistoryHeading:              "History",
		CurrentRevisionLabel:        "Current revision",
		CurrentApprovalLabel:        "Current approval",
		Unapproved:                  "not approved",
		ApprovalHistory:             "Approval history",
		RunTitle:                    "Import result",
		RunEyebrow:                  "Import run",
		NeedsReview:                 "Needs review",
		Complete:                    "Complete",
		InputDigestLabel:            "Input digest",
		PreStateGeneration:          "Pre-state generation",
		OutcomesHeading:             "Outcomes",
		IdentityLabel:               "Identity",
		ViewEntry:                   "View entry",
		AccountColumn:               "Account",
		AmountColumn:                "Amount",
		CommentColumn:               "Comment",
		OmittedAmount:               "omitted",
		DiagnosticsLabel:            "Diagnostics",
		MethodNotAllowedTitle:       "Method not allowed",
		MethodNotAllowedMessage:     "This operation is not supported.",
		NotFoundTitle:               "Not found",
		NotFoundMessage:             "The requested page was not found.",
		InternalErrorTitle:          "Internal error",
		InternalErrorMessage:        "The page could not be displayed.",
		SecurityUnauthorizedTitle:   "Authentication required",
		SecurityUnauthorizedMessage: "Authentication could not be verified. Reload the page and try again.",
		SecurityForbiddenTitle:      "Request blocked",
		SecurityForbiddenMessage:    "The request could not be verified as coming from this page. Reload the page and try again.",
		ErrorLabel:                  "Error",
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
	}
}

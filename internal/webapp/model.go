package webapp

import (
	"context"
	"errors"

	"github.com/hirokinko/bokiccio/internal/ledger"
	"github.com/hirokinko/bokiccio/internal/reporting"
)

const APISchemaVersion = 1

var (
	ErrNotFound               = errors.New("web resource not found")
	ErrConflict               = errors.New("web resource conflict")
	ErrInvalidRequest         = errors.New("invalid web request")
	ErrInvalidRevision        = errors.New("invalid entry revision")
	ErrReportingNotConfigured = errors.New("financial reporting is not configured")
	ErrUploadDisabled         = errors.New("file upload is disabled")
	ErrUploadForbidden        = errors.New("file upload is not permitted for this user")
	ErrWriteForbidden         = errors.New("data changes are not permitted for this user")
	ErrReportSnapshotChanged  = errors.New("report snapshot changed")
)

type ReportingConfigurationErrorCode string

const (
	ReportingOpeningEntryNotApproved      ReportingConfigurationErrorCode = "opening_entry_not_approved"
	ReportingOpeningEntryDateMismatch     ReportingConfigurationErrorCode = "opening_entry_date_mismatch"
	ReportingOpeningEntryTemporaryAccount ReportingConfigurationErrorCode = "opening_entry_temporary_account"
)

type ReportingConfigurationError struct {
	Code ReportingConfigurationErrorCode
}

func (err *ReportingConfigurationError) Error() string {
	return "invalid reporting configuration: " + string(err.Code)
}

func (err *ReportingConfigurationError) Unwrap() error {
	return ErrInvalidRequest
}

type Repository interface {
	Import(context.Context, string, []byte) (ImportResult, error)
	GetApplicationSettings(context.Context) (ApplicationSettings, error)
	GetUserAccess(context.Context, string) (UserAccess, error)
	GetRun(context.Context, string) (RunDetail, error)
	ListEntries(context.Context, EntryQuery) (EntryPage, error)
	GetEntry(context.Context, string) (EntryDetail, error)
	CreateRevision(context.Context, string, string, RevisionRequest) (RevisionDetail, error)
	ApproveRevision(context.Context, string, string, ApprovalRequest) (ApprovalDetail, error)
	ListApprovedEntries(context.Context, EntryFilter) ([]ApprovedEntry, error)
	GetCurrentReportingConfiguration(context.Context) (ReportingConfigurationDetail, error)
	GetReportingConfiguration(context.Context, int) (ReportingConfigurationDetail, error)
	CreateReportingConfiguration(context.Context, string, ReportingConfigurationRequest) (ReportingConfigurationDetail, error)
	GetTrialBalance(context.Context, reporting.Period) (TrialBalanceDetail, error)
	GetBalanceSheet(context.Context, reporting.Period) (BalanceSheetDetail, error)
	GetClosingBalanceSheet(context.Context, reporting.Period) (ClosingBalanceSheetDetail, error)
	GetIncomeStatement(context.Context, reporting.Period) (IncomeStatementDetail, error)
	GetBalanceTrend(context.Context, reporting.Period) (BalanceTrendDetail, error)
	GetCurrentOverview(context.Context, string, reporting.Period) (CurrentOverviewDetail, error)
	GetTrialBalanceDrillDown(context.Context, ReportDrillDownQuery) (TrialBalanceDrillDownDetail, error)
	GetIncomeStatementDrillDown(context.Context, ReportDrillDownQuery) (IncomeStatementDrillDownDetail, error)
}

type ApplicationSettings struct {
	FileUploadEnabled bool
}

type UserAccess struct {
	FileUploadEnabled bool
	CanWrite          bool
}

type ImportResult struct {
	RunIdentity string        `json:"run_identity"`
	HasErrors   bool          `json:"has_errors"`
	Counts      OutcomeCounts `json:"counts"`
	DetailURL   string        `json:"detail_url"`
}

type OutcomeCounts struct {
	Success   int `json:"success"`
	Warning   int `json:"warning"`
	Error     int `json:"error"`
	Duplicate int `json:"duplicate"`
}

type RunDetail struct {
	SchemaVersion      int             `json:"schema_version"`
	RunIdentity        string          `json:"run_identity"`
	InputDigest        string          `json:"input_digest"`
	PreStateGeneration uint64          `json:"pre_state_generation"`
	HasErrors          bool            `json:"has_errors"`
	Outcomes           []OutcomeDetail `json:"outcomes"`
}

type OutcomeDetail struct {
	RecordIndex int                `json:"record_index"`
	Status      string             `json:"status"`
	Identity    Identity           `json:"identity"`
	Source      Source             `json:"source"`
	Diagnostics []DiagnosticDetail `json:"diagnostics"`
	EntryID     string             `json:"entry_id,omitempty"`
}

type Identity struct {
	Kind             string `json:"kind"`
	AlgorithmVersion int    `json:"algorithm_version"`
	Digest           string `json:"digest"`
}

type Source struct {
	Namespace string `json:"namespace"`
	Display   string `json:"display"`
}

type DiagnosticDetail struct {
	Code         string `json:"code"`
	Severity     string `json:"severity"`
	Message      string `json:"message"`
	FieldPath    string `json:"field_path,omitempty"`
	PostingIndex *int   `json:"posting_index,omitempty"`
}

type EntryPage struct {
	SchemaVersion int            `json:"schema_version"`
	Entries       []EntrySummary `json:"entries"`
	NextCursor    string         `json:"next_cursor,omitempty"`
}

type EntrySummary struct {
	ID              string `json:"id"`
	OccurredAt      string `json:"occurred_at"`
	Description     string `json:"description"`
	Status          string `json:"status"`
	WorkflowStatus  string `json:"workflow_status"`
	CurrentRevision int    `json:"current_revision"`
	Source          Source `json:"source"`
}

type EntryFilter struct {
	DateFrom        string
	DateTo          string
	Account         string
	Description     string
	Status          string
	WorkflowStatus  string
	SourceNamespace string
	SourceDisplay   string
}

type EntryQuery struct {
	Filter EntryFilter
	Limit  int
	Cursor string
}

type EntryDetail struct {
	SchemaVersion   int                `json:"schema_version"`
	ID              string             `json:"id"`
	RunIdentity     string             `json:"run_identity"`
	RecordIndex     int                `json:"record_index"`
	OccurredAt      string             `json:"occurred_at"`
	Description     string             `json:"description"`
	Comments        []string           `json:"comments"`
	Postings        []PostingDetail    `json:"postings"`
	Status          string             `json:"status"`
	Source          Source             `json:"source"`
	Diagnostics     []DiagnosticDetail `json:"diagnostics"`
	CurrentRevision int                `json:"current_revision"`
	CurrentApproval *ApprovalDetail    `json:"current_approval,omitempty"`
	Revisions       []RevisionDetail   `json:"revisions"`
	Approvals       []ApprovalDetail   `json:"approvals"`
}

type PostingDetail struct {
	Account    string        `json:"account"`
	Amount     *string       `json:"amount,omitempty"`
	Commodity  string        `json:"commodity,omitempty"`
	TotalPrice *AmountDetail `json:"total_price,omitempty"`
	Comment    string        `json:"comment,omitempty"`
}

type AmountDetail struct {
	Amount    string `json:"amount"`
	Commodity string `json:"commodity"`
}

type RevisionRequest struct {
	BaseRevision *int            `json:"base_revision"`
	OccurredAt   string          `json:"occurred_at"`
	Description  string          `json:"description"`
	Comments     []string        `json:"comments"`
	Postings     []PostingDetail `json:"postings"`
}

type RevisionDetail struct {
	Revision     int                `json:"revision"`
	BaseRevision int                `json:"base_revision"`
	CreatedAt    string             `json:"created_at"`
	OccurredAt   string             `json:"occurred_at"`
	Description  string             `json:"description"`
	Comments     []string           `json:"comments"`
	Postings     []PostingDetail    `json:"postings"`
	Valid        bool               `json:"valid"`
	Diagnostics  []DiagnosticDetail `json:"diagnostics"`
}

type ApprovalRequest struct {
	Revision *int `json:"revision"`
}

type ApprovalDetail struct {
	Sequence   int64  `json:"sequence"`
	Revision   int    `json:"revision"`
	ApprovedAt string `json:"approved_at"`
}

type ApprovedEntry struct {
	ID         string
	Revision   int
	ApprovedAt string
	Source     Source
	Entry      ledger.JournalEntry
}

type ExportEntry struct {
	ID          string          `json:"id"`
	Revision    int             `json:"revision"`
	ApprovedAt  string          `json:"approved_at"`
	Source      Source          `json:"source"`
	OccurredAt  string          `json:"occurred_at"`
	Description string          `json:"description"`
	Comments    []string        `json:"comments"`
	Postings    []PostingDetail `json:"postings"`
}

type JSONExport struct {
	SchemaVersion int           `json:"schema_version"`
	Entries       []ExportEntry `json:"entries"`
}

type ReportingClassification struct {
	Account  string             `json:"account"`
	Category reporting.Category `json:"category"`
}

type ReportingFiscalYear struct {
	StartDate       string                `json:"start_date"`
	EndDate         string                `json:"end_date"`
	OpeningMode     reporting.OpeningMode `json:"opening_mode"`
	OpeningEntryIDs []string              `json:"opening_entry_ids"`
}

type ReportingConfigurationRequest struct {
	BaseRevision    *int                      `json:"base_revision"`
	StartMonth      int                       `json:"start_month"`
	Classifications []ReportingClassification `json:"classifications"`
	FiscalYears     []ReportingFiscalYear     `json:"fiscal_years"`
}

type ReportingConfigurationDetail struct {
	SchemaVersion   int                       `json:"schema_version"`
	Revision        int                       `json:"revision"`
	BaseRevision    int                       `json:"base_revision"`
	CreatedAt       string                    `json:"created_at"`
	StartMonth      int                       `json:"start_month"`
	Classifications []ReportingClassification `json:"classifications"`
	FiscalYears     []ReportingFiscalYear     `json:"fiscal_years"`
}

type TrialBalanceDetail struct {
	SchemaVersion    int    `json:"schema_version"`
	SnapshotIdentity string `json:"snapshot_identity"`
	reporting.TrialBalance
}

type BalanceSheetDetail struct {
	SchemaVersion int `json:"schema_version"`
	reporting.BalanceSheet
}

type ClosingBalanceSheetDetail struct {
	SchemaVersion int `json:"schema_version"`
	reporting.ClosingBalanceSheet
}

type IncomeStatementDetail struct {
	SchemaVersion    int    `json:"schema_version"`
	SnapshotIdentity string `json:"snapshot_identity"`
	reporting.IncomeStatement
}

type BalanceTrendDetail struct {
	SchemaVersion int `json:"schema_version"`
	reporting.BalanceTrend
}

type CurrentOverviewDetail struct {
	SchemaVersion int `json:"schema_version"`
	reporting.CurrentOverview
}

type ReportDrillDownQuery struct {
	DrillDown        reporting.DrillDownQuery
	SnapshotIdentity string
	Limit            int
	Cursor           string
}

type TrialBalanceDrillDownDetail struct {
	SchemaVersion    int    `json:"schema_version"`
	SnapshotIdentity string `json:"snapshot_identity"`
	TotalEntries     int    `json:"total_entries"`
	NextCursor       string `json:"next_cursor,omitempty"`
	reporting.TrialBalanceDrillDown
}

type IncomeStatementDrillDownDetail struct {
	SchemaVersion    int    `json:"schema_version"`
	SnapshotIdentity string `json:"snapshot_identity"`
	TotalEntries     int    `json:"total_entries"`
	NextCursor       string `json:"next_cursor,omitempty"`
	reporting.IncomeStatementDrillDown
}

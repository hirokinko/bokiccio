// Package webui provides Bokiccio's server-rendered interface.
package webui

import (
	"crypto/sha256"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"mime"
	"mime/multipart"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/a-h/templ"
	"github.com/hirokinko/bokiccio/internal/ingest"
	"github.com/hirokinko/bokiccio/internal/ledger"
	"github.com/hirokinko/bokiccio/internal/reporting"
	"github.com/hirokinko/bokiccio/internal/tacklerfmt"
	"github.com/hirokinko/bokiccio/internal/webapp"
)

const (
	defaultPageSize         = 50
	maxSearchFormBody       = 16 << 10
	maxRevisionFormBody     = 64 << 10
	maxReportingFormBody    = 256 << 10
	maxImportFileSize       = 10 << 20
	maxImportRequestSize    = maxImportFileSize + (64 << 10)
	importFileField         = "file"
	currentBalancesResultID = "current-balances-result"
	currentExpensesResultID = "current-expenses-result"
)

//go:embed assets/app.css assets/app.js assets/htmx-2.0.10.min.js
var assetFiles embed.FS

type Handler struct {
	repository  webapp.Repository
	development bool
	now         func() time.Time
}

type HandlerOptions struct {
	Development bool
	Now         func() time.Time
}

func NewHandler(repository webapp.Repository, options ...HandlerOptions) *Handler {
	handler := &Handler{repository: repository, now: time.Now}
	if len(options) > 0 {
		handler.development = options[0].Development
		if options[0].Now != nil {
			handler.now = options[0].Now
		}
	}
	return handler
}

func RenderSecurityError(response http.ResponseWriter, request *http.Request, securityError webapp.SecurityError) {
	setPrivateHeaders(response)
	requestLocale, _ := localeRoute(request.URL.Path)
	msg := messagesFor(requestLocale)
	title := msg.SecurityForbiddenTitle
	message := msg.SecurityForbiddenMessage
	if securityError.Status == http.StatusUnauthorized {
		title = msg.SecurityUnauthorizedTitle
		message = msg.SecurityUnauthorizedMessage
	}
	render(response, request, securityError.Status, errorPage(errorPageModel{
		Page: newPageContext(requestLocale, "/"), Status: securityError.Status,
		Title: title, Message: message,
	}))
}

func (handler *Handler) ServeHTTP(response http.ResponseWriter, request *http.Request) {
	setPrivateHeaders(response)
	if request.URL.Path == "/en" {
		http.Redirect(response, request, "/en/", http.StatusPermanentRedirect)
		return
	}
	if request.URL.Path == "/assets/app.css" {
		handler.asset(response, request, "assets/app.css", "text/css; charset=utf-8")
		return
	}
	if request.URL.Path == "/assets/htmx-2.0.10.min.js" {
		handler.asset(response, request, "assets/htmx-2.0.10.min.js", "text/javascript; charset=utf-8")
		return
	}
	if request.URL.Path == "/assets/app.js" {
		handler.asset(response, request, "assets/app.js", "text/javascript; charset=utf-8")
		return
	}

	requestLocale, localPath := localeRoute(request.URL.Path)
	switch {
	case localPath == "/":
		if request.Method != http.MethodGet && request.Method != http.MethodHead {
			handler.methodNotAllowed(response, request, requestLocale, localPath, "GET, HEAD")
			return
		}
		handler.index(response, request, requestLocale)
	case localPath == "/ui/entries/search":
		if request.Method != http.MethodPost {
			handler.methodNotAllowed(response, request, requestLocale, localPath, "POST")
			return
		}
		handler.searchEntries(response, request, requestLocale)
	case localPath == "/ui/imports":
		if request.Method != http.MethodPost {
			handler.methodNotAllowed(response, request, requestLocale, localPath, "POST")
			return
		}
		handler.importRecords(response, request, requestLocale)
	case localPath == "/ui/imports/tackler":
		if request.Method != http.MethodPost {
			handler.methodNotAllowed(response, request, requestLocale, localPath, "POST")
			return
		}
		handler.importTackler(response, request, requestLocale)
	case localPath == "/settings/reporting":
		if request.Method != http.MethodGet && request.Method != http.MethodHead {
			handler.methodNotAllowed(response, request, requestLocale, localPath, "GET, HEAD")
			return
		}
		handler.reportingSettings(response, request, requestLocale)
	case localPath == "/ui/settings/reporting":
		if request.Method != http.MethodPost {
			handler.methodNotAllowed(response, request, requestLocale, localPath, "POST")
			return
		}
		handler.updateReportingSettings(response, request, requestLocale)
	case localPath == "/reports/trial-balance":
		if request.Method != http.MethodGet && request.Method != http.MethodHead {
			handler.methodNotAllowed(response, request, requestLocale, localPath, "GET, HEAD")
			return
		}
		handler.trialBalance(response, request, requestLocale)
	case localPath == "/reports/current":
		if request.Method != http.MethodGet && request.Method != http.MethodHead {
			handler.methodNotAllowed(response, request, requestLocale, localPath, "GET, HEAD")
			return
		}
		handler.currentOverview(response, request, requestLocale)
	case localPath == "/ui/reports/current":
		if request.Method != http.MethodPost {
			handler.methodNotAllowed(response, request, requestLocale, localPath, "POST")
			return
		}
		handler.selectCurrentOverview(response, request, requestLocale)
	case localPath == "/ui/reports/trial-balance":
		if request.Method != http.MethodPost {
			handler.methodNotAllowed(response, request, requestLocale, localPath, "POST")
			return
		}
		handler.selectTrialBalance(response, request, requestLocale)
	case localPath == "/ui/reports/trial-balance/drill-down":
		if request.Method != http.MethodPost {
			handler.methodNotAllowed(response, request, requestLocale, localPath, "POST")
			return
		}
		handler.reportDrillDown(response, request, requestLocale, "trial-balance")
	case localPath == "/reports/balance-sheet":
		if request.Method != http.MethodGet && request.Method != http.MethodHead {
			handler.methodNotAllowed(response, request, requestLocale, localPath, "GET, HEAD")
			return
		}
		handler.balanceSheet(response, request, requestLocale)
	case localPath == "/ui/reports/balance-sheet":
		if request.Method != http.MethodPost {
			handler.methodNotAllowed(response, request, requestLocale, localPath, "POST")
			return
		}
		handler.selectStatementPeriod(response, request, requestLocale, balanceSheetHref(requestLocale))
	case localPath == "/reports/closing-balance-sheet":
		if request.Method != http.MethodGet && request.Method != http.MethodHead {
			handler.methodNotAllowed(response, request, requestLocale, localPath, "GET, HEAD")
			return
		}
		handler.closingBalanceSheet(response, request, requestLocale)
	case localPath == "/ui/reports/closing-balance-sheet":
		if request.Method != http.MethodPost {
			handler.methodNotAllowed(response, request, requestLocale, localPath, "POST")
			return
		}
		handler.selectStatementPeriod(response, request, requestLocale, closingBalanceSheetHref(requestLocale))
	case localPath == "/reports/income-statement":
		if request.Method != http.MethodGet && request.Method != http.MethodHead {
			handler.methodNotAllowed(response, request, requestLocale, localPath, "GET, HEAD")
			return
		}
		handler.incomeStatement(response, request, requestLocale)
	case localPath == "/ui/reports/income-statement":
		if request.Method != http.MethodPost {
			handler.methodNotAllowed(response, request, requestLocale, localPath, "POST")
			return
		}
		handler.selectStatementPeriod(response, request, requestLocale, incomeStatementHref(requestLocale))
	case localPath == "/ui/reports/income-statement/drill-down":
		if request.Method != http.MethodPost {
			handler.methodNotAllowed(response, request, requestLocale, localPath, "POST")
			return
		}
		handler.reportDrillDown(response, request, requestLocale, "income-statement")
	case localPath == "/reports/balance-trend":
		if request.Method != http.MethodGet && request.Method != http.MethodHead {
			handler.methodNotAllowed(response, request, requestLocale, localPath, "GET, HEAD")
			return
		}
		handler.balanceTrend(response, request, requestLocale)
	case localPath == "/ui/reports/balance-trend":
		if request.Method != http.MethodPost {
			handler.methodNotAllowed(response, request, requestLocale, localPath, "POST")
			return
		}
		handler.selectStatementPeriod(response, request, requestLocale, balanceTrendHref(requestLocale))
	case strings.HasPrefix(localPath, "/ui/exports/"):
		if request.Method != http.MethodPost {
			handler.methodNotAllowed(response, request, requestLocale, localPath, "POST")
			return
		}
		handler.exportEntries(response, request, requestLocale, strings.TrimPrefix(localPath, "/ui/exports/"))
	case strings.HasPrefix(localPath, "/ui/entries/"):
		if request.Method != http.MethodPost {
			handler.methodNotAllowed(response, request, requestLocale, localPath, "POST")
			return
		}
		handler.entryMutation(response, request, requestLocale, strings.TrimPrefix(localPath, "/ui/entries/"))
	case strings.HasPrefix(localPath, "/entries/"):
		if request.Method != http.MethodGet && request.Method != http.MethodHead {
			handler.methodNotAllowed(response, request, requestLocale, localPath, "GET, HEAD")
			return
		}
		handler.entry(response, request, requestLocale, strings.TrimPrefix(localPath, "/entries/"))
	case strings.HasPrefix(localPath, "/imports/"):
		if request.Method != http.MethodGet && request.Method != http.MethodHead {
			handler.methodNotAllowed(response, request, requestLocale, localPath, "GET, HEAD")
			return
		}
		handler.run(response, request, requestLocale, strings.TrimPrefix(localPath, "/imports/"))
	default:
		handler.notFound(response, request, requestLocale)
	}
}

func (handler *Handler) index(response http.ResponseWriter, request *http.Request, requestLocale locale) {
	_, access, err := handler.userAccess(request)
	if err != nil {
		handler.internalError(response, request, requestLocale, err)
		return
	}
	page, err := handler.repository.ListEntries(request.Context(), webapp.EntryQuery{Limit: defaultPageSize})
	if err != nil {
		handler.internalError(response, request, requestLocale, err)
		return
	}
	model := newIndexPageModel(requestLocale, webapp.EntryFilter{}, page, false, access.FileUploadEnabled && access.CanWrite)
	render(response, request, http.StatusOK, indexPage(model))
}

func (handler *Handler) reportingSettings(response http.ResponseWriter, request *http.Request, requestLocale locale) {
	_, access, err := handler.userAccess(request)
	if err != nil {
		handler.internalError(response, request, requestLocale, err)
		return
	}
	detail, err := handler.repository.GetCurrentReportingConfiguration(request.Context())
	if errors.Is(err, webapp.ErrReportingNotConfigured) {
		model := newReportingSettingsPageModel(requestLocale, nil, access.CanWrite, "")
		render(response, request, http.StatusOK, reportingSettingsPage(model))
		return
	}
	if err != nil {
		handler.internalError(response, request, requestLocale, err)
		return
	}
	unclassified, err := handler.unclassifiedAccounts(request, detail)
	if err != nil {
		handler.internalError(response, request, requestLocale, err)
		return
	}
	model := newReportingSettingsPageModel(requestLocale, &detail, access.CanWrite, "")
	model.UnclassifiedAccounts = unclassified
	render(response, request, http.StatusOK, reportingSettingsPage(model))
}

func (handler *Handler) updateReportingSettings(response http.ResponseWriter, request *http.Request, requestLocale locale) {
	actorEmail, ok := handler.requireWriteActor(response, request, requestLocale)
	if !ok {
		return
	}
	input, ok := decodeReportingConfigurationForm(response, request)
	if !ok {
		handler.renderReportingSettingsError(response, request, requestLocale,
			webapp.ReportingConfigurationRequest{}, http.StatusBadRequest, messagesFor(requestLocale).ReportingInvalidFormMessage)
		return
	}
	if _, err := handler.repository.CreateReportingConfiguration(request.Context(), actorEmail, input); err != nil {
		status := http.StatusInternalServerError
		message := messagesFor(requestLocale).ReportingSaveFailedMessage
		switch {
		case errors.Is(err, webapp.ErrInvalidRequest):
			status = http.StatusBadRequest
			message = reportingConfigurationErrorMessage(messagesFor(requestLocale), err)
		case errors.Is(err, webapp.ErrConflict):
			status = http.StatusConflict
			message = messagesFor(requestLocale).ReportingConflictMessage
		case errors.Is(err, webapp.ErrWriteForbidden):
			handler.writeForbidden(response, request, requestLocale)
			return
		}
		if status == http.StatusInternalServerError {
			handler.internalError(response, request, requestLocale, err)
			return
		}
		handler.renderReportingSettingsError(response, request, requestLocale, input, status, message)
		return
	}
	http.Redirect(response, request, reportingSettingsHref(requestLocale), http.StatusSeeOther)
}

func (handler *Handler) renderReportingSettingsError(response http.ResponseWriter, request *http.Request, requestLocale locale,
	input webapp.ReportingConfigurationRequest, status int, message string,
) {
	detail := reportingConfigurationDetailFromRequest(input)
	model := newReportingSettingsPageModel(requestLocale, &detail, true, message)
	model.Configured = input.BaseRevision != nil && *input.BaseRevision > 0
	render(response, request, status, reportingSettingsPage(model))
}

func reportingConfigurationErrorMessage(msg messages, err error) string {
	var configurationErr *reporting.ConfigurationError
	if errors.As(err, &configurationErr) {
		switch configurationErr.Code {
		case reporting.ConfigurationInvalidStartMonth:
			return msg.ReportingInvalidStartMonthMessage
		case reporting.ConfigurationMissingFiscalYears:
			return msg.ReportingMissingFiscalYearsMessage
		case reporting.ConfigurationInvalidAccount, reporting.ConfigurationInvalidCategory:
			return msg.ReportingInvalidClassificationMessage
		case reporting.ConfigurationOverlappingAccounts:
			return msg.ReportingOverlappingClassificationsMessage
		case reporting.ConfigurationInvalidFiscalYear:
			return msg.ReportingInvalidFiscalYearMessage
		case reporting.ConfigurationNoncontiguousYears:
			return msg.ReportingNoncontiguousYearsMessage
		case reporting.ConfigurationInvalidOpeningMode, reporting.ConfigurationMissingOpeningEntries,
			reporting.ConfigurationInvalidOpeningEntries:
			return msg.ReportingInvalidOpeningSettingsMessage
		}
	}
	var openingErr *webapp.ReportingConfigurationError
	if errors.As(err, &openingErr) {
		switch openingErr.Code {
		case webapp.ReportingOpeningEntryNotApproved:
			return msg.ReportingOpeningNotApprovedMessage
		case webapp.ReportingOpeningEntryDateMismatch:
			return msg.ReportingOpeningDateMismatchMessage
		case webapp.ReportingOpeningEntryTemporaryAccount:
			return msg.ReportingOpeningTemporaryAccountMessage
		}
	}
	return msg.ReportingInvalidFormMessage
}

func (handler *Handler) unclassifiedAccounts(request *http.Request, detail webapp.ReportingConfigurationDetail) ([]string, error) {
	accounts := map[string]struct{}{}
	for _, year := range detail.FiscalYears {
		report, err := handler.repository.GetTrialBalance(request.Context(), reporting.Period{
			StartDate: year.StartDate, EndDate: year.EndDate,
		})
		if err != nil {
			return nil, err
		}
		for _, warning := range report.Warnings {
			if warning.Code == "unclassified_account" {
				accounts[warning.Account] = struct{}{}
			}
		}
	}
	result := make([]string, 0, len(accounts))
	for account := range accounts {
		result = append(result, account)
	}
	sort.Strings(result)
	return result, nil
}

func (handler *Handler) trialBalance(response http.ResponseWriter, request *http.Request, requestLocale locale) {
	detail, err := handler.repository.GetCurrentReportingConfiguration(request.Context())
	if errors.Is(err, webapp.ErrReportingNotConfigured) {
		render(response, request, http.StatusOK, trialBalancePage(trialBalancePageModel{
			Page: newPageContext(requestLocale, "/reports/trial-balance"), SetupHref: reportingSettingsHref(requestLocale),
		}))
		return
	}
	if err != nil {
		handler.internalError(response, request, requestLocale, err)
		return
	}
	model, err := newTrialBalancePageModel(requestLocale, detail)
	if err != nil {
		handler.internalError(response, request, requestLocale, err)
		return
	}
	period, explicit, ok := trialBalancePeriodQuery(request.URL.Query(), model.Selected)
	if !ok {
		model.FormError = messagesFor(requestLocale).TrialBalanceInvalidPeriodMessage
		render(response, request, http.StatusBadRequest, trialBalancePage(model))
		return
	}
	if explicit {
		model.Selected = period
	}
	report, err := handler.repository.GetTrialBalance(request.Context(), model.Selected)
	if errors.Is(err, reporting.ErrInvalidPeriod) {
		model.FormError = messagesFor(requestLocale).TrialBalanceInvalidPeriodMessage
		render(response, request, http.StatusBadRequest, trialBalancePage(model))
		return
	}
	if err != nil {
		handler.internalError(response, request, requestLocale, err)
		return
	}
	model.Report = &report
	render(response, request, http.StatusOK, trialBalancePage(model))
}

func (handler *Handler) selectTrialBalance(response http.ResponseWriter, request *http.Request, requestLocale locale) {
	form, ok := decodeStrictForm(response, request, maxSearchFormBody, map[string]bool{
		"period": false,
	})
	parts := strings.Split(form.Get("period"), "/")
	if !ok || len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		model := trialBalancePageModel{
			Page: newPageContext(requestLocale, "/reports/trial-balance"), SetupHref: reportingSettingsHref(requestLocale),
			FormError: messagesFor(requestLocale).TrialBalanceInvalidPeriodMessage,
		}
		render(response, request, http.StatusBadRequest, trialBalancePage(model))
		return
	}
	query := url.Values{"start_date": {parts[0]}, "end_date": {parts[1]}}
	http.Redirect(response, request, trialBalanceHref(requestLocale)+"?"+query.Encode(), http.StatusSeeOther)
}

func (handler *Handler) reportDrillDown(response http.ResponseWriter, request *http.Request, requestLocale locale, reportKind string) {
	form, ok := decodeURLEncodedForm(response, request, maxSearchFormBody)
	allowed := map[string]bool{
		"start_date": true, "end_date": true, "snapshot_identity": true, "commodity": true,
		"category": true, "account": true, "scope": true, "cursor": false,
	}
	if ok {
		for key := range form {
			if _, found := allowed[key]; !found {
				ok = false
			}
		}
		for key, required := range allowed {
			values, found := form[key]
			if required && (!found || len(values) != 1 || values[0] == "") {
				ok = false
			}
			if !required && found && (len(values) != 1 || values[0] == "") {
				ok = false
			}
		}
	}
	category := reporting.Category(form.Get("category"))
	scope := reporting.DrillDownScope(form.Get("scope"))
	if !ok || ledger.ValidateAccount(form.Get("account")) != nil ||
		(category != reporting.CategoryAsset && category != reporting.CategoryLiability &&
			category != reporting.CategoryEquity && category != reporting.CategoryRevenue &&
			category != reporting.CategoryExpense && category != reporting.CategoryUnknown) ||
		(scope != reporting.DrillDownDirect && scope != reporting.DrillDownSubtree) {
		handler.reportDrillDownError(response, request, requestLocale, reportKind, http.StatusBadRequest, false, reporting.Period{})
		return
	}
	period := reporting.Period{StartDate: form.Get("start_date"), EndDate: form.Get("end_date")}
	query := webapp.ReportDrillDownQuery{
		DrillDown: reporting.DrillDownQuery{
			Period: period, Commodity: form.Get("commodity"), Category: category,
			Account: form.Get("account"), Scope: scope,
		},
		SnapshotIdentity: form.Get("snapshot_identity"), Limit: defaultPageSize, Cursor: form.Get("cursor"),
	}
	if reportKind == "trial-balance" {
		detail, err := handler.repository.GetTrialBalanceDrillDown(request.Context(), query)
		if err != nil {
			handler.handleReportDrillDownError(response, request, requestLocale, reportKind, period, err)
			return
		}
		render(response, request, http.StatusOK, reportDrillDownPage(trialBalanceDrillDownPageModel(requestLocale, detail)))
		return
	}
	detail, err := handler.repository.GetIncomeStatementDrillDown(request.Context(), query)
	if err != nil {
		handler.handleReportDrillDownError(response, request, requestLocale, reportKind, period, err)
		return
	}
	render(response, request, http.StatusOK, reportDrillDownPage(incomeStatementDrillDownPageModel(requestLocale, detail)))
}

func (handler *Handler) handleReportDrillDownError(response http.ResponseWriter, request *http.Request, requestLocale locale, reportKind string, period reporting.Period, err error) {
	switch {
	case errors.Is(err, webapp.ErrReportSnapshotChanged):
		handler.reportDrillDownError(response, request, requestLocale, reportKind, http.StatusConflict, true, period)
	case errors.Is(err, reporting.ErrInvalidDrillDown), errors.Is(err, reporting.ErrInvalidPeriod), errors.Is(err, webapp.ErrInvalidRequest):
		handler.reportDrillDownError(response, request, requestLocale, reportKind, http.StatusBadRequest, false, reporting.Period{})
	default:
		handler.internalError(response, request, requestLocale, err)
	}
}

func (handler *Handler) reportDrillDownError(response http.ResponseWriter, request *http.Request, requestLocale locale, reportKind string, status int, stale bool, period reporting.Period) {
	msg := messagesFor(requestLocale)
	title, message := msg.DrillDownInvalidTitle, msg.DrillDownInvalidMessage
	if stale {
		title, message = msg.DrillDownStaleTitle, msg.DrillDownStaleMessage
	}
	model := errorPageModel{
		Page: newPageContext(requestLocale, reportPath(reportKind)), Status: status, Title: title, Message: message,
	}
	if stale {
		model.Page.HomeHref = reportBackHref(requestLocale, reportKind, period)
		model.Page.Messages.BackToEntries = msg.DrillDownBackToReport
	}
	render(response, request, status, errorPage(model))
}

func trialBalanceDrillDownPageModel(requestLocale locale, detail webapp.TrialBalanceDrillDownDetail) reportDrillDownPageModel {
	amounts := detail.Amounts
	model := reportDrillDownPageModel{
		Page: newPageContext(requestLocale, "/reports/trial-balance"), ReportName: messagesFor(requestLocale).TrialBalanceTitle,
		BackHref: reportBackHref(requestLocale, "trial-balance", detail.Period.Period), FormAction: trialBalanceDrillDownHref(requestLocale),
		ConfigurationRevision: detail.ConfigurationRevision, Period: detail.Period.Period,
		SnapshotIdentity: detail.SnapshotIdentity, Commodity: detail.Commodity, Category: detail.Category,
		Account: detail.Account, Scope: detail.Scope, TotalEntries: detail.TotalEntries,
		NextCursor: detail.NextCursor, TrialAmounts: &amounts, Entries: []reportDrillDownEntryModel{},
	}
	for _, item := range detail.Entries {
		itemAmounts := item.Amounts
		model.Entries = append(model.Entries, reportDrillDownEntryModel{
			Href: entryHref(requestLocale, item.ID), ID: item.ID, OccurredAt: item.OccurredAt,
			Description: item.Description, Role: item.Role, Contributions: item.Contributions, TrialAmounts: &itemAmounts,
		})
	}
	return model
}

func incomeStatementDrillDownPageModel(requestLocale locale, detail webapp.IncomeStatementDrillDownDetail) reportDrillDownPageModel {
	balance := detail.Balance
	model := reportDrillDownPageModel{
		Page: newPageContext(requestLocale, "/reports/income-statement"), ReportName: messagesFor(requestLocale).IncomeStatementTitle,
		BackHref: reportBackHref(requestLocale, "income-statement", detail.Period.Period), FormAction: incomeStatementDrillDownHref(requestLocale),
		ConfigurationRevision: detail.ConfigurationRevision, Period: detail.Period.Period,
		SnapshotIdentity: detail.SnapshotIdentity, Commodity: detail.Commodity, Category: detail.Category,
		Account: detail.Account, Scope: detail.Scope, TotalEntries: detail.TotalEntries,
		NextCursor: detail.NextCursor, Balance: &balance, Entries: []reportDrillDownEntryModel{},
	}
	for _, item := range detail.Entries {
		itemBalance := item.Balance
		model.Entries = append(model.Entries, reportDrillDownEntryModel{
			Href: entryHref(requestLocale, item.ID), ID: item.ID, OccurredAt: item.OccurredAt,
			Description: item.Description, Contributions: item.Contributions, Balance: &itemBalance,
		})
	}
	return model
}

func reportPath(reportKind string) string {
	if reportKind == "trial-balance" {
		return "/reports/trial-balance"
	}
	return "/reports/income-statement"
}

func reportBackHref(requestLocale locale, reportKind string, period reporting.Period) string {
	query := url.Values{"start_date": {period.StartDate}, "end_date": {period.EndDate}}
	if reportKind == "trial-balance" {
		return trialBalanceHref(requestLocale) + "?" + query.Encode()
	}
	return incomeStatementHref(requestLocale) + "?" + query.Encode()
}

func (handler *Handler) currentOverview(response http.ResponseWriter, request *http.Request, requestLocale locale) {
	addVary(response, "HX-Request")
	addVary(response, "HX-Target")
	defaultAsOf := handler.now().In(time.FixedZone("JST", 9*60*60)).Format(time.DateOnly)
	detail, err := handler.repository.GetCurrentReportingConfiguration(request.Context())
	if errors.Is(err, webapp.ErrReportingNotConfigured) {
		handler.renderCurrentOverview(response, request, http.StatusOK, currentOverviewPageModel{
			Page: newPageContext(requestLocale, "/reports/current"), SetupHref: reportingSettingsHref(requestLocale), AsOf: defaultAsOf,
		})
		return
	}
	if err != nil {
		handler.currentOverviewInternalError(response, request, requestLocale, err)
		return
	}
	model, err := newCurrentOverviewPageModel(requestLocale, detail, defaultAsOf)
	if err != nil {
		handler.currentOverviewInternalError(response, request, requestLocale, err)
		return
	}
	asOf, expensePeriod, ok := currentOverviewQuery(request.URL.Query(), model.AsOf, model.Selected)
	if !ok {
		model.FormError = messagesFor(requestLocale).CurrentOverviewInvalidDate
		handler.renderCurrentOverview(response, request, http.StatusBadRequest, model)
		return
	}
	report, err := handler.repository.GetCurrentOverview(request.Context(), asOf, expensePeriod)
	if errors.Is(err, reporting.ErrInvalidPeriod) {
		model.FormError = messagesFor(requestLocale).CurrentOverviewInvalidDate
		handler.renderCurrentOverview(response, request, http.StatusBadRequest, model)
		return
	}
	if err != nil {
		handler.currentOverviewInternalError(response, request, requestLocale, err)
		return
	}
	model.AsOf = asOf
	model.Selected = expensePeriod
	model.Report = &report
	if _, ok := currentOverviewPartialTarget(request); ok {
		response.Header().Set("HX-Push-Url", currentOverviewLocation(requestLocale, asOf, expensePeriod))
	}
	handler.renderCurrentOverview(response, request, http.StatusOK, model)
}

func (handler *Handler) selectCurrentOverview(response http.ResponseWriter, request *http.Request, requestLocale locale) {
	form, ok := decodeStrictForm(response, request, maxSearchFormBody, map[string]bool{"as_of": false, "expense_period": false})
	parts := strings.Split(form.Get("expense_period"), "/")
	if !ok || form.Get("as_of") == "" || len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		request.URL.RawQuery = "invalid_date=1"
		handler.currentOverview(response, request, requestLocale)
		return
	}
	query := url.Values{
		"as_of": {form.Get("as_of")}, "expense_start_date": {parts[0]}, "expense_end_date": {parts[1]},
	}
	if _, ok := currentOverviewPartialTarget(request); ok {
		request.URL.RawQuery = query.Encode()
		handler.currentOverview(response, request, requestLocale)
		return
	}
	http.Redirect(response, request, currentOverviewHref(requestLocale)+"?"+query.Encode(), http.StatusSeeOther)
}

func (handler *Handler) renderCurrentOverview(response http.ResponseWriter, request *http.Request, status int, model currentOverviewPageModel) {
	target, partial := currentOverviewPartialTarget(request)
	if partial {
		switch target {
		case currentBalancesResultID:
			render(response, request, status, currentBalanceUpdate(model))
		case currentExpensesResultID:
			render(response, request, status, currentExpenseUpdate(model))
		}
		return
	}
	render(response, request, status, currentOverviewPage(model))
}

func (handler *Handler) currentOverviewInternalError(response http.ResponseWriter, request *http.Request, requestLocale locale, err error) {
	target, partial := currentOverviewPartialTarget(request)
	if !partial {
		handler.internalError(response, request, requestLocale, err)
		return
	}
	detail := ""
	if handler.development {
		detail = err.Error()
	}
	msg := messagesFor(requestLocale)
	render(response, request, http.StatusInternalServerError, currentOverviewResultError(target, msg, detail))
}

func currentOverviewPartialTarget(request *http.Request) (string, bool) {
	if !isHXRequest(request) {
		return "", false
	}
	target := request.Header.Get("HX-Target")
	return target, target == currentBalancesResultID || target == currentExpensesResultID
}

func currentOverviewLocation(requestLocale locale, asOf string, expensePeriod reporting.Period) string {
	query := url.Values{
		"as_of": {asOf}, "expense_start_date": {expensePeriod.StartDate}, "expense_end_date": {expensePeriod.EndDate},
	}
	return currentOverviewHref(requestLocale) + "?" + query.Encode()
}

func currentOverviewQuery(query url.Values, fallbackAsOf string, fallbackExpense reporting.Period) (string, reporting.Period, bool) {
	if len(query) == 0 {
		return fallbackAsOf, fallbackExpense, true
	}
	if len(query) != 3 || len(query["as_of"]) != 1 || len(query["expense_start_date"]) != 1 ||
		len(query["expense_end_date"]) != 1 || query.Get("as_of") == "" || query.Get("expense_start_date") == "" ||
		query.Get("expense_end_date") == "" {
		return fallbackAsOf, fallbackExpense, false
	}
	asOf := query.Get("as_of")
	parsed, err := time.Parse(time.DateOnly, asOf)
	if err != nil || parsed.Format(time.DateOnly) != asOf {
		return fallbackAsOf, fallbackExpense, false
	}
	expensePeriod := reporting.Period{StartDate: query.Get("expense_start_date"), EndDate: query.Get("expense_end_date")}
	start, startErr := time.Parse(time.DateOnly, expensePeriod.StartDate)
	end, endErr := time.Parse(time.DateOnly, expensePeriod.EndDate)
	if startErr != nil || endErr != nil || start.Format(time.DateOnly) != expensePeriod.StartDate ||
		end.Format(time.DateOnly) != expensePeriod.EndDate {
		return fallbackAsOf, fallbackExpense, false
	}
	return asOf, expensePeriod, true
}

func (handler *Handler) balanceSheet(response http.ResponseWriter, request *http.Request, requestLocale locale) {
	detail, err := handler.repository.GetCurrentReportingConfiguration(request.Context())
	if errors.Is(err, webapp.ErrReportingNotConfigured) {
		render(response, request, http.StatusOK, balanceSheetPage(balanceSheetPageModel{
			Page: newPageContext(requestLocale, "/reports/balance-sheet"), SetupHref: reportingSettingsHref(requestLocale),
		}))
		return
	}
	if err != nil {
		handler.internalError(response, request, requestLocale, err)
		return
	}
	model, err := newBalanceSheetPageModel(requestLocale, detail)
	if err != nil {
		handler.internalError(response, request, requestLocale, err)
		return
	}
	period, explicit, ok := trialBalancePeriodQuery(request.URL.Query(), model.Selected)
	if !ok {
		model.FormError = messagesFor(requestLocale).StatementInvalidPeriodMessage
		render(response, request, http.StatusBadRequest, balanceSheetPage(model))
		return
	}
	if explicit {
		model.Selected = period
	}
	report, err := handler.repository.GetBalanceSheet(request.Context(), model.Selected)
	if errors.Is(err, reporting.ErrInvalidPeriod) {
		model.FormError = messagesFor(requestLocale).StatementInvalidPeriodMessage
		render(response, request, http.StatusBadRequest, balanceSheetPage(model))
		return
	}
	if errors.Is(err, reporting.ErrOpeningUnbalanced) {
		model.FormError = messagesFor(requestLocale).StatementOpeningUnbalancedMessage
		render(response, request, http.StatusUnprocessableEntity, balanceSheetPage(model))
		return
	}
	if err != nil {
		handler.internalError(response, request, requestLocale, err)
		return
	}
	model.Report = &report
	render(response, request, http.StatusOK, balanceSheetPage(model))
}

func (handler *Handler) closingBalanceSheet(response http.ResponseWriter, request *http.Request, requestLocale locale) {
	detail, err := handler.repository.GetCurrentReportingConfiguration(request.Context())
	if errors.Is(err, webapp.ErrReportingNotConfigured) {
		render(response, request, http.StatusOK, closingBalanceSheetPage(closingBalanceSheetPageModel{
			Page: newPageContext(requestLocale, "/reports/closing-balance-sheet"), SetupHref: reportingSettingsHref(requestLocale),
		}))
		return
	}
	if err != nil {
		handler.internalError(response, request, requestLocale, err)
		return
	}
	model, err := newClosingBalanceSheetPageModel(requestLocale, detail)
	if err != nil {
		handler.internalError(response, request, requestLocale, err)
		return
	}
	period, explicit, ok := trialBalancePeriodQuery(request.URL.Query(), model.Selected)
	if !ok {
		model.FormError = messagesFor(requestLocale).StatementInvalidPeriodMessage
		render(response, request, http.StatusBadRequest, closingBalanceSheetPage(model))
		return
	}
	if explicit {
		model.Selected = period
	}
	report, err := handler.repository.GetClosingBalanceSheet(request.Context(), model.Selected)
	if errors.Is(err, reporting.ErrInvalidPeriod) {
		model.FormError = messagesFor(requestLocale).StatementInvalidPeriodMessage
		render(response, request, http.StatusBadRequest, closingBalanceSheetPage(model))
		return
	}
	if errors.Is(err, reporting.ErrOpeningUnbalanced) {
		model.FormError = messagesFor(requestLocale).ClosingBalanceSheetOpeningUnbalanced
		render(response, request, http.StatusUnprocessableEntity, closingBalanceSheetPage(model))
		return
	}
	if errors.Is(err, reporting.ErrClosingUnbalanced) {
		model.FormError = messagesFor(requestLocale).StatementClosingUnbalancedMessage
		render(response, request, http.StatusUnprocessableEntity, closingBalanceSheetPage(model))
		return
	}
	if err != nil {
		handler.internalError(response, request, requestLocale, err)
		return
	}
	model.Report = &report
	render(response, request, http.StatusOK, closingBalanceSheetPage(model))
}

func (handler *Handler) incomeStatement(response http.ResponseWriter, request *http.Request, requestLocale locale) {
	detail, err := handler.repository.GetCurrentReportingConfiguration(request.Context())
	if errors.Is(err, webapp.ErrReportingNotConfigured) {
		render(response, request, http.StatusOK, incomeStatementPage(incomeStatementPageModel{
			Page: newPageContext(requestLocale, "/reports/income-statement"), SetupHref: reportingSettingsHref(requestLocale),
		}))
		return
	}
	if err != nil {
		handler.internalError(response, request, requestLocale, err)
		return
	}
	model, err := newIncomeStatementPageModel(requestLocale, detail)
	if err != nil {
		handler.internalError(response, request, requestLocale, err)
		return
	}
	period, explicit, ok := trialBalancePeriodQuery(request.URL.Query(), model.Selected)
	if !ok {
		model.FormError = messagesFor(requestLocale).StatementInvalidPeriodMessage
		render(response, request, http.StatusBadRequest, incomeStatementPage(model))
		return
	}
	if explicit {
		model.Selected = period
	}
	report, err := handler.repository.GetIncomeStatement(request.Context(), model.Selected)
	if errors.Is(err, reporting.ErrInvalidPeriod) {
		model.FormError = messagesFor(requestLocale).StatementInvalidPeriodMessage
		render(response, request, http.StatusBadRequest, incomeStatementPage(model))
		return
	}
	if err != nil {
		handler.internalError(response, request, requestLocale, err)
		return
	}
	model.Report = &report
	render(response, request, http.StatusOK, incomeStatementPage(model))
}

func (handler *Handler) balanceTrend(response http.ResponseWriter, request *http.Request, requestLocale locale) {
	detail, err := handler.repository.GetCurrentReportingConfiguration(request.Context())
	if errors.Is(err, webapp.ErrReportingNotConfigured) {
		render(response, request, http.StatusOK, balanceTrendPage(balanceTrendPageModel{
			Page: newPageContext(requestLocale, "/reports/balance-trend"), SetupHref: reportingSettingsHref(requestLocale),
		}))
		return
	}
	if err != nil {
		handler.internalError(response, request, requestLocale, err)
		return
	}
	model, err := newBalanceTrendPageModel(requestLocale, detail)
	if err != nil {
		handler.internalError(response, request, requestLocale, err)
		return
	}
	period, explicit, ok := trialBalancePeriodQuery(request.URL.Query(), model.Selected)
	if !ok {
		model.FormError = messagesFor(requestLocale).StatementInvalidPeriodMessage
		render(response, request, http.StatusBadRequest, balanceTrendPage(model))
		return
	}
	if explicit {
		model.Selected = period
	}
	report, err := handler.repository.GetBalanceTrend(request.Context(), model.Selected)
	if errors.Is(err, reporting.ErrInvalidPeriod) {
		model.FormError = messagesFor(requestLocale).StatementInvalidPeriodMessage
		render(response, request, http.StatusBadRequest, balanceTrendPage(model))
		return
	}
	if errors.Is(err, reporting.ErrOpeningUnbalanced) {
		model.FormError = messagesFor(requestLocale).StatementOpeningUnbalancedMessage
		render(response, request, http.StatusUnprocessableEntity, balanceTrendPage(model))
		return
	}
	if err != nil {
		handler.internalError(response, request, requestLocale, err)
		return
	}
	model.Report = &report
	render(response, request, http.StatusOK, balanceTrendPage(model))
}

func (handler *Handler) selectStatementPeriod(response http.ResponseWriter, request *http.Request, requestLocale locale, target string) {
	form, ok := decodeStrictForm(response, request, maxSearchFormBody, map[string]bool{"period": false})
	parts := strings.Split(form.Get("period"), "/")
	if !ok || len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		request.URL.RawQuery = "invalid_period=1"
		switch target {
		case balanceSheetHref(requestLocale):
			handler.balanceSheet(response, request, requestLocale)
		case closingBalanceSheetHref(requestLocale):
			handler.closingBalanceSheet(response, request, requestLocale)
		case incomeStatementHref(requestLocale):
			handler.incomeStatement(response, request, requestLocale)
		default:
			handler.balanceTrend(response, request, requestLocale)
		}
		return
	}
	query := url.Values{"start_date": {parts[0]}, "end_date": {parts[1]}}
	http.Redirect(response, request, target+"?"+query.Encode(), http.StatusSeeOther)
}

func (handler *Handler) searchEntries(response http.ResponseWriter, request *http.Request, requestLocale locale) {
	addVary(response, "HX-Request")
	filter, cursor, ok := decodeSearchForm(response, request)
	if !ok {
		handler.invalidSearch(response, request, requestLocale)
		return
	}
	page, err := handler.repository.ListEntries(request.Context(), webapp.EntryQuery{
		Filter: filter, Limit: defaultPageSize, Cursor: cursor,
	})
	if errors.Is(err, webapp.ErrInvalidRequest) {
		handler.invalidSearch(response, request, requestLocale)
		return
	}
	if err != nil {
		handler.internalError(response, request, requestLocale)
		return
	}
	model := newIndexPageModel(requestLocale, filter, page, true, false)
	if isHXRequest(request) {
		render(response, request, http.StatusOK, entryResults(model))
		return
	}
	_, access, err := handler.userAccess(request)
	if err != nil {
		handler.internalError(response, request, requestLocale, err)
		return
	}
	model.UploadEnabled = access.FileUploadEnabled && access.CanWrite
	render(response, request, http.StatusOK, indexPage(model))
}

func (handler *Handler) importRecords(response http.ResponseWriter, request *http.Request, requestLocale locale) {
	actorEmail, ok := handler.uploadActor(response, request, requestLocale)
	if !ok {
		return
	}
	body, uploadStatus, ok := decodeImportUpload(response, request)
	if !ok {
		handler.invalidUpload(response, request, requestLocale, uploadStatus)
		return
	}
	result, err := handler.repository.Import(request.Context(), actorEmail, body)
	if errors.Is(err, webapp.ErrUploadDisabled) {
		handler.uploadDisabled(response, request, requestLocale)
		return
	}
	if errors.Is(err, webapp.ErrUploadForbidden) {
		handler.uploadForbidden(response, request, requestLocale)
		return
	}
	if errors.Is(err, ingest.ErrInvalidInput) || errors.Is(err, webapp.ErrInvalidRequest) {
		handler.invalidUpload(response, request, requestLocale, http.StatusBadRequest)
		return
	}
	if err != nil {
		handler.uploadFailed(response, request, requestLocale)
		return
	}
	http.Redirect(response, request, runHref(requestLocale, result.RunIdentity), http.StatusSeeOther)
}

func (handler *Handler) importTackler(response http.ResponseWriter, request *http.Request, requestLocale locale) {
	actorEmail, ok := handler.uploadActor(response, request, requestLocale)
	if !ok {
		return
	}
	body, uploadStatus, ok := decodeImportUpload(response, request)
	if !ok {
		handler.invalidUpload(response, request, requestLocale, uploadStatus)
		return
	}
	entries, err := tacklerfmt.Parse(body)
	if err != nil {
		handler.invalidTacklerUpload(response, request, requestLocale, err)
		return
	}
	input, err := normalizedInputForTacklerEntries(entries)
	if err != nil {
		handler.invalidTacklerUpload(response, request, requestLocale, err)
		return
	}
	result, err := handler.repository.Import(request.Context(), actorEmail, input)
	if errors.Is(err, webapp.ErrUploadDisabled) {
		handler.uploadDisabled(response, request, requestLocale)
		return
	}
	if errors.Is(err, webapp.ErrUploadForbidden) {
		handler.uploadForbidden(response, request, requestLocale)
		return
	}
	if errors.Is(err, ingest.ErrInvalidInput) || errors.Is(err, webapp.ErrInvalidRequest) {
		handler.invalidUpload(response, request, requestLocale, http.StatusBadRequest)
		return
	}
	if err != nil {
		handler.uploadFailed(response, request, requestLocale)
		return
	}
	http.Redirect(response, request, runHref(requestLocale, result.RunIdentity), http.StatusSeeOther)
}

func (handler *Handler) exportEntries(response http.ResponseWriter, request *http.Request, requestLocale locale, format string) {
	if format != "tackler" && format != "json" {
		handler.notFound(response, request, requestLocale)
		return
	}
	filter, ok := decodeExportForm(response, request)
	if !ok {
		handler.invalidSearch(response, request, requestLocale)
		return
	}
	approved, err := handler.repository.ListApprovedEntries(request.Context(), filter)
	if errors.Is(err, webapp.ErrInvalidRequest) {
		handler.invalidSearch(response, request, requestLocale)
		return
	}
	if err != nil {
		handler.internalError(response, request, requestLocale)
		return
	}
	if format == "tackler" {
		handler.exportTackler(response, approved)
		return
	}
	handler.exportJSON(response, approved)
}

func (handler *Handler) exportTackler(response http.ResponseWriter, approved []webapp.ApprovedEntry) {
	entries := make([]ledger.JournalEntry, 0, len(approved))
	for _, item := range approved {
		entries = append(entries, item.Entry)
	}
	output, err := tacklerfmt.Export(entries, tacklerfmt.Options{OmittedAmounts: tacklerfmt.PreserveOmitted})
	if err != nil {
		response.WriteHeader(http.StatusInternalServerError)
		return
	}
	response.Header().Set("Content-Type", "text/plain; charset=utf-8")
	response.Header().Set("Content-Disposition", `attachment; filename="bokiccio-export.txn"`)
	response.WriteHeader(http.StatusOK)
	_, _ = response.Write(output)
}

func (handler *Handler) exportJSON(response http.ResponseWriter, approved []webapp.ApprovedEntry) {
	exported := webapp.JSONExport{SchemaVersion: webapp.APISchemaVersion, Entries: []webapp.ExportEntry{}}
	for _, item := range approved {
		entry := webapp.ExportEntry{
			ID: item.ID, Revision: item.Revision, ApprovedAt: item.ApprovedAt, Source: item.Source,
			OccurredAt: item.Entry.Date.String(), Description: item.Entry.Description,
			Comments: append([]string(nil), item.Entry.Comments...), Postings: []webapp.PostingDetail{},
		}
		for _, posting := range item.Entry.Postings {
			detail := webapp.PostingDetail{Account: posting.Account, Comment: posting.Comment}
			if posting.Amount != nil {
				amount := posting.Amount.Value.String()
				detail.Amount = &amount
				detail.Commodity = string(posting.Amount.Commodity)
			}
			if posting.TotalPrice != nil {
				detail.TotalPrice = &webapp.AmountDetail{
					Amount: posting.TotalPrice.Value.String(), Commodity: string(posting.TotalPrice.Commodity),
				}
			}
			entry.Postings = append(entry.Postings, detail)
		}
		exported.Entries = append(exported.Entries, entry)
	}
	response.Header().Set("Content-Type", "application/json; charset=utf-8")
	response.Header().Set("Content-Disposition", `attachment; filename="bokiccio-export.json"`)
	response.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(response).Encode(exported)
}

func (handler *Handler) entry(response http.ResponseWriter, request *http.Request, requestLocale locale, escapedID string) {
	id, ok := pathID(escapedID)
	if !ok {
		handler.notFound(response, request, requestLocale)
		return
	}
	detail, err := handler.repository.GetEntry(request.Context(), id)
	if errors.Is(err, webapp.ErrNotFound) {
		handler.notFound(response, request, requestLocale)
		return
	}
	if err != nil {
		handler.internalError(response, request, requestLocale)
		return
	}
	_, access, err := handler.userAccess(request)
	if err != nil {
		handler.internalError(response, request, requestLocale, err)
		return
	}
	render(response, request, http.StatusOK, entryPage(newEntryPageModel(requestLocale, id, detail, currentCandidate(detail), access.CanWrite, "")))
}

func (handler *Handler) entryMutation(response http.ResponseWriter, request *http.Request, requestLocale locale, path string) {
	parts := strings.Split(path, "/")
	if len(parts) != 2 {
		handler.notFound(response, request, requestLocale)
		return
	}
	id, ok := pathID(parts[0])
	if !ok {
		handler.notFound(response, request, requestLocale)
		return
	}
	switch parts[1] {
	case "revisions":
		handler.createRevision(response, request, requestLocale, id)
	case "approvals":
		handler.approveRevision(response, request, requestLocale, id)
	default:
		handler.notFound(response, request, requestLocale)
	}
}

func (handler *Handler) createRevision(response http.ResponseWriter, request *http.Request, requestLocale locale, id string) {
	actorEmail, ok := handler.requireWriteActor(response, request, requestLocale)
	if !ok {
		return
	}
	input, ok := decodeRevisionForm(response, request)
	if !ok {
		handler.renderEntryFormError(response, request, requestLocale, id, http.StatusBadRequest, messagesFor(requestLocale).InvalidRevisionFormMessage)
		return
	}
	if _, err := handler.repository.CreateRevision(request.Context(), actorEmail, id, input); err != nil {
		handler.renderEntryMutationError(response, request, requestLocale, id, err, mutationRevision)
		return
	}
	http.Redirect(response, request, entryHref(requestLocale, id), http.StatusSeeOther)
}

func (handler *Handler) approveRevision(response http.ResponseWriter, request *http.Request, requestLocale locale, id string) {
	actorEmail, ok := handler.requireWriteActor(response, request, requestLocale)
	if !ok {
		return
	}
	input, ok := decodeApprovalForm(response, request)
	if !ok {
		handler.renderEntryFormError(response, request, requestLocale, id, http.StatusBadRequest, messagesFor(requestLocale).InvalidApprovalFormMessage)
		return
	}
	if _, err := handler.repository.ApproveRevision(request.Context(), actorEmail, id, input); err != nil {
		handler.renderEntryMutationError(response, request, requestLocale, id, err, mutationApproval)
		return
	}
	http.Redirect(response, request, entryHref(requestLocale, id), http.StatusSeeOther)
}

type mutationKind int

const (
	mutationRevision mutationKind = iota
	mutationApproval
)

func (handler *Handler) renderEntryMutationError(response http.ResponseWriter, request *http.Request, requestLocale locale, id string, err error, kind mutationKind) {
	msg := messagesFor(requestLocale)
	status := http.StatusInternalServerError
	message := msg.RevisionFailedMessage
	if kind == mutationApproval {
		message = msg.ApprovalFailedMessage
	}
	switch {
	case errors.Is(err, webapp.ErrWriteForbidden):
		handler.writeForbidden(response, request, requestLocale)
		return
	case errors.Is(err, webapp.ErrNotFound):
		handler.notFound(response, request, requestLocale)
		return
	case errors.Is(err, webapp.ErrConflict):
		status = http.StatusConflict
		message = msg.RevisionConflictMessage
		if kind == mutationApproval {
			message = msg.ApprovalConflictMessage
		}
	case errors.Is(err, webapp.ErrInvalidRequest):
		status = http.StatusBadRequest
		message = msg.InvalidRevisionFormMessage
		if kind == mutationApproval {
			message = msg.InvalidApprovalFormMessage
		}
	case errors.Is(err, webapp.ErrInvalidRevision):
		status = http.StatusUnprocessableEntity
		message = msg.ApprovalInvalidRevisionMessage
	}
	handler.renderEntryFormError(response, request, requestLocale, id, status, message)
}

func (handler *Handler) renderEntryFormError(response http.ResponseWriter, request *http.Request, requestLocale locale, id string, status int, message string) {
	detail, err := handler.repository.GetEntry(request.Context(), id)
	if errors.Is(err, webapp.ErrNotFound) {
		handler.notFound(response, request, requestLocale)
		return
	}
	if err != nil {
		handler.internalError(response, request, requestLocale)
		return
	}
	current := currentCandidate(detail)
	render(response, request, status, entryPage(newEntryPageModel(requestLocale, id, detail, current, true, message)))
}

func (handler *Handler) run(response http.ResponseWriter, request *http.Request, requestLocale locale, escapedID string) {
	id, ok := pathID(escapedID)
	if !ok {
		handler.notFound(response, request, requestLocale)
		return
	}
	detail, err := handler.repository.GetRun(request.Context(), id)
	if errors.Is(err, webapp.ErrNotFound) {
		handler.notFound(response, request, requestLocale)
		return
	}
	if err != nil {
		handler.internalError(response, request, requestLocale)
		return
	}
	model := runPageModel{
		Page:     newPageContext(requestLocale, "/imports/"+url.PathEscape(id)),
		Detail:   detail,
		Outcomes: make([]outcomePageModel, 0, len(detail.Outcomes)),
	}
	for _, outcome := range detail.Outcomes {
		outcomeEntryHref := ""
		if outcome.EntryID != "" {
			outcomeEntryHref = entryHref(requestLocale, outcome.EntryID)
		}
		model.Outcomes = append(model.Outcomes, outcomePageModel{Detail: outcome, EntryHref: outcomeEntryHref})
	}
	render(response, request, http.StatusOK, runPage(model))
}

func (handler *Handler) asset(response http.ResponseWriter, request *http.Request, name, contentType string) {
	if request.Method != http.MethodGet && request.Method != http.MethodHead {
		handler.methodNotAllowed(response, request, localeJA, request.URL.Path, "GET, HEAD")
		return
	}
	data, err := assetFiles.ReadFile(name)
	if err != nil {
		handler.notFound(response, request, localeJA)
		return
	}
	response.Header().Set("Content-Type", contentType)
	response.Header().Set("Content-Length", strconv.Itoa(len(data)))
	response.WriteHeader(http.StatusOK)
	if request.Method != http.MethodHead {
		_, _ = response.Write(data)
	}
}

func (handler *Handler) methodNotAllowed(response http.ResponseWriter, request *http.Request, requestLocale locale, localPath, allow string) {
	msg := messagesFor(requestLocale)
	response.Header().Set("Allow", allow)
	render(response, request, http.StatusMethodNotAllowed, errorPage(errorPageModel{
		Page: newPageContext(requestLocale, localPath), Status: http.StatusMethodNotAllowed,
		Title: msg.MethodNotAllowedTitle, Message: msg.MethodNotAllowedMessage,
	}))
}

func (handler *Handler) notFound(response http.ResponseWriter, request *http.Request, requestLocale locale) {
	msg := messagesFor(requestLocale)
	render(response, request, http.StatusNotFound, errorPage(errorPageModel{
		Page: newPageContext(requestLocale, "/"), Status: http.StatusNotFound,
		Title: msg.NotFoundTitle, Message: msg.NotFoundMessage,
	}))
}

func (handler *Handler) internalError(response http.ResponseWriter, request *http.Request, requestLocale locale, causes ...error) {
	msg := messagesFor(requestLocale)
	detail := ""
	if handler.development && len(causes) > 0 && causes[0] != nil {
		detail = causes[0].Error()
	}
	render(response, request, http.StatusInternalServerError, errorPage(errorPageModel{
		Page: newPageContext(requestLocale, "/"), Status: http.StatusInternalServerError,
		Title: msg.InternalErrorTitle, Message: msg.InternalErrorMessage, Detail: detail,
	}))
}

func (handler *Handler) invalidSearch(response http.ResponseWriter, request *http.Request, requestLocale locale) {
	msg := messagesFor(requestLocale)
	render(response, request, http.StatusBadRequest, errorPage(errorPageModel{
		Page: newPageContext(requestLocale, "/"), Status: http.StatusBadRequest,
		Title: msg.InvalidSearchTitle, Message: msg.InvalidSearchMessage,
	}))
}

func (handler *Handler) invalidUpload(response http.ResponseWriter, request *http.Request, requestLocale locale, status int) {
	msg := messagesFor(requestLocale)
	title := msg.InvalidUploadTitle
	message := msg.InvalidUploadMessage
	switch status {
	case http.StatusRequestEntityTooLarge:
		title = msg.UploadTooLargeTitle
		message = msg.UploadTooLargeMessage
	case http.StatusUnsupportedMediaType:
		title = msg.UnsupportedUploadTitle
		message = msg.UnsupportedUploadMessage
	}
	render(response, request, status, errorPage(errorPageModel{
		Page: newPageContext(requestLocale, "/"), Status: status,
		Title: title, Message: message,
	}))
}

func (handler *Handler) invalidTacklerUpload(response http.ResponseWriter, request *http.Request, requestLocale locale, err error) {
	msg := messagesFor(requestLocale)
	diagnostic := tacklerfmt.DetailedDiagnostic(err)
	log.Printf("webui: tackler import rejected: %s", diagnostic)
	render(response, request, http.StatusBadRequest, errorPage(errorPageModel{
		Page: newPageContext(requestLocale, "/"), Status: http.StatusBadRequest,
		Title: msg.InvalidTacklerUploadTitle, Message: msg.InvalidTacklerUploadMessage + " " + diagnostic,
	}))
}

func (handler *Handler) uploadFailed(response http.ResponseWriter, request *http.Request, requestLocale locale) {
	msg := messagesFor(requestLocale)
	render(response, request, http.StatusInternalServerError, errorPage(errorPageModel{
		Page: newPageContext(requestLocale, "/"), Status: http.StatusInternalServerError,
		Title: msg.UploadFailedTitle, Message: msg.UploadFailedMessage,
	}))
}

func (handler *Handler) uploadActor(response http.ResponseWriter, request *http.Request, requestLocale locale) (string, bool) {
	actorEmail, access, err := handler.userAccess(request)
	if err != nil {
		handler.internalError(response, request, requestLocale, err)
		return "", false
	}
	if !access.FileUploadEnabled {
		handler.uploadDisabled(response, request, requestLocale)
		return "", false
	}
	if !access.CanWrite {
		handler.uploadForbidden(response, request, requestLocale)
		return "", false
	}
	return actorEmail, true
}

func (handler *Handler) userAccess(request *http.Request) (string, webapp.UserAccess, error) {
	actorEmail := ""
	if claims, ok := webapp.IAPClaimsFromContext(request.Context()); ok {
		actorEmail = claims.Email
	}
	access, err := handler.repository.GetUserAccess(request.Context(), actorEmail)
	return actorEmail, access, err
}

func (handler *Handler) requireWriteActor(response http.ResponseWriter, request *http.Request, requestLocale locale) (string, bool) {
	actorEmail, access, err := handler.userAccess(request)
	if err != nil {
		handler.internalError(response, request, requestLocale, err)
		return "", false
	}
	if !access.CanWrite {
		handler.writeForbidden(response, request, requestLocale)
		return "", false
	}
	return actorEmail, true
}

func (handler *Handler) writeForbidden(response http.ResponseWriter, request *http.Request, requestLocale locale) {
	msg := messagesFor(requestLocale)
	render(response, request, http.StatusForbidden, errorPage(errorPageModel{
		Page: newPageContext(requestLocale, "/"), Status: http.StatusForbidden,
		Title: msg.WriteForbiddenTitle, Message: msg.WriteForbiddenMessage,
	}))
}

func (handler *Handler) uploadDisabled(response http.ResponseWriter, request *http.Request, requestLocale locale) {
	msg := messagesFor(requestLocale)
	render(response, request, http.StatusForbidden, errorPage(errorPageModel{
		Page: newPageContext(requestLocale, "/"), Status: http.StatusForbidden,
		Title: msg.UploadDisabledTitle, Message: msg.UploadDisabledMessage,
	}))
}

func (handler *Handler) uploadForbidden(response http.ResponseWriter, request *http.Request, requestLocale locale) {
	msg := messagesFor(requestLocale)
	render(response, request, http.StatusForbidden, errorPage(errorPageModel{
		Page: newPageContext(requestLocale, "/"), Status: http.StatusForbidden,
		Title: msg.UploadForbiddenTitle, Message: msg.UploadForbiddenMessage,
	}))
}

func newIndexPageModel(requestLocale locale, filter webapp.EntryFilter, page webapp.EntryPage, searchApplied, uploadEnabled bool) indexPageModel {
	model := indexPageModel{
		Page:          newPageContext(requestLocale, "/"),
		UploadEnabled: uploadEnabled,
		Upload:        uploadFormModel{Action: importHref(requestLocale)},
		TacklerUpload: uploadFormModel{Action: tacklerImportHref(requestLocale)},
		Search:        searchFormModel{Action: searchHref(requestLocale), ResetHref: localizedPath(requestLocale, "/"), Filter: filter},
		Export:        exportFormModel{TacklerAction: exportHref(requestLocale, "tackler"), JSONAction: exportHref(requestLocale, "json"), Filter: filter},
		Entries:       make([]entrySummaryModel, 0, len(page.Entries)),
		NextCursor:    page.NextCursor,
		SearchApplied: searchApplied,
	}
	for _, entry := range page.Entries {
		model.Entries = append(model.Entries, entrySummaryModel{
			Href: entryHref(requestLocale, entry.ID), OccurredAt: entry.OccurredAt,
			Description: entry.Description, Status: entry.Status, WorkflowStatus: entry.WorkflowStatus,
			CurrentRevision: entry.CurrentRevision, Source: entry.Source,
		})
	}
	return model
}

func newEntryPageModel(requestLocale locale, id string, detail webapp.EntryDetail, current candidateModel, canWrite bool, formError string) entryPageModel {
	current.Approved = detail.CurrentApproval != nil && detail.CurrentApproval.Revision == current.Revision
	return entryPageModel{
		Page:     newPageContext(requestLocale, "/entries/"+url.PathEscape(id)),
		CanWrite: canWrite,
		Detail:   detail,
		Current:  current,
		RunHref:  runHref(requestLocale, detail.RunIdentity),
		RevisionForm: revisionFormModel{
			Action:       entryRevisionHref(requestLocale, id),
			BaseRevision: current.Revision,
			EntryText:    entryText(current),
		},
		ApprovalForm: approvalFormModel{
			Action:     entryApprovalHref(requestLocale, id),
			Revision:   current.Revision,
			CanApprove: current.Valid && !current.Approved,
		},
		FormError: formError,
	}
}

func newReportingSettingsPageModel(requestLocale locale, detail *webapp.ReportingConfigurationDetail, canWrite bool, formError string) reportingSettingsPageModel {
	model := reportingSettingsPageModel{
		Page: newPageContext(requestLocale, "/settings/reporting"), CanWrite: canWrite,
		Form: reportingConfigurationFormModel{
			Action: reportingSettingsMutationHref(requestLocale), StartMonth: 1,
			Classifications: []webapp.ReportingClassification{}, FiscalYears: []reportingFiscalYearFormModel{},
		},
		UnclassifiedAccounts: []string{}, FormError: formError,
	}
	if detail != nil {
		model.Configured = detail.Revision > 0
		model.Form.BaseRevision = detail.Revision
		if detail.StartMonth > 0 {
			model.Form.StartMonth = detail.StartMonth
		}
		model.Form.Classifications = append(model.Form.Classifications, detail.Classifications...)
		for _, year := range detail.FiscalYears {
			model.Form.FiscalYears = append(model.Form.FiscalYears, reportingFiscalYearFormModel{
				StartDate: year.StartDate, EndDate: year.EndDate, OpeningMode: year.OpeningMode,
				OpeningEntryIDs: strings.Join(year.OpeningEntryIDs, "\n"),
			})
		}
	}
	if detail == nil {
		model.Form.Classifications = append(model.Form.Classifications,
			webapp.ReportingClassification{Category: reporting.CategoryAsset})
		model.Form.FiscalYears = append(model.Form.FiscalYears,
			reportingFiscalYearFormModel{OpeningMode: reporting.OpeningAutomatic})
	}
	return model
}

func reportingConfigurationDetailFromRequest(input webapp.ReportingConfigurationRequest) webapp.ReportingConfigurationDetail {
	detail := webapp.ReportingConfigurationDetail{
		StartMonth: input.StartMonth, Classifications: input.Classifications, FiscalYears: input.FiscalYears,
	}
	if input.BaseRevision != nil {
		detail.Revision = *input.BaseRevision
	}
	return detail
}

func newTrialBalancePageModel(requestLocale locale, detail webapp.ReportingConfigurationDetail) (trialBalancePageModel, error) {
	model := trialBalancePageModel{
		Page: newPageContext(requestLocale, "/reports/trial-balance"), Configured: true,
		SetupHref: reportingSettingsHref(requestLocale), FormAction: trialBalanceMutationHref(requestLocale),
		Periods: []trialBalancePeriodOption{},
	}
	for _, year := range detail.FiscalYears {
		periods, err := reporting.FiscalPeriods(reporting.FiscalYear{
			StartDate: year.StartDate, EndDate: year.EndDate, OpeningMode: year.OpeningMode,
			OpeningEntryIDs: year.OpeningEntryIDs,
		}, detail.StartMonth)
		if err != nil {
			return trialBalancePageModel{}, err
		}
		for _, period := range periods {
			label := messagesFor(requestLocale).FiscalYearPeriodLabel(period.StartDate, period.EndDate)
			if period.Month > 0 {
				label = messagesFor(requestLocale).MonthlyPeriodLabel(period.Month, period.StartDate, period.EndDate)
			}
			model.Periods = append(model.Periods, trialBalancePeriodOption{Period: period.Period, Label: label})
		}
		model.Selected = periods[0].Period
	}
	return model, nil
}

func newCurrentOverviewPageModel(requestLocale locale, detail webapp.ReportingConfigurationDetail, asOf string) (currentOverviewPageModel, error) {
	model := currentOverviewPageModel{
		Page: newPageContext(requestLocale, "/reports/current"), Configured: true,
		SetupHref: reportingSettingsHref(requestLocale), FormAction: currentOverviewMutationHref(requestLocale),
		AsOf: asOf, Periods: []trialBalancePeriodOption{},
	}
	selected := false
	for _, year := range detail.FiscalYears {
		periods, err := reporting.FiscalPeriods(reporting.FiscalYear{
			StartDate: year.StartDate, EndDate: year.EndDate, OpeningMode: year.OpeningMode,
			OpeningEntryIDs: year.OpeningEntryIDs,
		}, detail.StartMonth)
		if err != nil {
			return currentOverviewPageModel{}, err
		}
		for _, period := range periods[1:] {
			model.Periods = append(model.Periods, trialBalancePeriodOption{
				Period: period.Period,
				Label:  messagesFor(requestLocale).MonthlyPeriodLabel(period.Month, period.StartDate, period.EndDate),
			})
			if !selected {
				model.Selected = period.Period
			}
			if asOf >= period.StartDate && asOf <= period.EndDate {
				model.Selected = period.Period
				selected = true
			}
		}
	}
	return model, nil
}

func newBalanceSheetPageModel(requestLocale locale, detail webapp.ReportingConfigurationDetail) (balanceSheetPageModel, error) {
	model := balanceSheetPageModel{
		Page: newPageContext(requestLocale, "/reports/balance-sheet"), Configured: true,
		SetupHref: reportingSettingsHref(requestLocale), FormAction: balanceSheetMutationHref(requestLocale),
		Periods: []trialBalancePeriodOption{},
	}
	for _, year := range detail.FiscalYears {
		period := reporting.Period{StartDate: year.StartDate, EndDate: year.EndDate}
		model.Periods = append(model.Periods, trialBalancePeriodOption{
			Period: period, Label: messagesFor(requestLocale).FiscalYearPeriodLabel(period.StartDate, period.EndDate),
		})
		model.Selected = period
	}
	return model, nil
}

func newClosingBalanceSheetPageModel(requestLocale locale, detail webapp.ReportingConfigurationDetail) (closingBalanceSheetPageModel, error) {
	model := closingBalanceSheetPageModel{
		Page: newPageContext(requestLocale, "/reports/closing-balance-sheet"), Configured: true,
		SetupHref: reportingSettingsHref(requestLocale), FormAction: closingBalanceSheetMutationHref(requestLocale),
		Periods: []trialBalancePeriodOption{},
	}
	for _, year := range detail.FiscalYears {
		period := reporting.Period{StartDate: year.StartDate, EndDate: year.EndDate}
		model.Periods = append(model.Periods, trialBalancePeriodOption{
			Period: period, Label: messagesFor(requestLocale).FiscalYearPeriodLabel(period.StartDate, period.EndDate),
		})
		model.Selected = period
	}
	return model, nil
}

func newIncomeStatementPageModel(requestLocale locale, detail webapp.ReportingConfigurationDetail) (incomeStatementPageModel, error) {
	model := incomeStatementPageModel{
		Page: newPageContext(requestLocale, "/reports/income-statement"), Configured: true,
		SetupHref: reportingSettingsHref(requestLocale), FormAction: incomeStatementMutationHref(requestLocale),
		Periods: []trialBalancePeriodOption{},
	}
	for _, year := range detail.FiscalYears {
		periods, err := reporting.FiscalPeriods(reporting.FiscalYear{
			StartDate: year.StartDate, EndDate: year.EndDate, OpeningMode: year.OpeningMode,
			OpeningEntryIDs: year.OpeningEntryIDs,
		}, detail.StartMonth)
		if err != nil {
			return incomeStatementPageModel{}, err
		}
		model.Periods = append(model.Periods, trialBalancePeriodOption{
			Period: periods[0].Period,
			Label:  messagesFor(requestLocale).FullYearIncomeLabel(periods[0].StartDate, periods[0].EndDate),
		})
		for _, period := range periods[1:] {
			model.Periods = append(model.Periods, trialBalancePeriodOption{
				Period: period.Period,
				Label:  messagesFor(requestLocale).MonthlyPeriodLabel(period.Month, period.StartDate, period.EndDate),
			})
			model.Selected = period.Period
		}
	}
	return model, nil
}

func newBalanceTrendPageModel(requestLocale locale, detail webapp.ReportingConfigurationDetail) (balanceTrendPageModel, error) {
	model := balanceTrendPageModel{
		Page: newPageContext(requestLocale, "/reports/balance-trend"), Configured: true,
		SetupHref: reportingSettingsHref(requestLocale), FormAction: balanceTrendMutationHref(requestLocale),
		Periods: []trialBalancePeriodOption{},
	}
	for _, year := range detail.FiscalYears {
		period := reporting.Period{StartDate: year.StartDate, EndDate: year.EndDate}
		model.Periods = append(model.Periods, trialBalancePeriodOption{
			Period: period, Label: messagesFor(requestLocale).FiscalYearPeriodLabel(period.StartDate, period.EndDate),
		})
		model.Selected = period
	}
	return model, nil
}

func trialBalancePeriodQuery(query url.Values, fallback reporting.Period) (reporting.Period, bool, bool) {
	if len(query) == 0 {
		return fallback, false, true
	}
	for key, values := range query {
		if (key != "start_date" && key != "end_date") || len(values) != 1 {
			return reporting.Period{}, false, false
		}
	}
	period := reporting.Period{StartDate: query.Get("start_date"), EndDate: query.Get("end_date")}
	if period.StartDate == "" || period.EndDate == "" {
		return reporting.Period{}, false, false
	}
	return period, true, true
}

func currentCandidate(detail webapp.EntryDetail) candidateModel {
	current := candidateModel{
		Revision: 0, OccurredAt: detail.OccurredAt, Description: detail.Description,
		Comments: detail.Comments, Postings: detail.Postings, Valid: true,
	}
	if len(detail.Revisions) > 0 {
		latest := detail.Revisions[len(detail.Revisions)-1]
		current = candidateModel{
			Revision: latest.Revision, OccurredAt: latest.OccurredAt, Description: latest.Description,
			Comments: latest.Comments, Postings: latest.Postings, Valid: latest.Valid,
		}
	}
	return current
}

func decodeImportUpload(response http.ResponseWriter, request *http.Request) ([]byte, int, bool) {
	if request.URL.RawQuery != "" {
		return nil, http.StatusBadRequest, false
	}
	mediaType, params, err := mime.ParseMediaType(request.Header.Get("Content-Type"))
	if err != nil || mediaType != "multipart/form-data" {
		return nil, http.StatusUnsupportedMediaType, false
	}
	boundary := params["boundary"]
	if boundary == "" {
		return nil, http.StatusBadRequest, false
	}

	reader := multipart.NewReader(http.MaxBytesReader(response, request.Body, maxImportRequestSize), boundary)
	var body []byte
	fileSeen := false
	for {
		part, err := reader.NextPart()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			if requestTooLarge(err) {
				return nil, http.StatusRequestEntityTooLarge, false
			}
			return nil, http.StatusBadRequest, false
		}
		data, status, ok := readImportPart(part, fileSeen)
		_ = part.Close()
		if !ok {
			return nil, status, false
		}
		body = data
		fileSeen = true
	}
	if !fileSeen {
		return nil, http.StatusBadRequest, false
	}
	return body, http.StatusOK, true
}

func readImportPart(part *multipart.Part, fileSeen bool) ([]byte, int, bool) {
	if fileSeen || part.FormName() != importFileField || part.FileName() == "" {
		return nil, http.StatusBadRequest, false
	}
	body, err := io.ReadAll(io.LimitReader(part, maxImportFileSize+1))
	if err != nil {
		if requestTooLarge(err) {
			return nil, http.StatusRequestEntityTooLarge, false
		}
		return nil, http.StatusBadRequest, false
	}
	if len(body) > maxImportFileSize {
		return nil, http.StatusRequestEntityTooLarge, false
	}
	return body, http.StatusOK, true
}

func requestTooLarge(err error) bool {
	var maxBytesError *http.MaxBytesError
	return errors.As(err, &maxBytesError)
}

func render(response http.ResponseWriter, request *http.Request, status int, component templ.Component) {
	response.Header().Set("Content-Type", "text/html; charset=utf-8")
	response.WriteHeader(status)
	if request.Method == http.MethodHead {
		return
	}
	_ = component.Render(request.Context(), response)
}

func decodeSearchForm(response http.ResponseWriter, request *http.Request) (webapp.EntryFilter, string, bool) {
	if request.URL.RawQuery != "" {
		return webapp.EntryFilter{}, "", false
	}
	mediaType, _, err := mime.ParseMediaType(request.Header.Get("Content-Type"))
	if err != nil || mediaType != "application/x-www-form-urlencoded" {
		return webapp.EntryFilter{}, "", false
	}
	body, err := io.ReadAll(http.MaxBytesReader(response, request.Body, maxSearchFormBody))
	if err != nil {
		return webapp.EntryFilter{}, "", false
	}
	form, err := url.ParseQuery(string(body))
	if err != nil {
		return webapp.EntryFilter{}, "", false
	}
	for key, values := range form {
		if !searchFormFieldAllowed(key) || len(values) > 1 {
			return webapp.EntryFilter{}, "", false
		}
	}
	return webapp.EntryFilter{
		DateFrom:        form.Get(searchFieldDateFrom),
		DateTo:          form.Get(searchFieldDateTo),
		Account:         form.Get(searchFieldAccount),
		Description:     form.Get(searchFieldDescription),
		Status:          form.Get(searchFieldStatus),
		WorkflowStatus:  form.Get(searchFieldWorkflowStatus),
		SourceNamespace: form.Get(searchFieldSourceNamespace),
		SourceDisplay:   form.Get(searchFieldSourceDisplay),
	}, form.Get(searchFieldCursor), true
}

func decodeExportForm(response http.ResponseWriter, request *http.Request) (webapp.EntryFilter, bool) {
	filter, cursor, ok := decodeSearchForm(response, request)
	return filter, ok && cursor == ""
}

func decodeRevisionForm(response http.ResponseWriter, request *http.Request) (webapp.RevisionRequest, bool) {
	form, ok := decodeURLEncodedForm(response, request, maxRevisionFormBody)
	if !ok {
		return webapp.RevisionRequest{}, false
	}
	if !revisionFormFieldsAllowed(form) {
		return webapp.RevisionRequest{}, false
	}
	baseRevision, err := strconv.Atoi(form.Get(revisionFieldBaseRevision))
	if err != nil || baseRevision < 0 {
		return webapp.RevisionRequest{}, false
	}
	input, ok := parseEntryText(form.Get(revisionFieldEntry))
	if !ok {
		return webapp.RevisionRequest{}, false
	}
	input.BaseRevision = &baseRevision
	return input, true
}

func decodeApprovalForm(response http.ResponseWriter, request *http.Request) (webapp.ApprovalRequest, bool) {
	form, ok := decodeURLEncodedForm(response, request, maxRevisionFormBody)
	if !ok {
		return webapp.ApprovalRequest{}, false
	}
	for key, values := range form {
		if key != approvalFieldRevision || len(values) > 1 {
			return webapp.ApprovalRequest{}, false
		}
	}
	revision, err := strconv.Atoi(form.Get(approvalFieldRevision))
	if err != nil || revision < 0 {
		return webapp.ApprovalRequest{}, false
	}
	return webapp.ApprovalRequest{Revision: &revision}, true
}

func decodeReportingConfigurationForm(response http.ResponseWriter, request *http.Request) (webapp.ReportingConfigurationRequest, bool) {
	form, ok := decodeStrictForm(response, request, maxReportingFormBody, map[string]bool{
		"base_revision": false, "start_month": false,
		"classification_account": true, "classification_category": true,
		"fiscal_start_date": true, "fiscal_end_date": true, "opening_mode": true, "opening_entry_ids": true,
	})
	if !ok {
		return webapp.ReportingConfigurationRequest{}, false
	}
	baseRevision, err := strconv.Atoi(form.Get("base_revision"))
	if err != nil || baseRevision < 0 || strconv.Itoa(baseRevision) != form.Get("base_revision") {
		return webapp.ReportingConfigurationRequest{}, false
	}
	startMonth, err := strconv.Atoi(form.Get("start_month"))
	if err != nil || startMonth < 1 || startMonth > 12 || strconv.Itoa(startMonth) != form.Get("start_month") {
		return webapp.ReportingConfigurationRequest{}, false
	}
	input := webapp.ReportingConfigurationRequest{
		BaseRevision: &baseRevision, StartMonth: startMonth,
		Classifications: []webapp.ReportingClassification{}, FiscalYears: []webapp.ReportingFiscalYear{},
	}
	accounts, categories := form["classification_account"], form["classification_category"]
	if len(accounts) != len(categories) {
		return webapp.ReportingConfigurationRequest{}, false
	}
	for index, account := range accounts {
		account = strings.TrimSpace(account)
		if account == "" {
			continue
		}
		input.Classifications = append(input.Classifications, webapp.ReportingClassification{
			Account: account, Category: reporting.Category(categories[index]),
		})
	}
	starts, ends := form["fiscal_start_date"], form["fiscal_end_date"]
	modes, openingIDs := form["opening_mode"], form["opening_entry_ids"]
	if len(starts) != len(ends) || len(starts) != len(modes) || len(starts) != len(openingIDs) {
		return webapp.ReportingConfigurationRequest{}, false
	}
	for index := range starts {
		start, end, idsText := strings.TrimSpace(starts[index]), strings.TrimSpace(ends[index]), strings.TrimSpace(openingIDs[index])
		if start == "" && end == "" && idsText == "" {
			continue
		}
		year := webapp.ReportingFiscalYear{
			StartDate: start, EndDate: end, OpeningMode: reporting.OpeningMode(modes[index]), OpeningEntryIDs: []string{},
		}
		for _, line := range strings.Split(idsText, "\n") {
			if id := strings.TrimSpace(line); id != "" {
				year.OpeningEntryIDs = append(year.OpeningEntryIDs, id)
			}
		}
		input.FiscalYears = append(input.FiscalYears, year)
	}
	return input, true
}

func decodeStrictForm(response http.ResponseWriter, request *http.Request, limit int64, allowed map[string]bool) (url.Values, bool) {
	if request.URL.RawQuery != "" {
		return nil, false
	}
	mediaType, _, err := mime.ParseMediaType(request.Header.Get("Content-Type"))
	if err != nil || mediaType != "application/x-www-form-urlencoded" {
		return nil, false
	}
	body, err := io.ReadAll(http.MaxBytesReader(response, request.Body, limit))
	if err != nil {
		return nil, false
	}
	form, err := url.ParseQuery(string(body))
	if err != nil {
		return nil, false
	}
	for key, values := range form {
		repeated, found := allowed[key]
		if !found || len(values) == 0 || (!repeated && len(values) != 1) {
			return nil, false
		}
	}
	for key := range allowed {
		if _, found := form[key]; !found {
			return nil, false
		}
	}
	return form, true
}

func decodeURLEncodedForm(response http.ResponseWriter, request *http.Request, limit int64) (url.Values, bool) {
	if request.URL.RawQuery != "" {
		return nil, false
	}
	mediaType, _, err := mime.ParseMediaType(request.Header.Get("Content-Type"))
	if err != nil || mediaType != "application/x-www-form-urlencoded" {
		return nil, false
	}
	body, err := io.ReadAll(http.MaxBytesReader(response, request.Body, limit))
	if err != nil {
		return nil, false
	}
	form, err := url.ParseQuery(string(body))
	if err != nil {
		return nil, false
	}
	for _, values := range form {
		if len(values) > 1 {
			return nil, false
		}
	}
	return form, true
}

func revisionFormFieldsAllowed(form url.Values) bool {
	for key := range form {
		switch key {
		case revisionFieldBaseRevision, revisionFieldEntry:
			continue
		default:
			return false
		}
	}
	return true
}

func entryText(entry candidateModel) string {
	lines := []string{entry.OccurredAt + "  '" + entry.Description}
	for _, comment := range entry.Comments {
		lines = append(lines, "    ; "+comment)
	}
	for _, posting := range entry.Postings {
		var builder strings.Builder
		builder.WriteString(posting.Account)
		if posting.Amount != nil {
			builder.WriteByte(' ')
			builder.WriteString(*posting.Amount)
			builder.WriteByte(' ')
			builder.WriteString(posting.Commodity)
		}
		if posting.TotalPrice != nil {
			builder.WriteString(" = ")
			builder.WriteString(posting.TotalPrice.Amount)
			builder.WriteByte(' ')
			builder.WriteString(posting.TotalPrice.Commodity)
		}
		if posting.Comment != "" {
			builder.WriteString(" ; ")
			builder.WriteString(posting.Comment)
		}
		lines = append(lines, "    "+builder.String())
	}
	return strings.Join(lines, "\n")
}

func parseEntryText(text string) (webapp.RevisionRequest, bool) {
	entries, err := tacklerfmt.ParseUnvalidated([]byte(text))
	if err != nil || len(entries) != 1 {
		return webapp.RevisionRequest{}, false
	}
	entry := entries[0]
	input := webapp.RevisionRequest{
		OccurredAt:  entry.Date.String(),
		Description: entry.Description,
		Comments:    append([]string(nil), entry.Comments...),
		Postings:    make([]webapp.PostingDetail, 0, len(entry.Postings)),
	}
	for _, posting := range entry.Postings {
		detail := webapp.PostingDetail{Account: posting.Account, Comment: posting.Comment}
		if posting.Amount != nil {
			amount := posting.Amount.Value.String()
			detail.Amount = &amount
			detail.Commodity = string(posting.Amount.Commodity)
		}
		if posting.TotalPrice != nil {
			detail.TotalPrice = &webapp.AmountDetail{
				Amount: posting.TotalPrice.Value.String(), Commodity: string(posting.TotalPrice.Commodity),
			}
		}
		input.Postings = append(input.Postings, detail)
	}
	return input, true
}

func normalizedInputForTacklerEntries(entries []ledger.JournalEntry) ([]byte, error) {
	batch := normalizedBatch{SchemaVersion: ingest.SchemaVersion, Records: make([]normalizedRecord, 0, len(entries))}
	for index, entry := range entries {
		record := normalizedRecord{
			Source: normalizedSource{
				Namespace:  "tackler",
				Display:    "uploaded.txn",
				ExternalID: tacklerExternalID(entry, index),
			},
			OccurredAt:  entry.Date.String(),
			Description: entry.Description,
			Comments:    append([]string(nil), entry.Comments...),
			Postings:    make([]normalizedPosting, 0, len(entry.Postings)),
		}
		for _, posting := range entry.Postings {
			detail := normalizedPosting{Account: posting.Account, Comment: posting.Comment}
			if posting.Amount != nil {
				detail.Amount = posting.Amount.Value.String()
				detail.Commodity = string(posting.Amount.Commodity)
			}
			if posting.TotalPrice != nil {
				detail.TotalPrice = &normalizedAmount{
					Amount: posting.TotalPrice.Value.String(), Commodity: string(posting.TotalPrice.Commodity),
				}
			}
			record.Postings = append(record.Postings, detail)
		}
		batch.Records = append(batch.Records, record)
	}
	return json.Marshal(batch)
}

func tacklerExternalID(entry ledger.JournalEntry, index int) string {
	output, err := tacklerfmt.Export([]ledger.JournalEntry{entry}, tacklerfmt.Options{OmittedAmounts: tacklerfmt.PreserveOmitted})
	if err != nil {
		output = []byte(entry.Date.String() + "\n" + entry.Description)
	}
	sum := sha256.Sum256(append([]byte("bokiccio.tackler-import.v1\x00"), output...))
	return fmt.Sprintf("entry-%06d-%x", index+1, sum[:])
}

const (
	revisionFieldBaseRevision = "base_revision"
	revisionFieldEntry        = "entry"
	approvalFieldRevision     = "revision"
)

func formInt(value int) string {
	return strconv.Itoa(value)
}

const (
	searchFieldDateFrom        = "date_from"
	searchFieldDateTo          = "date_to"
	searchFieldAccount         = "account"
	searchFieldDescription     = "description"
	searchFieldStatus          = "status"
	searchFieldWorkflowStatus  = "workflow_status"
	searchFieldSourceNamespace = "source_namespace"
	searchFieldSourceDisplay   = "source_display"
	searchFieldCursor          = "cursor"
)

var searchFormFields = map[string]struct{}{
	searchFieldDateFrom:        {},
	searchFieldDateTo:          {},
	searchFieldAccount:         {},
	searchFieldDescription:     {},
	searchFieldStatus:          {},
	searchFieldWorkflowStatus:  {},
	searchFieldSourceNamespace: {},
	searchFieldSourceDisplay:   {},
	searchFieldCursor:          {},
}

type normalizedBatch struct {
	SchemaVersion int                `json:"schema_version"`
	Records       []normalizedRecord `json:"records"`
}

type normalizedRecord struct {
	Source      normalizedSource    `json:"source"`
	OccurredAt  string              `json:"occurred_at"`
	Description string              `json:"description"`
	Comments    []string            `json:"comments,omitempty"`
	Postings    []normalizedPosting `json:"postings"`
}

type normalizedSource struct {
	Namespace  string `json:"namespace"`
	Display    string `json:"display"`
	ExternalID string `json:"external_id"`
}

type normalizedPosting struct {
	Account    string            `json:"account"`
	Amount     string            `json:"amount,omitempty"`
	Commodity  string            `json:"commodity,omitempty"`
	TotalPrice *normalizedAmount `json:"total_price,omitempty"`
	Comment    string            `json:"comment,omitempty"`
}

type normalizedAmount struct {
	Amount    string `json:"amount"`
	Commodity string `json:"commodity"`
}

func searchFormFieldAllowed(key string) bool {
	_, ok := searchFormFields[key]
	return ok
}

func isHXRequest(request *http.Request) bool {
	return request.Header.Get("HX-Request") == "true"
}

func addVary(response http.ResponseWriter, value string) {
	current := response.Header().Get("Vary")
	if current == "" {
		response.Header().Set("Vary", value)
		return
	}
	for _, item := range strings.Split(current, ",") {
		if strings.EqualFold(strings.TrimSpace(item), value) {
			return
		}
	}
	response.Header().Set("Vary", current+", "+value)
}

func pathID(escaped string) (string, bool) {
	if escaped == "" || strings.Contains(escaped, "/") {
		return "", false
	}
	id, err := url.PathUnescape(escaped)
	return id, err == nil && id != "" && !strings.Contains(id, "/")
}

func localeRoute(path string) (locale, string) {
	if strings.HasPrefix(path, "/en/") {
		localPath := strings.TrimPrefix(path, "/en")
		if localPath == "" {
			localPath = "/"
		}
		return localeEN, localPath
	}
	return localeJA, path
}

func setPrivateHeaders(response http.ResponseWriter) {
	response.Header().Set("Cache-Control", "no-store")
	response.Header().Set("Content-Security-Policy", "default-src 'self'; object-src 'none'; base-uri 'none'; frame-ancestors 'none'; form-action 'self'")
	response.Header().Set("Referrer-Policy", "same-origin")
	response.Header().Set("X-Content-Type-Options", "nosniff")
}

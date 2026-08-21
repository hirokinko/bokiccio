package webstore

import (
	"testing"

	"github.com/hirokinko/bokiccio/internal/ledger"
	"github.com/hirokinko/bokiccio/internal/reporting"
	"github.com/hirokinko/bokiccio/internal/webapp"
)

func TestReportingSnapshotIdentityIsStableAndCoversReportInputs(t *testing.T) {
	t.Parallel()
	date, err := ledger.ParseEntryTime("2025-04-01")
	if err != nil {
		t.Fatal(err)
	}
	amount, err := ledger.ParseDecimal("10.00")
	if err != nil {
		t.Fatal(err)
	}
	configuration := reporting.Configuration{Revision: 3}
	entries := []reporting.Entry{{ID: "entry-1", Entry: ledger.JournalEntry{
		Date: date, Description: "anonymous transaction", Comments: []string{"source note"},
		Postings: []ledger.Posting{{Account: "Assets:Cash", Amount: &ledger.Amount{Value: amount, Commodity: "JPY"}}},
	}}}

	identity := reportingSnapshotIdentity(configuration, entries)
	if identity != reportingSnapshotIdentity(configuration, entries) || len(identity) != 64 {
		t.Fatalf("snapshot identity is not stable: %q", identity)
	}
	changedComment := append([]reporting.Entry(nil), entries...)
	changedComment[0].Entry.Comments = []string{"changed source note"}
	if identity == reportingSnapshotIdentity(configuration, changedComment) {
		t.Fatal("snapshot identity did not cover entry comments")
	}
	configuration.Revision++
	if identity == reportingSnapshotIdentity(configuration, entries) {
		t.Fatal("snapshot identity did not cover the configuration revision")
	}
}

func TestDrillDownPaginationBindsCursorToSelector(t *testing.T) {
	t.Parallel()
	query := webapp.ReportDrillDownQuery{
		DrillDown: reporting.DrillDownQuery{
			Period:    reporting.Period{StartDate: "2025-04-01", EndDate: "2025-04-30"},
			Commodity: "JPY", Category: reporting.CategoryAsset, Account: "Assets", Scope: reporting.DrillDownSubtree,
		},
		SnapshotIdentity: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		Limit:            2,
	}
	result := reporting.TrialBalanceDrillDown{Entries: []reporting.TrialBalanceDrillDownEntry{
		{ID: "one"}, {ID: "two"}, {ID: "three"},
	}}
	cursor, err := paginateTrialBalanceDrillDown(&result, query)
	if err != nil || cursor == "" || len(result.Entries) != 2 || result.Entries[1].ID != "two" {
		t.Fatalf("first page cursor=%q entries=%+v error=%v", cursor, result.Entries, err)
	}
	query.Cursor = cursor
	next := reporting.TrialBalanceDrillDown{Entries: []reporting.TrialBalanceDrillDownEntry{
		{ID: "one"}, {ID: "two"}, {ID: "three"},
	}}
	nextCursor, err := paginateTrialBalanceDrillDown(&next, query)
	if err != nil || nextCursor != "" || len(next.Entries) != 1 || next.Entries[0].ID != "three" {
		t.Fatalf("second page cursor=%q entries=%+v error=%v", nextCursor, next.Entries, err)
	}

	query.DrillDown.Account = "Assets:Cash"
	invalid := reporting.TrialBalanceDrillDown{Entries: []reporting.TrialBalanceDrillDownEntry{
		{ID: "one"}, {ID: "two"}, {ID: "three"},
	}}
	if _, err := paginateTrialBalanceDrillDown(&invalid, query); err == nil {
		t.Fatal("cursor was accepted for a different selector")
	}

	query.DrillDown.Account = "Assets"
	income := reporting.IncomeStatementDrillDown{Entries: []reporting.IncomeStatementDrillDownEntry{
		{ID: "one"}, {ID: "two"}, {ID: "three"},
	}}
	if _, err := paginateIncomeStatementDrillDown(&income, query); err == nil {
		t.Fatal("trial-balance cursor was accepted by income-statement pagination")
	}
}

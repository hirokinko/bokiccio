package webstore

import (
	"context"
	"fmt"

	"github.com/hirokinko/bokiccio/internal/reporting"
	"github.com/hirokinko/bokiccio/internal/webapp"
)

func (store *Store) GetCurrentOverview(ctx context.Context, asOf string, expensePeriod reporting.Period) (_ webapp.CurrentOverviewDetail, resultErr error) {
	transaction, configuration, entries, err := store.reportingSnapshot(ctx, "current overview")
	if err != nil {
		return webapp.CurrentOverviewDetail{}, err
	}
	defer func() {
		if resultErr != nil {
			_ = transaction.Rollback()
		}
	}()
	report, err := reporting.BuildCurrentOverview(configuration, entries, asOf, expensePeriod)
	if err != nil {
		return webapp.CurrentOverviewDetail{}, err
	}
	if err := transaction.Commit(); err != nil {
		return webapp.CurrentOverviewDetail{}, fmt.Errorf("commit current overview transaction: %w", err)
	}
	return webapp.CurrentOverviewDetail{SchemaVersion: webapp.APISchemaVersion, CurrentOverview: report}, nil
}

func (store *Store) GetBalanceSheet(ctx context.Context, period reporting.Period) (_ webapp.BalanceSheetDetail, resultErr error) {
	transaction, configuration, entries, err := store.reportingSnapshot(ctx, "balance sheet")
	if err != nil {
		return webapp.BalanceSheetDetail{}, err
	}
	defer func() {
		if resultErr != nil {
			_ = transaction.Rollback()
		}
	}()
	report, err := reporting.BuildBalanceSheet(configuration, entries, period)
	if err != nil {
		return webapp.BalanceSheetDetail{}, err
	}
	if err := transaction.Commit(); err != nil {
		return webapp.BalanceSheetDetail{}, fmt.Errorf("commit balance sheet transaction: %w", err)
	}
	return webapp.BalanceSheetDetail{SchemaVersion: webapp.APISchemaVersion, BalanceSheet: report}, nil
}

func (store *Store) GetIncomeStatement(ctx context.Context, period reporting.Period) (_ webapp.IncomeStatementDetail, resultErr error) {
	transaction, configuration, entries, err := store.reportingSnapshot(ctx, "income statement")
	if err != nil {
		return webapp.IncomeStatementDetail{}, err
	}
	defer func() {
		if resultErr != nil {
			_ = transaction.Rollback()
		}
	}()
	report, err := reporting.BuildIncomeStatement(configuration, entries, period)
	if err != nil {
		return webapp.IncomeStatementDetail{}, err
	}
	if err := transaction.Commit(); err != nil {
		return webapp.IncomeStatementDetail{}, fmt.Errorf("commit income statement transaction: %w", err)
	}
	return webapp.IncomeStatementDetail{SchemaVersion: webapp.APISchemaVersion, IncomeStatement: report}, nil
}

func (store *Store) GetBalanceTrend(ctx context.Context, period reporting.Period) (_ webapp.BalanceTrendDetail, resultErr error) {
	transaction, configuration, entries, err := store.reportingSnapshot(ctx, "balance trend")
	if err != nil {
		return webapp.BalanceTrendDetail{}, err
	}
	defer func() {
		if resultErr != nil {
			_ = transaction.Rollback()
		}
	}()
	report, err := reporting.BuildBalanceTrend(configuration, entries, period)
	if err != nil {
		return webapp.BalanceTrendDetail{}, err
	}
	if err := transaction.Commit(); err != nil {
		return webapp.BalanceTrendDetail{}, fmt.Errorf("commit balance trend transaction: %w", err)
	}
	return webapp.BalanceTrendDetail{SchemaVersion: webapp.APISchemaVersion, BalanceTrend: report}, nil
}

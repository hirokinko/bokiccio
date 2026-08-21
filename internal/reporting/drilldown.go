package reporting

import (
	"errors"
	"strings"

	"github.com/hirokinko/bokiccio/internal/ledger"
)

var ErrInvalidDrillDown = errors.New("invalid report drill-down")

type DrillDownScope string

const (
	DrillDownDirect  DrillDownScope = "direct"
	DrillDownSubtree DrillDownScope = "subtree"
)

type DrillDownQuery struct {
	Period    Period
	Commodity string
	Category  Category
	Account   string
	Scope     DrillDownScope
}

type PostingContribution struct {
	PostingIndex int    `json:"posting_index"`
	Account      string `json:"account"`
	Amount       string `json:"amount"`
	Commodity    string `json:"commodity"`
}

type TrialBalanceDrillDownEntry struct {
	ID            string                `json:"id"`
	OccurredAt    string                `json:"occurred_at"`
	Description   string                `json:"description"`
	Role          string                `json:"role"`
	Contributions []PostingContribution `json:"contributions"`
	Amounts       Amounts               `json:"amounts"`
}

type TrialBalanceDrillDown struct {
	ConfigurationRevision int                          `json:"configuration_revision"`
	Period                FiscalPeriod                 `json:"period"`
	Commodity             string                       `json:"commodity"`
	Category              Category                     `json:"category"`
	Account               string                       `json:"account"`
	Scope                 DrillDownScope               `json:"scope"`
	Amounts               Amounts                      `json:"amounts"`
	Entries               []TrialBalanceDrillDownEntry `json:"entries"`
}

type IncomeStatementDrillDownEntry struct {
	ID            string                `json:"id"`
	OccurredAt    string                `json:"occurred_at"`
	Description   string                `json:"description"`
	Contributions []PostingContribution `json:"contributions"`
	Balance       Balance               `json:"balance"`
}

type IncomeStatementDrillDown struct {
	ConfigurationRevision int                             `json:"configuration_revision"`
	Period                FiscalPeriod                    `json:"period"`
	Commodity             string                          `json:"commodity"`
	Category              Category                        `json:"category"`
	Account               string                          `json:"account"`
	Scope                 DrillDownScope                  `json:"scope"`
	Balance               Balance                         `json:"balance"`
	Entries               []IncomeStatementDrillDownEntry `json:"entries"`
}

type contributionRole string

const (
	contributionOpening  contributionRole = "opening"
	contributionMovement contributionRole = "movement"
)

type provenanceContribution struct {
	entry        Entry
	postingIndex int
	key          amountKey
	value        ledger.Decimal
	role         contributionRole
}

type provenanceCollector struct {
	items []provenanceContribution
}

func (collector *provenanceCollector) add(entry Entry, postingIndex int, key amountKey, value ledger.Decimal, role contributionRole) {
	if collector == nil {
		return
	}
	collector.items = append(collector.items, provenanceContribution{
		entry: entry, postingIndex: postingIndex, key: key, value: value, role: role,
	})
}

func (collector *provenanceCollector) reset() {
	if collector != nil {
		collector.items = nil
	}
}

func (collector *provenanceCollector) permanentOnly() {
	if collector == nil {
		return
	}
	items := collector.items[:0]
	for _, item := range collector.items {
		if item.key.category == CategoryAsset || item.key.category == CategoryLiability || item.key.category == CategoryEquity {
			items = append(items, item)
		}
	}
	collector.items = items
}

func addEntryCollected(target amountMap, entry Entry, classifier classifier, permanentOnly bool, role contributionRole, collector *provenanceCollector) error {
	return forEachEntryAmountIndexed(entry.Entry, classifier, permanentOnly, func(postingIndex int, key amountKey, value ledger.Decimal) {
		target.add(key, value)
		collector.add(entry, postingIndex, key, value, role)
	})
}

func BuildTrialBalanceDrillDown(configuration Configuration, entries []Entry, query DrillDownQuery) (TrialBalanceDrillDown, error) {
	if err := validateDrillDownQuery(configuration, query); err != nil {
		return TrialBalanceDrillDown{}, err
	}
	years := sortedFiscalYears(configuration.FiscalYears)
	period, targetIndex, err := selectPeriod(configuration.StartMonth, years, query.Period)
	if err != nil {
		return TrialBalanceDrillDown{}, err
	}
	validated, err := validateEntries(entries)
	if err != nil {
		return TrialBalanceDrillDown{}, err
	}
	classifier := newClassifier(configuration.Classifications)
	openingCollector := &provenanceCollector{}
	opening, err := buildFiscalOpeningCollected(years, targetIndex, validated, entries, classifier, openingCollector)
	if err != nil {
		return TrialBalanceDrillDown{}, err
	}
	year := years[targetIndex]
	if period.StartDate != period.FiscalYearStart {
		if err := addMovementsCollected(opening, entries, period.FiscalYearStart, dateBefore(period.StartDate),
			year.OpeningEntryIDs, classifier, contributionOpening, openingCollector); err != nil {
			return TrialBalanceDrillDown{}, err
		}
	}
	movementCollector := &provenanceCollector{}
	movement, err := collectMovementsCollected(entries, period.StartDate, period.EndDate, year.OpeningEntryIDs, classifier, movementCollector)
	if err != nil {
		return TrialBalanceDrillDown{}, err
	}
	report, err := assembleTrialBalance(configuration.Revision, period, opening, movement)
	if err != nil {
		return TrialBalanceDrillDown{}, err
	}
	target, found := trialBalanceTarget(report, query)
	if !found {
		return TrialBalanceDrillDown{}, ErrInvalidDrillDown
	}
	items := append(append([]provenanceContribution(nil), openingCollector.items...), movementCollector.items...)
	resultEntries, reconstructed, err := trialBalanceContributionEntries(items, query)
	if err != nil {
		return TrialBalanceDrillDown{}, err
	}
	if reconstructed != target {
		return TrialBalanceDrillDown{}, ErrInvalidEntry
	}
	return TrialBalanceDrillDown{
		ConfigurationRevision: configuration.Revision, Period: period, Commodity: query.Commodity,
		Category: query.Category, Account: query.Account, Scope: query.Scope, Amounts: target, Entries: resultEntries,
	}, nil
}

func BuildIncomeStatementDrillDown(configuration Configuration, entries []Entry, query DrillDownQuery) (IncomeStatementDrillDown, error) {
	if err := validateDrillDownQuery(configuration, query); err != nil {
		return IncomeStatementDrillDown{}, err
	}
	years, targetIndex, _, classifier, err := statementInputs(configuration, entries, query.Period, statementIncomePeriod)
	if err != nil {
		return IncomeStatementDrillDown{}, err
	}
	period, _, err := selectPeriod(configuration.StartMonth, years, query.Period)
	if err != nil {
		return IncomeStatementDrillDown{}, ErrInvalidPeriod
	}
	collector := &provenanceCollector{}
	movement, err := collectMovementsCollected(entries, period.StartDate, period.EndDate,
		years[targetIndex].OpeningEntryIDs, classifier, collector)
	if err != nil {
		return IncomeStatementDrillDown{}, err
	}
	amounts := filterAmounts(netMovements(movement), CategoryRevenue, CategoryExpense, CategoryUnknown)
	sections, _, _, err := assembleStatementCommodities(amounts)
	if err != nil {
		return IncomeStatementDrillDown{}, err
	}
	target, found := statementTarget(sections, query)
	if !found {
		return IncomeStatementDrillDown{}, ErrInvalidDrillDown
	}
	resultEntries, reconstructed, err := incomeStatementContributionEntries(collector.items, query)
	if err != nil {
		return IncomeStatementDrillDown{}, err
	}
	if reconstructed != target {
		return IncomeStatementDrillDown{}, ErrInvalidEntry
	}
	return IncomeStatementDrillDown{
		ConfigurationRevision: configuration.Revision, Period: period, Commodity: query.Commodity,
		Category: query.Category, Account: query.Account, Scope: query.Scope, Balance: target, Entries: resultEntries,
	}, nil
}

func validateDrillDownQuery(configuration Configuration, query DrillDownQuery) error {
	if err := ValidateConfiguration(configuration); err != nil {
		return err
	}
	if err := ledger.ValidateAccount(query.Account); err != nil || query.Commodity == "" || strings.ContainsAny(query.Commodity, "\r\n") {
		return ErrInvalidDrillDown
	}
	if query.Scope != DrillDownDirect && query.Scope != DrillDownSubtree {
		return ErrInvalidDrillDown
	}
	if query.Category != CategoryAsset && query.Category != CategoryLiability && query.Category != CategoryEquity &&
		query.Category != CategoryRevenue && query.Category != CategoryExpense && query.Category != CategoryUnknown {
		return ErrInvalidDrillDown
	}
	return nil
}

func trialBalanceTarget(report TrialBalance, query DrillDownQuery) (Amounts, bool) {
	for _, commodity := range report.Commodities {
		if commodity.Commodity != query.Commodity {
			continue
		}
		for _, group := range commodity.Groups {
			if group.Category != query.Category {
				continue
			}
			for _, row := range group.Accounts {
				if row.Account == query.Account {
					if query.Scope == DrillDownDirect {
						return row.Direct, true
					}
					return row.Subtotal, true
				}
			}
		}
	}
	return Amounts{}, false
}

func statementTarget(sections []StatementCommoditySection, query DrillDownQuery) (Balance, bool) {
	for _, commodity := range sections {
		if commodity.Commodity != query.Commodity {
			continue
		}
		for _, group := range commodity.Groups {
			if group.Category != query.Category {
				continue
			}
			for _, row := range group.Accounts {
				if row.Account == query.Account {
					if query.Scope == DrillDownDirect {
						return row.Direct, true
					}
					return row.Subtotal, true
				}
			}
		}
	}
	return Balance{}, false
}

func contributionMatches(item provenanceContribution, query DrillDownQuery) bool {
	if item.key.commodity != query.Commodity || item.key.category != query.Category {
		return false
	}
	return item.key.account == query.Account ||
		(query.Scope == DrillDownSubtree && strings.HasPrefix(item.key.account, query.Account+":"))
}

func postingContribution(item provenanceContribution) PostingContribution {
	return PostingContribution{
		PostingIndex: item.postingIndex, Account: item.key.account,
		Amount: item.value.String(), Commodity: item.key.commodity,
	}
}

func trialBalanceContributionEntries(items []provenanceContribution, query DrillDownQuery) ([]TrialBalanceDrillDownEntry, Amounts, error) {
	result := []TrialBalanceDrillDownEntry{}
	indexes := map[string]int{}
	sums := []rowSums{}
	total := rowSums{}
	for _, item := range items {
		if !contributionMatches(item, query) {
			continue
		}
		groupKey := string(item.role) + "\x00" + item.entry.ID
		index, found := indexes[groupKey]
		if !found {
			index = len(result)
			indexes[groupKey] = index
			result = append(result, TrialBalanceDrillDownEntry{
				ID: item.entry.ID, OccurredAt: item.entry.Entry.Date.String(), Description: item.entry.Entry.Description,
				Role: string(item.role), Contributions: []PostingContribution{},
			})
			sums = append(sums, rowSums{})
		}
		result[index].Contributions = append(result[index].Contributions, postingContribution(item))
		entrySums := sums[index]
		if item.role == contributionOpening {
			entrySums.opening.add(item.value)
			entrySums.closing.add(item.value)
			total.opening.add(item.value)
			total.closing.add(item.value)
		} else if item.value.Sign() >= 0 {
			entrySums.debit.add(item.value)
			entrySums.closing.add(item.value)
			total.debit.add(item.value)
			total.closing.add(item.value)
		} else {
			entrySums.credit.add(item.value)
			entrySums.closing.add(item.value)
			total.credit.add(item.value)
			total.closing.add(item.value)
		}
		sums[index] = entrySums
	}
	for index := range result {
		amounts, err := formatAmounts(sums[index])
		if err != nil {
			return nil, Amounts{}, err
		}
		result[index].Amounts = amounts
	}
	amounts, err := formatAmounts(total)
	return result, amounts, err
}

func incomeStatementContributionEntries(items []provenanceContribution, query DrillDownQuery) ([]IncomeStatementDrillDownEntry, Balance, error) {
	result := []IncomeStatementDrillDownEntry{}
	indexes := map[string]int{}
	sums := []accumulator{}
	total := accumulator{}
	for _, item := range items {
		if !contributionMatches(item, query) {
			continue
		}
		index, found := indexes[item.entry.ID]
		if !found {
			index = len(result)
			indexes[item.entry.ID] = index
			result = append(result, IncomeStatementDrillDownEntry{
				ID: item.entry.ID, OccurredAt: item.entry.Entry.Date.String(), Description: item.entry.Entry.Description,
				Contributions: []PostingContribution{},
			})
			sums = append(sums, accumulator{})
		}
		result[index].Contributions = append(result[index].Contributions, postingContribution(item))
		sum := sums[index]
		sum.add(item.value)
		sums[index] = sum
		total.add(item.value)
	}
	for index := range result {
		balance, err := formatBalance(sums[index])
		if err != nil {
			return nil, Balance{}, err
		}
		result[index].Balance = balance
	}
	balance, err := formatBalance(total)
	return result, balance, err
}

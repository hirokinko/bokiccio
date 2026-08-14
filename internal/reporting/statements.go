package reporting

import (
	"sort"
	"time"
)

type StatementAccountRow struct {
	Account  string    `json:"account"`
	Label    string    `json:"label"`
	Depth    int       `json:"depth"`
	Direct   Balance   `json:"direct"`
	Subtotal Balance   `json:"subtotal"`
	Warnings []Warning `json:"warnings"`
}

type StatementCategoryGroup struct {
	Category Category              `json:"category"`
	Total    Balance               `json:"total"`
	Accounts []StatementAccountRow `json:"accounts"`
}

type StatementCommoditySection struct {
	Commodity string                   `json:"commodity"`
	Total     Balance                  `json:"total"`
	Groups    []StatementCategoryGroup `json:"groups"`
}

type BalanceSheet struct {
	ConfigurationRevision  int                         `json:"configuration_revision"`
	FiscalYear             Period                      `json:"fiscal_year"`
	AsOf                   string                      `json:"as_of"`
	ClassificationComplete bool                        `json:"classification_complete"`
	Warnings               []Warning                   `json:"warnings"`
	Commodities            []StatementCommoditySection `json:"commodities"`
}

type IncomeStatementCommodity struct {
	StatementCommoditySection
	NetIncome Balance `json:"net_income"`
}

type IncomeStatement struct {
	ConfigurationRevision  int                        `json:"configuration_revision"`
	Period                 FiscalPeriod               `json:"period"`
	ClassificationComplete bool                       `json:"classification_complete"`
	Warnings               []Warning                  `json:"warnings"`
	Commodities            []IncomeStatementCommodity `json:"commodities"`
}

type BalanceTrendPoint struct {
	Period                 FiscalPeriod                `json:"period"`
	ClassificationComplete bool                        `json:"classification_complete"`
	Warnings               []Warning                   `json:"warnings"`
	Commodities            []StatementCommoditySection `json:"commodities"`
}

type BalanceTrend struct {
	ConfigurationRevision  int                 `json:"configuration_revision"`
	FiscalYear             Period              `json:"fiscal_year"`
	ClassificationComplete bool                `json:"classification_complete"`
	Warnings               []Warning           `json:"warnings"`
	Points                 []BalanceTrendPoint `json:"points"`
}

type CurrentOverview struct {
	ConfigurationRevision  int                         `json:"configuration_revision"`
	AsOf                   string                      `json:"as_of"`
	FiscalYear             Period                      `json:"fiscal_year"`
	ExpensePeriod          Period                      `json:"expense_period"`
	ClassificationComplete bool                        `json:"classification_complete"`
	Warnings               []Warning                   `json:"warnings"`
	Balances               []StatementCommoditySection `json:"balances"`
	Expenses               []StatementCommoditySection `json:"expenses"`
}

func BuildCurrentOverview(configuration Configuration, entries []Entry, asOf string, expensePeriod Period) (CurrentOverview, error) {
	if err := ValidateConfiguration(configuration); err != nil {
		return CurrentOverview{}, err
	}
	parsed, err := time.Parse(time.DateOnly, asOf)
	if err != nil || parsed.Format(time.DateOnly) != asOf {
		return CurrentOverview{}, ErrInvalidPeriod
	}
	years := sortedFiscalYears(configuration.FiscalYears)
	targetIndex := -1
	for index, year := range years {
		if asOf >= year.StartDate && asOf <= year.EndDate {
			targetIndex = index
			break
		}
	}
	if targetIndex < 0 {
		return CurrentOverview{}, ErrInvalidPeriod
	}
	validated, err := validateEntries(entries)
	if err != nil {
		return CurrentOverview{}, err
	}
	classifier := newClassifier(configuration.Classifications)
	opening, err := buildFiscalOpening(years, targetIndex, validated, entries, classifier)
	if err != nil {
		return CurrentOverview{}, err
	}
	year := years[targetIndex]
	current := cloneAmounts(opening)
	if err := addMovements(current, entries, year.StartDate, asOf, year.OpeningEntryIDs, classifier); err != nil {
		return CurrentOverview{}, err
	}
	balances, balanceWarnings, balanceComplete, err := assembleStatementCommodities(
		filterAmounts(current, CategoryAsset, CategoryLiability, CategoryEquity, CategoryUnknown),
	)
	if err != nil {
		return CurrentOverview{}, err
	}
	expenseFiscalPeriod, expenseYearIndex, err := selectPeriod(configuration.StartMonth, years, expensePeriod)
	if err != nil || expenseFiscalPeriod.Month == 0 {
		return CurrentOverview{}, ErrInvalidPeriod
	}
	movement, err := collectMovements(entries, expensePeriod.StartDate, expensePeriod.EndDate,
		years[expenseYearIndex].OpeningEntryIDs, classifier)
	if err != nil {
		return CurrentOverview{}, err
	}
	expenses, expenseWarnings, expenseComplete, err := assembleStatementCommodities(
		filterAmounts(netMovements(movement), CategoryExpense),
	)
	if err != nil {
		return CurrentOverview{}, err
	}
	warnings := append(balanceWarnings, expenseWarnings...)
	return CurrentOverview{
		ConfigurationRevision:  configuration.Revision,
		AsOf:                   asOf,
		FiscalYear:             Period{StartDate: year.StartDate, EndDate: year.EndDate},
		ExpensePeriod:          expensePeriod,
		ClassificationComplete: balanceComplete && expenseComplete,
		Warnings:               warnings,
		Balances:               balances,
		Expenses:               expenses,
	}, nil
}

func BuildBalanceSheet(configuration Configuration, entries []Entry, selected Period) (BalanceSheet, error) {
	years, targetIndex, validated, classifier, err := statementInputs(configuration, entries, selected, true)
	if err != nil {
		return BalanceSheet{}, err
	}
	opening, err := buildFiscalOpening(years, targetIndex, validated, entries, classifier)
	if err != nil {
		return BalanceSheet{}, err
	}
	if years[targetIndex].OpeningMode == OpeningAutomatic && !amountsBalanced(opening) {
		return BalanceSheet{}, ErrOpeningUnbalanced
	}
	amounts := filterAmounts(opening, CategoryAsset, CategoryLiability, CategoryEquity, CategoryUnknown)
	commodities, warnings, complete, err := assembleStatementCommodities(amounts)
	if err != nil {
		return BalanceSheet{}, err
	}
	return BalanceSheet{
		ConfigurationRevision:  configuration.Revision,
		FiscalYear:             selected,
		AsOf:                   selected.StartDate,
		ClassificationComplete: complete,
		Warnings:               warnings,
		Commodities:            commodities,
	}, nil
}

func BuildIncomeStatement(configuration Configuration, entries []Entry, selected Period) (IncomeStatement, error) {
	years, targetIndex, _, classifier, err := statementInputs(configuration, entries, selected, false)
	if err != nil {
		return IncomeStatement{}, err
	}
	period, _, err := selectPeriod(configuration.StartMonth, years, selected)
	if err != nil || period.Month == 0 {
		return IncomeStatement{}, ErrInvalidPeriod
	}
	movement, err := collectMovements(entries, selected.StartDate, selected.EndDate, years[targetIndex].OpeningEntryIDs, classifier)
	if err != nil {
		return IncomeStatement{}, err
	}
	amounts := filterAmounts(netMovements(movement), CategoryRevenue, CategoryExpense, CategoryUnknown)
	sections, warnings, complete, err := assembleStatementCommodities(amounts)
	if err != nil {
		return IncomeStatement{}, err
	}
	commodities := make([]IncomeStatementCommodity, 0, len(sections))
	for _, section := range sections {
		netIncome, err := categoryBalance(section, CategoryRevenue, CategoryExpense)
		if err != nil {
			return IncomeStatement{}, err
		}
		commodities = append(commodities, IncomeStatementCommodity{
			StatementCommoditySection: section,
			NetIncome:                 netIncome,
		})
	}
	return IncomeStatement{
		ConfigurationRevision:  configuration.Revision,
		Period:                 period,
		ClassificationComplete: complete,
		Warnings:               warnings,
		Commodities:            commodities,
	}, nil
}

func BuildBalanceTrend(configuration Configuration, entries []Entry, selected Period) (BalanceTrend, error) {
	years, targetIndex, validated, classifier, err := statementInputs(configuration, entries, selected, true)
	if err != nil {
		return BalanceTrend{}, err
	}
	opening, err := buildFiscalOpening(years, targetIndex, validated, entries, classifier)
	if err != nil {
		return BalanceTrend{}, err
	}
	if years[targetIndex].OpeningMode == OpeningAutomatic && !amountsBalanced(opening) {
		return BalanceTrend{}, ErrOpeningUnbalanced
	}
	periods, err := FiscalPeriods(years[targetIndex], configuration.StartMonth)
	if err != nil {
		return BalanceTrend{}, err
	}
	pointAmounts, err := monthlyClosingAmounts(opening, entries, periods[1:], years[targetIndex], classifier)
	if err != nil {
		return BalanceTrend{}, err
	}
	alignStatementAmounts(pointAmounts)
	report := BalanceTrend{
		ConfigurationRevision:  configuration.Revision,
		FiscalYear:             selected,
		ClassificationComplete: true,
		Warnings:               []Warning{},
		Points:                 make([]BalanceTrendPoint, 0, len(periods)-1),
	}
	for index, period := range periods[1:] {
		if !amountsBalanced(pointAmounts[index]) {
			return BalanceTrend{}, ErrOpeningUnbalanced
		}
		sections, warnings, complete, err := assembleStatementCommodities(pointAmounts[index])
		if err != nil {
			return BalanceTrend{}, err
		}
		report.Points = append(report.Points, BalanceTrendPoint{
			Period: period, ClassificationComplete: complete, Warnings: warnings, Commodities: sections,
		})
		for _, warning := range warnings {
			warning.PeriodEnd = period.EndDate
			report.Warnings = append(report.Warnings, warning)
		}
		report.ClassificationComplete = report.ClassificationComplete && complete
	}
	return report, nil
}

func statementInputs(configuration Configuration, entries []Entry, selected Period, fiscalYearOnly bool) ([]FiscalYear, int, map[string]Entry, classifier, error) {
	if err := ValidateConfiguration(configuration); err != nil {
		return nil, 0, nil, classifier{}, err
	}
	years := sortedFiscalYears(configuration.FiscalYears)
	period, targetIndex, err := selectPeriod(configuration.StartMonth, years, selected)
	if err != nil || (fiscalYearOnly && period.Month != 0) || (!fiscalYearOnly && period.Month == 0) {
		return nil, 0, nil, classifier{}, ErrInvalidPeriod
	}
	validated, err := validateEntries(entries)
	if err != nil {
		return nil, 0, nil, classifier{}, err
	}
	return years, targetIndex, validated, newClassifier(configuration.Classifications), nil
}

func netMovements(movement movements) amountMap {
	result := cloneAmounts(movement.debits)
	for key, value := range movement.credits {
		result[key] = result[key].addAccumulator(value)
	}
	return result
}

func filterAmounts(source amountMap, categories ...Category) amountMap {
	allowed := make(map[Category]struct{}, len(categories))
	for _, category := range categories {
		allowed[category] = struct{}{}
	}
	result := amountMap{}
	for key, value := range source {
		if _, found := allowed[key.category]; found {
			result[key] = value.clone()
		}
	}
	return result
}

func monthlyClosingAmounts(opening amountMap, entries []Entry, periods []FiscalPeriod, year FiscalYear, classifier classifier) ([]amountMap, error) {
	excluded := make(map[string]struct{}, len(year.OpeningEntryIDs))
	for _, id := range year.OpeningEntryIDs {
		excluded[id] = struct{}{}
	}
	sorted := append([]Entry(nil), entries...)
	sort.SliceStable(sorted, func(i, j int) bool {
		left, right := sorted[i].Entry.Date.String()[:10], sorted[j].Entry.Date.String()[:10]
		if left == right {
			return sorted[i].ID < sorted[j].ID
		}
		return left < right
	})
	running := cloneAmounts(opening)
	result := make([]amountMap, 0, len(periods))
	entryIndex := 0
	for _, period := range periods {
		for entryIndex < len(sorted) {
			item := sorted[entryIndex]
			date := item.Entry.Date.String()[:10]
			if date > period.EndDate {
				break
			}
			entryIndex++
			if date < year.StartDate || date > year.EndDate {
				continue
			}
			if _, found := excluded[item.ID]; found {
				continue
			}
			if err := running.addEntry(item.Entry, classifier, false); err != nil {
				return nil, err
			}
		}
		result = append(result, cloneAmounts(running))
	}
	return result, nil
}

func alignStatementAmounts(points []amountMap) {
	keys := map[amountKey]struct{}{}
	for _, point := range points {
		for key := range point {
			keys[key] = struct{}{}
		}
	}
	for _, point := range points {
		for key := range keys {
			if _, found := point[key]; !found {
				point[key] = accumulator{}
			}
		}
	}
}

func amountsBalanced(amounts amountMap) bool {
	totals := map[string]accumulator{}
	for key, value := range amounts {
		totals[key.commodity] = totals[key.commodity].addAccumulator(value)
	}
	for _, total := range totals {
		if total.sign() != 0 {
			return false
		}
	}
	return true
}

func assembleStatementCommodities(amounts amountMap) ([]StatementCommoditySection, []Warning, bool, error) {
	commoditySet := map[string]struct{}{}
	for key := range amounts {
		commoditySet[key.commodity] = struct{}{}
	}
	commodities := make([]string, 0, len(commoditySet))
	for commodity := range commoditySet {
		commodities = append(commodities, commodity)
	}
	sort.Strings(commodities)
	sections := make([]StatementCommoditySection, 0, len(commodities))
	warnings := []Warning{}
	complete := true
	for _, commodity := range commodities {
		section, sectionWarnings, sectionComplete, err := buildStatementCommodity(commodity, amounts)
		if err != nil {
			return nil, nil, false, err
		}
		sections = append(sections, section)
		warnings = append(warnings, sectionWarnings...)
		complete = complete && sectionComplete
	}
	return sections, warnings, complete, nil
}

func buildStatementCommodity(commodity string, amounts amountMap) (StatementCommoditySection, []Warning, bool, error) {
	section := StatementCommoditySection{Commodity: commodity, Groups: []StatementCategoryGroup{}}
	warnings := []Warning{}
	complete := true
	sectionTotal := accumulator{}
	for _, category := range categoryOrder {
		direct := map[string]accumulator{}
		for key, value := range amounts {
			if key.commodity == commodity && key.category == category {
				direct[key.account] = direct[key.account].addAccumulator(value)
			}
		}
		if len(direct) == 0 {
			continue
		}
		group, groupWarnings, err := buildStatementCategoryGroup(commodity, category, direct)
		if err != nil {
			return StatementCommoditySection{}, nil, false, err
		}
		section.Groups = append(section.Groups, group)
		warnings = append(warnings, groupWarnings...)
		sectionTotal = sectionTotal.addAccumulator(parseBalance(group.Total))
		if category == CategoryUnknown {
			complete = false
		}
	}
	var err error
	section.Total, err = formatBalance(sectionTotal)
	return section, warnings, complete, err
}

func buildStatementCategoryGroup(commodity string, category Category, direct map[string]accumulator) (StatementCategoryGroup, []Warning, error) {
	nodes := map[string]accumulator{}
	for account, value := range direct {
		parts := splitAccount(account)
		for index := range parts {
			prefix := joinAccount(parts[:index+1])
			nodes[prefix] = nodes[prefix].addAccumulator(value)
		}
	}
	accounts := make([]string, 0, len(nodes))
	for account := range nodes {
		accounts = append(accounts, account)
	}
	sort.Slice(accounts, func(i, j int) bool { return accountLess(accounts[i], accounts[j]) })
	group := StatementCategoryGroup{Category: category, Accounts: []StatementAccountRow{}}
	warnings := []Warning{}
	groupTotal := accumulator{}
	for _, account := range accounts {
		directValue, directlyPosted := direct[account]
		rowWarnings := statementWarnings(commodity, account, category, directValue, directlyPosted)
		directBalance, err := formatBalance(directValue)
		if err != nil {
			return StatementCategoryGroup{}, nil, err
		}
		subtotalBalance, err := formatBalance(nodes[account])
		if err != nil {
			return StatementCategoryGroup{}, nil, err
		}
		parts := splitAccount(account)
		group.Accounts = append(group.Accounts, StatementAccountRow{
			Account: account, Label: parts[len(parts)-1], Depth: len(parts) - 1,
			Direct: directBalance, Subtotal: subtotalBalance, Warnings: rowWarnings,
		})
		warnings = append(warnings, rowWarnings...)
		if len(parts) == 1 {
			groupTotal = groupTotal.addAccumulator(nodes[account])
		}
	}
	var err error
	group.Total, err = formatBalance(groupTotal)
	return group, warnings, err
}

func splitAccount(account string) []string {
	parts := []string{}
	start := 0
	for index := range account {
		if account[index] == ':' {
			parts = append(parts, account[start:index])
			start = index + 1
		}
	}
	return append(parts, account[start:])
}

func joinAccount(parts []string) string {
	if len(parts) == 0 {
		return ""
	}
	result := parts[0]
	for _, part := range parts[1:] {
		result += ":" + part
	}
	return result
}

func accountLess(left, right string) bool {
	leftParts, rightParts := splitAccount(left), splitAccount(right)
	limit := len(leftParts)
	if len(rightParts) < limit {
		limit = len(rightParts)
	}
	for index := 0; index < limit; index++ {
		if leftParts[index] != rightParts[index] {
			return leftParts[index] < rightParts[index]
		}
	}
	return len(leftParts) < len(rightParts)
}

func statementWarnings(commodity, account string, category Category, value accumulator, directlyPosted bool) []Warning {
	if !directlyPosted {
		return []Warning{}
	}
	if category == CategoryUnknown {
		return []Warning{{Code: "unclassified_account", Account: account, Commodity: commodity}}
	}
	if value.sign() == 0 || normalSide(category) == side(value) {
		return []Warning{}
	}
	return []Warning{{
		Code: "opposite_normal_balance", Account: account, Commodity: commodity, Balance: "statement", Side: side(value),
	}}
}

func categoryBalance(section StatementCommoditySection, categories ...Category) (Balance, error) {
	total := accumulator{}
	for _, group := range section.Groups {
		for _, category := range categories {
			if group.Category == category {
				total = total.addAccumulator(parseBalance(group.Total))
			}
		}
	}
	return formatBalance(total)
}

// Package reporting calculates financial reports from validated ledger entries.
package reporting

import (
	"errors"
	"fmt"
	"math/big"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/hirokinko/bokiccio/internal/ledger"
)

var (
	ErrInvalidConfiguration = errors.New("invalid reporting configuration")
	ErrInvalidPeriod        = errors.New("invalid reporting period")
	ErrInvalidEntry         = errors.New("invalid reporting entry")
	ErrAmountOverflow       = errors.New("reporting amount exceeds decimal range")
)

type Category string

const (
	CategoryAsset     Category = "asset"
	CategoryLiability Category = "liability"
	CategoryEquity    Category = "equity"
	CategoryRevenue   Category = "revenue"
	CategoryExpense   Category = "expense"
	CategoryUnknown   Category = "unclassified"
)

var categoryOrder = []Category{
	CategoryAsset,
	CategoryLiability,
	CategoryEquity,
	CategoryRevenue,
	CategoryExpense,
	CategoryUnknown,
}

type OpeningMode string

const (
	OpeningAutomatic OpeningMode = "automatic"
	OpeningEntries   OpeningMode = "opening_entries"
)

type Classification struct {
	Account  string
	Category Category
}

type FiscalYear struct {
	StartDate       string
	EndDate         string
	OpeningMode     OpeningMode
	OpeningEntryIDs []string
}

type Configuration struct {
	Revision        int
	StartMonth      int
	Classifications []Classification
	FiscalYears     []FiscalYear
}

type Entry struct {
	ID    string
	Entry ledger.JournalEntry
}

type Period struct {
	StartDate string `json:"start_date"`
	EndDate   string `json:"end_date"`
}

type FiscalPeriod struct {
	Period
	FiscalYearStart string `json:"fiscal_year_start"`
	FiscalYearEnd   string `json:"fiscal_year_end"`
	Month           int    `json:"month,omitempty"`
}

type Warning struct {
	Code      string `json:"code"`
	Account   string `json:"account"`
	Commodity string `json:"commodity"`
	Balance   string `json:"balance,omitempty"`
	Side      string `json:"side,omitempty"`
}

type Balance struct {
	Debit  string `json:"debit"`
	Credit string `json:"credit"`
}

type Amounts struct {
	Opening        Balance `json:"opening"`
	DebitTurnover  string  `json:"debit_turnover"`
	CreditTurnover string  `json:"credit_turnover"`
	Closing        Balance `json:"closing"`
}

type AccountRow struct {
	Account  string    `json:"account"`
	Label    string    `json:"label"`
	Depth    int       `json:"depth"`
	Direct   Amounts   `json:"direct"`
	Subtotal Amounts   `json:"subtotal"`
	Warnings []Warning `json:"warnings"`
}

type CategoryGroup struct {
	Category Category     `json:"category"`
	Total    Amounts      `json:"total"`
	Accounts []AccountRow `json:"accounts"`
}

type CommoditySection struct {
	Commodity string          `json:"commodity"`
	Total     Amounts         `json:"total"`
	Groups    []CategoryGroup `json:"groups"`
}

type TrialBalance struct {
	ConfigurationRevision  int                `json:"configuration_revision"`
	Period                 FiscalPeriod       `json:"period"`
	ClassificationComplete bool               `json:"classification_complete"`
	Warnings               []Warning          `json:"warnings"`
	Commodities            []CommoditySection `json:"commodities"`
}

func ValidateConfiguration(configuration Configuration) error {
	if configuration.Revision < 1 || configuration.StartMonth < 1 || configuration.StartMonth > 12 || len(configuration.FiscalYears) == 0 {
		return ErrInvalidConfiguration
	}
	classifications := append([]Classification(nil), configuration.Classifications...)
	sort.Slice(classifications, func(i, j int) bool { return classifications[i].Account < classifications[j].Account })
	for index, classification := range classifications {
		if err := ledger.ValidateAccount(classification.Account); err != nil || !validCategory(classification.Category) {
			return ErrInvalidConfiguration
		}
		for previous := 0; previous < index; previous++ {
			ancestor := classifications[previous].Account
			if classification.Account == ancestor || strings.HasPrefix(classification.Account, ancestor+":") {
				return ErrInvalidConfiguration
			}
		}
	}
	years := append([]FiscalYear(nil), configuration.FiscalYears...)
	sort.Slice(years, func(i, j int) bool { return years[i].StartDate < years[j].StartDate })
	for index, year := range years {
		start, end, err := parseFiscalYear(year, configuration.StartMonth)
		if err != nil {
			return err
		}
		if index > 0 {
			previousEnd, _ := time.Parse(time.DateOnly, years[index-1].EndDate)
			if !previousEnd.AddDate(0, 0, 1).Equal(start) {
				return ErrInvalidConfiguration
			}
		}
		if year.OpeningMode != OpeningAutomatic && year.OpeningMode != OpeningEntries {
			return ErrInvalidConfiguration
		}
		if year.OpeningMode == OpeningEntries && len(year.OpeningEntryIDs) == 0 {
			return ErrInvalidConfiguration
		}
		if hasDuplicateOrEmpty(year.OpeningEntryIDs) || end.Before(start) {
			return ErrInvalidConfiguration
		}
	}
	return nil
}

func FiscalPeriods(year FiscalYear, startMonth int) ([]FiscalPeriod, error) {
	start, end, err := parseFiscalYear(year, startMonth)
	if err != nil {
		return nil, err
	}
	periods := make([]FiscalPeriod, 0, 13)
	periods = append(periods, FiscalPeriod{
		Period:          Period{StartDate: year.StartDate, EndDate: year.EndDate},
		FiscalYearStart: year.StartDate, FiscalYearEnd: year.EndDate,
	})
	for month := 1; month <= 12; month++ {
		monthStart := start.AddDate(0, month-1, 0)
		monthEnd := monthStart.AddDate(0, 1, -1)
		if monthEnd.After(end) {
			return nil, ErrInvalidConfiguration
		}
		periods = append(periods, FiscalPeriod{
			Period:          Period{StartDate: monthStart.Format(time.DateOnly), EndDate: monthEnd.Format(time.DateOnly)},
			FiscalYearStart: year.StartDate, FiscalYearEnd: year.EndDate, Month: month,
		})
	}
	return periods, nil
}

func BuildTrialBalance(configuration Configuration, entries []Entry, selected Period) (TrialBalance, error) {
	if err := ValidateConfiguration(configuration); err != nil {
		return TrialBalance{}, err
	}
	years := sortedFiscalYears(configuration.FiscalYears)
	period, targetIndex, err := selectPeriod(configuration.StartMonth, years, selected)
	if err != nil {
		return TrialBalance{}, err
	}
	validated, err := validateEntries(entries)
	if err != nil {
		return TrialBalance{}, err
	}
	classifier := newClassifier(configuration.Classifications)
	opening := amountMap{}
	for index := 0; index <= targetIndex; index++ {
		year := years[index]
		if year.OpeningMode == OpeningEntries {
			opening = amountMap{}
			for _, id := range year.OpeningEntryIDs {
				entry, found := validated[id]
				if !found || entry.Entry.Date.String()[:10] != year.StartDate {
					return TrialBalance{}, ErrInvalidConfiguration
				}
				if err := opening.addEntry(entry.Entry, classifier, true); err != nil {
					return TrialBalance{}, err
				}
			}
		} else if index == 0 {
			opening = amountMap{}
		} else {
			opening = opening.permanentOnly()
		}

		if index < targetIndex {
			if err := addMovements(opening, entries, year.StartDate, year.EndDate, year.OpeningEntryIDs, classifier); err != nil {
				return TrialBalance{}, err
			}
			continue
		}
		return buildSelected(configuration.Revision, period, opening, entries, year.OpeningEntryIDs, classifier)
	}
	return TrialBalance{}, ErrInvalidPeriod
}

func validCategory(category Category) bool {
	return category == CategoryAsset || category == CategoryLiability || category == CategoryEquity ||
		category == CategoryRevenue || category == CategoryExpense
}

func parseFiscalYear(year FiscalYear, startMonth int) (time.Time, time.Time, error) {
	start, err := time.Parse(time.DateOnly, year.StartDate)
	if err != nil || start.Format(time.DateOnly) != year.StartDate || int(start.Month()) != startMonth || start.Day() != 1 {
		return time.Time{}, time.Time{}, ErrInvalidConfiguration
	}
	end, err := time.Parse(time.DateOnly, year.EndDate)
	wantEnd := start.AddDate(1, 0, -1)
	if err != nil || end.Format(time.DateOnly) != year.EndDate || !end.Equal(wantEnd) {
		return time.Time{}, time.Time{}, ErrInvalidConfiguration
	}
	return start, end, nil
}

func hasDuplicateOrEmpty(values []string) bool {
	seen := map[string]struct{}{}
	for _, value := range values {
		if value == "" {
			return true
		}
		if _, found := seen[value]; found {
			return true
		}
		seen[value] = struct{}{}
	}
	return false
}

func selectPeriod(startMonth int, years []FiscalYear, selected Period) (FiscalPeriod, int, error) {
	for index, year := range years {
		periods, err := FiscalPeriods(year, startMonth)
		if err != nil {
			return FiscalPeriod{}, 0, err
		}
		for _, period := range periods {
			if period.StartDate == selected.StartDate && period.EndDate == selected.EndDate {
				return period, index, nil
			}
		}
	}
	return FiscalPeriod{}, 0, ErrInvalidPeriod
}

func sortedFiscalYears(years []FiscalYear) []FiscalYear {
	years = append([]FiscalYear(nil), years...)
	sort.Slice(years, func(i, j int) bool { return years[i].StartDate < years[j].StartDate })
	return years
}

func validateEntries(entries []Entry) (map[string]Entry, error) {
	result := make(map[string]Entry, len(entries))
	for _, item := range entries {
		if item.ID == "" {
			return nil, ErrInvalidEntry
		}
		if _, found := result[item.ID]; found {
			return nil, ErrInvalidEntry
		}
		if err := ledger.Validate(item.Entry); err != nil {
			return nil, fmt.Errorf("%w: %v", ErrInvalidEntry, err)
		}
		result[item.ID] = item
	}
	return result, nil
}

type classifier struct {
	items []Classification
}

func newClassifier(items []Classification) classifier {
	items = append([]Classification(nil), items...)
	sort.Slice(items, func(i, j int) bool { return items[i].Account < items[j].Account })
	return classifier{items: items}
}

func (classifier classifier) category(account string) Category {
	for _, item := range classifier.items {
		if account == item.Account || strings.HasPrefix(account, item.Account+":") {
			return item.Category
		}
	}
	return CategoryUnknown
}

type amountKey struct {
	commodity string
	category  Category
	account   string
}

type accumulator struct {
	coefficient *big.Int
	scale       uint8
}

func (sum *accumulator) add(value ledger.Decimal) {
	coefficient, scale := decimalParts(value)
	if sum.coefficient == nil {
		sum.coefficient = coefficient
		sum.scale = scale
		return
	}
	if sum.scale < scale {
		sum.coefficient.Mul(sum.coefficient, power10(scale-sum.scale))
		sum.scale = scale
	} else if scale < sum.scale {
		coefficient.Mul(coefficient, power10(sum.scale-scale))
	}
	sum.coefficient.Add(sum.coefficient, coefficient)
}

func (sum accumulator) addAccumulator(other accumulator) accumulator {
	result := sum.clone()
	if other.coefficient == nil {
		return result
	}
	coefficient := new(big.Int).Set(other.coefficient)
	if result.coefficient == nil {
		return other.clone()
	}
	if result.scale < other.scale {
		result.coefficient.Mul(result.coefficient, power10(other.scale-result.scale))
		result.scale = other.scale
	} else if other.scale < result.scale {
		coefficient.Mul(coefficient, power10(result.scale-other.scale))
	}
	result.coefficient.Add(result.coefficient, coefficient)
	return result
}

func (sum accumulator) clone() accumulator {
	if sum.coefficient == nil {
		return accumulator{}
	}
	return accumulator{coefficient: new(big.Int).Set(sum.coefficient), scale: sum.scale}
}

func (sum accumulator) sign() int {
	if sum.coefficient == nil {
		return 0
	}
	return sum.coefficient.Sign()
}

func (sum accumulator) absoluteString() (string, error) {
	if sum.coefficient == nil {
		return "0", nil
	}
	absolute := new(big.Int).Abs(new(big.Int).Set(sum.coefficient))
	if sum.scale > ledger.MaxDecimalScale || absolute.BitLen() > 96 {
		return "", ErrAmountOverflow
	}
	return formatDecimal(absolute, sum.scale), nil
}

func decimalParts(value ledger.Decimal) (*big.Int, uint8) {
	text := value.String()
	negative := strings.HasPrefix(text, "-")
	text = strings.TrimPrefix(text, "-")
	text = strings.ReplaceAll(text, ".", "")
	coefficient, _ := new(big.Int).SetString(text, 10)
	if negative {
		coefficient.Neg(coefficient)
	}
	return coefficient, value.Scale()
}

func power10(scale uint8) *big.Int {
	return new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(scale)), nil)
}

func formatDecimal(coefficient *big.Int, scale uint8) string {
	digits := coefficient.String()
	if scale == 0 {
		return digits
	}
	if len(digits) <= int(scale) {
		digits = strings.Repeat("0", int(scale)-len(digits)+1) + digits
	}
	split := len(digits) - int(scale)
	return digits[:split] + "." + digits[split:]
}

type amountMap map[amountKey]accumulator

func (amounts amountMap) add(key amountKey, value ledger.Decimal) {
	sum := amounts[key]
	sum.add(value)
	amounts[key] = sum
}

func (amounts amountMap) addEntry(entry ledger.JournalEntry, classifier classifier, permanentOnly bool) error {
	return forEachEntryAmount(entry, classifier, permanentOnly, amounts.add)
}

func forEachEntryAmount(entry ledger.JournalEntry, classifier classifier, permanentOnly bool, add func(amountKey, ledger.Decimal)) error {
	inferred, err := inferredAmount(entry)
	if err != nil {
		return err
	}
	for _, posting := range entry.Postings {
		category := classifier.category(posting.Account)
		if permanentOnly && category != CategoryAsset && category != CategoryLiability && category != CategoryEquity {
			return ErrInvalidConfiguration
		}
		amount := posting.Amount
		if posting.TotalPrice != nil {
			amount = posting.TotalPrice
		} else if amount == nil {
			amount = &inferred
		}
		if amount == nil {
			return ErrInvalidEntry
		}
		add(amountKey{commodity: string(amount.Commodity), category: category, account: posting.Account}, amount.Value)
	}
	return nil
}

func inferredAmount(entry ledger.JournalEntry) (ledger.Amount, error) {
	if entry.Postings[len(entry.Postings)-1].Amount != nil {
		return ledger.Amount{}, nil
	}
	amount, err := ledger.InferFinalAmount(entry)
	if err != nil {
		return ledger.Amount{}, fmt.Errorf("%w: %v", ErrInvalidEntry, err)
	}
	return amount, nil
}

func (amounts amountMap) permanentOnly() amountMap {
	result := amountMap{}
	for key, value := range amounts {
		if key.category == CategoryAsset || key.category == CategoryLiability || key.category == CategoryEquity {
			result[key] = value.clone()
		}
	}
	return result
}

func addMovements(target amountMap, entries []Entry, startDate, endDate string, excluded []string, classifier classifier) error {
	exclude := make(map[string]struct{}, len(excluded))
	for _, id := range excluded {
		exclude[id] = struct{}{}
	}
	for _, item := range entries {
		if _, found := exclude[item.ID]; found {
			continue
		}
		date := item.Entry.Date.String()[:10]
		if date < startDate || date > endDate {
			continue
		}
		if err := target.addEntry(item.Entry, classifier, false); err != nil {
			return err
		}
	}
	return nil
}

type rowSums struct {
	opening accumulator
	debit   accumulator
	credit  accumulator
	closing accumulator
}

type movements struct {
	debits  amountMap
	credits amountMap
}

func buildSelected(revision int, period FiscalPeriod, fiscalOpening amountMap, entries []Entry, excluded []string, classifier classifier) (TrialBalance, error) {
	opening := cloneAmounts(fiscalOpening)
	if period.StartDate != period.FiscalYearStart {
		previous := dateBefore(period.StartDate)
		if err := addMovements(opening, entries, period.FiscalYearStart, previous, excluded, classifier); err != nil {
			return TrialBalance{}, err
		}
	}
	movement, err := collectMovements(entries, period.StartDate, period.EndDate, excluded, classifier)
	if err != nil {
		return TrialBalance{}, err
	}
	return assembleTrialBalance(revision, period, opening, movement)
}

func collectMovements(entries []Entry, startDate, endDate string, excluded []string, classifier classifier) (movements, error) {
	result := movements{debits: amountMap{}, credits: amountMap{}}
	exclude := make(map[string]struct{}, len(excluded))
	for _, id := range excluded {
		exclude[id] = struct{}{}
	}
	for _, item := range entries {
		if _, found := exclude[item.ID]; found {
			continue
		}
		date := item.Entry.Date.String()[:10]
		if date < startDate || date > endDate {
			continue
		}
		err := forEachEntryAmount(item.Entry, classifier, false, func(key amountKey, value ledger.Decimal) {
			if value.Sign() >= 0 {
				result.debits.add(key, value)
			} else {
				result.credits.add(key, value)
			}
		})
		if err != nil {
			return movements{}, err
		}
	}
	return result, nil
}

func cloneAmounts(source amountMap) amountMap {
	result := amountMap{}
	for key, value := range source {
		result[key] = value.clone()
	}
	return result
}

func dateBefore(date string) string {
	parsed, _ := time.Parse(time.DateOnly, date)
	return parsed.AddDate(0, 0, -1).Format(time.DateOnly)
}

func assembleTrialBalance(revision int, period FiscalPeriod, opening amountMap, movement movements) (TrialBalance, error) {
	report := TrialBalance{ConfigurationRevision: revision, Period: period, ClassificationComplete: true, Warnings: []Warning{}, Commodities: []CommoditySection{}}
	commodities := map[string]struct{}{}
	for key := range opening {
		commodities[key.commodity] = struct{}{}
	}
	for key := range movement.debits {
		commodities[key.commodity] = struct{}{}
	}
	for key := range movement.credits {
		commodities[key.commodity] = struct{}{}
	}
	commodityNames := make([]string, 0, len(commodities))
	for commodity := range commodities {
		commodityNames = append(commodityNames, commodity)
	}
	sort.Strings(commodityNames)
	for _, commodity := range commodityNames {
		section, warnings, complete, err := buildCommodity(commodity, opening, movement)
		if err != nil {
			return TrialBalance{}, err
		}
		report.Commodities = append(report.Commodities, section)
		report.Warnings = append(report.Warnings, warnings...)
		report.ClassificationComplete = report.ClassificationComplete && complete
	}
	return report, nil
}

func buildCommodity(commodity string, opening amountMap, movement movements) (CommoditySection, []Warning, bool, error) {
	section := CommoditySection{Commodity: commodity, Groups: []CategoryGroup{}}
	warnings := []Warning{}
	complete := true
	sectionSums := rowSums{}
	for _, category := range categoryOrder {
		direct := map[string]rowSums{}
		for key, value := range opening {
			if key.commodity == commodity && key.category == category {
				row := direct[key.account]
				row.opening = row.opening.addAccumulator(value)
				row.closing = row.closing.addAccumulator(value)
				direct[key.account] = row
			}
		}
		for key, value := range movement.debits {
			if key.commodity != commodity || key.category != category {
				continue
			}
			row := direct[key.account]
			row.debit = row.debit.addAccumulator(value)
			row.closing = row.closing.addAccumulator(value)
			direct[key.account] = row
		}
		for key, value := range movement.credits {
			if key.commodity != commodity || key.category != category {
				continue
			}
			row := direct[key.account]
			row.credit = row.credit.addAccumulator(value)
			row.closing = row.closing.addAccumulator(value)
			direct[key.account] = row
		}
		if len(direct) == 0 {
			continue
		}
		group, groupWarnings, err := buildCategoryGroup(commodity, category, direct)
		if err != nil {
			return CommoditySection{}, nil, false, err
		}
		section.Groups = append(section.Groups, group)
		warnings = append(warnings, groupWarnings...)
		if category == CategoryUnknown {
			complete = false
		}
		sectionSums = addRowSums(sectionSums, sumsFromAmounts(group.Total))
	}
	var err error
	section.Total, err = formatAmounts(sectionSums)
	return section, warnings, complete, err
}

func buildCategoryGroup(commodity string, category Category, direct map[string]rowSums) (CategoryGroup, []Warning, error) {
	nodes := map[string]rowSums{}
	for account, sums := range direct {
		parts := strings.Split(account, ":")
		for index := range parts {
			prefix := strings.Join(parts[:index+1], ":")
			nodes[prefix] = addRowSums(nodes[prefix], sums)
		}
	}
	accounts := make([]string, 0, len(nodes))
	for account := range nodes {
		accounts = append(accounts, account)
	}
	sort.Slice(accounts, func(i, j int) bool {
		left := strings.Split(accounts[i], ":")
		right := strings.Split(accounts[j], ":")
		return slices.Compare(left, right) < 0
	})
	group := CategoryGroup{Category: category, Accounts: make([]AccountRow, 0, len(accounts))}
	warnings := []Warning{}
	groupSums := rowSums{}
	for _, account := range accounts {
		directSums := direct[account]
		subtotalSums := nodes[account]
		directAmounts, err := formatAmounts(directSums)
		if err != nil {
			return CategoryGroup{}, nil, err
		}
		subtotalAmounts, err := formatAmounts(subtotalSums)
		if err != nil {
			return CategoryGroup{}, nil, err
		}
		_, directlyPosted := direct[account]
		rowWarnings := warningsForRow(commodity, account, category, directSums, directlyPosted)
		warnings = append(warnings, rowWarnings...)
		parts := strings.Split(account, ":")
		group.Accounts = append(group.Accounts, AccountRow{
			Account: account, Label: parts[len(parts)-1], Depth: len(parts) - 1,
			Direct: directAmounts, Subtotal: subtotalAmounts, Warnings: rowWarnings,
		})
		if len(parts) == 1 {
			groupSums = addRowSums(groupSums, subtotalSums)
		}
	}
	var err error
	group.Total, err = formatAmounts(groupSums)
	return group, warnings, err
}

func addRowSums(left, right rowSums) rowSums {
	return rowSums{
		opening: left.opening.addAccumulator(right.opening),
		debit:   left.debit.addAccumulator(right.debit),
		credit:  left.credit.addAccumulator(right.credit),
		closing: left.closing.addAccumulator(right.closing),
	}
}

func formatAmounts(sums rowSums) (Amounts, error) {
	opening, err := formatBalance(sums.opening)
	if err != nil {
		return Amounts{}, err
	}
	debit, err := sums.debit.absoluteString()
	if err != nil {
		return Amounts{}, err
	}
	credit, err := sums.credit.absoluteString()
	if err != nil {
		return Amounts{}, err
	}
	closing, err := formatBalance(sums.closing)
	if err != nil {
		return Amounts{}, err
	}
	return Amounts{Opening: opening, DebitTurnover: debit, CreditTurnover: credit, Closing: closing}, nil
}

func formatBalance(value accumulator) (Balance, error) {
	text, err := value.absoluteString()
	if err != nil {
		return Balance{}, err
	}
	if value.sign() < 0 {
		return Balance{Debit: "0", Credit: text}, nil
	}
	return Balance{Debit: text, Credit: "0"}, nil
}

func sumsFromAmounts(amounts Amounts) rowSums {
	return rowSums{
		opening: parseBalance(amounts.Opening), debit: parseAccumulator(amounts.DebitTurnover),
		credit: negateAccumulator(parseAccumulator(amounts.CreditTurnover)), closing: parseBalance(amounts.Closing),
	}
}

func parseBalance(balance Balance) accumulator {
	if balance.Credit != "0" {
		return negateAccumulator(parseAccumulator(balance.Credit))
	}
	return parseAccumulator(balance.Debit)
}

func parseAccumulator(text string) accumulator {
	value, _ := ledger.ParseDecimal(text)
	coefficient, scale := decimalParts(value)
	return accumulator{coefficient: coefficient, scale: scale}
}

func negateAccumulator(value accumulator) accumulator {
	if value.coefficient != nil {
		value.coefficient.Neg(value.coefficient)
	}
	return value
}

func warningsForRow(commodity, account string, category Category, sums rowSums, directlyPosted bool) []Warning {
	if category == CategoryUnknown && directlyPosted {
		return []Warning{{Code: "unclassified_account", Account: account, Commodity: commodity}}
	}
	if !directlyPosted {
		return []Warning{}
	}
	warnings := []Warning{}
	for _, item := range []struct {
		name  string
		value accumulator
	}{{"opening", sums.opening}, {"closing", sums.closing}} {
		if item.value.sign() == 0 || normalSide(category) == side(item.value) {
			continue
		}
		warnings = append(warnings, Warning{
			Code: "opposite_normal_balance", Account: account, Commodity: commodity,
			Balance: item.name, Side: side(item.value),
		})
	}
	return warnings
}

func normalSide(category Category) string {
	if category == CategoryAsset || category == CategoryExpense {
		return "debit"
	}
	return "credit"
}

func side(value accumulator) string {
	if value.sign() < 0 {
		return "credit"
	}
	return "debit"
}

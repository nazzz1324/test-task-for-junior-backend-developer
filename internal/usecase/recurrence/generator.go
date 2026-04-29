package recurrence

import (
	"sort"
	"time"

	domain "example.com/taskservice/internal/domain/recurrence"
)

// NextDates returns up to maxCount occurrence dates for rule, starting from (and including) from.
// Dates are always truncated to UTC midnight. Results respect rule.EndsAt when set.
func NextDates(rule domain.Rule, from time.Time, maxCount int) []time.Time {
	from = dayUTC(from)
	startsAt := dayUTC(rule.StartsAt)
	if from.Before(startsAt) {
		from = startsAt
	}

	switch rule.Type {
	case domain.TypeEveryNDays:
		return everyNDays(rule, from, maxCount)
	case domain.TypeMonthlyOnDay:
		return monthlyOnDay(rule, from, maxCount)
	case domain.TypeSpecificDates:
		return specificDates(rule, from, maxCount)
	case domain.TypeEvenDays:
		return byParity(rule, from, maxCount, 0)
	case domain.TypeOddDays:
		return byParity(rule, from, maxCount, 1)
	default:
		return nil
	}
}

// dayUTC truncates t to the start of its calendar day in UTC.
func dayUTC(t time.Time) time.Time {
	t = t.UTC()
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
}

func withinEndsAt(rule domain.Rule, d time.Time) bool {
	if rule.EndsAt == nil {
		return true
	}
	return !d.After(dayUTC(*rule.EndsAt))
}

func everyNDays(rule domain.Rule, from time.Time, maxCount int) []time.Time {
	n := *rule.IntervalDays
	startsAt := dayUTC(rule.StartsAt)

	// Find the smallest k such that startsAt + k*n >= from.
	diffDays := int(from.Sub(startsAt).Hours() / 24)
	k := 0
	if diffDays > 0 {
		k = (diffDays + n - 1) / n
	}

	dates := make([]time.Time, 0, maxCount)
	for len(dates) < maxCount {
		d := startsAt.AddDate(0, 0, k*n)
		if !withinEndsAt(rule, d) {
			break
		}
		dates = append(dates, d)
		k++
	}
	return dates
}

func monthlyOnDay(rule domain.Rule, from time.Time, maxCount int) []time.Time {
	day := *rule.DayOfMonth
	dates := make([]time.Time, 0, maxCount)

	year, month := from.Year(), from.Month()
	for len(dates) < maxCount {
		lastDay := daysInMonth(year, month)
		if day <= lastDay {
			d := time.Date(year, month, day, 0, 0, 0, 0, time.UTC)
			if !d.Before(from) && withinEndsAt(rule, d) {
				dates = append(dates, d)
			}
		}
		month++
		if month > 12 {
			month = 1
			year++
		}
	}
	return dates
}

func daysInMonth(year int, month time.Month) int {
	return time.Date(year, month+1, 0, 0, 0, 0, 0, time.UTC).Day()
}

func specificDates(rule domain.Rule, from time.Time, maxCount int) []time.Time {
	sorted := make([]time.Time, len(rule.SpecificDates))
	for i, d := range rule.SpecificDates {
		sorted[i] = dayUTC(d)
	}
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Before(sorted[j]) })

	dates := make([]time.Time, 0, maxCount)
	for _, d := range sorted {
		if !d.Before(from) && withinEndsAt(rule, d) {
			dates = append(dates, d)
			if len(dates) >= maxCount {
				break
			}
		}
	}
	return dates
}

func byParity(rule domain.Rule, from time.Time, maxCount int, remainder int) []time.Time {
	dates := make([]time.Time, 0, maxCount)
	cur := from
	for len(dates) < maxCount {
		if !withinEndsAt(rule, cur) {
			break
		}
		if cur.Day()%2 == remainder {
			dates = append(dates, cur)
		}
		cur = cur.AddDate(0, 0, 1)
	}
	return dates
}

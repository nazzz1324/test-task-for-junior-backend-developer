package recurrence_test

import (
	"testing"
	"time"

	domain "example.com/taskservice/internal/domain/recurrence"
	"example.com/taskservice/internal/usecase/recurrence"
)

// ── helpers ───────────────────────────────────────────────────────────────────

func day(year int, month time.Month, d int) time.Time {
	return time.Date(year, month, d, 0, 0, 0, 0, time.UTC)
}

func intPtr(n int) *int   { return &n }
func timePtr(t time.Time) *time.Time { return &t }

func assertDates(t *testing.T, want, got []time.Time) {
	t.Helper()
	if len(want) != len(got) {
		t.Fatalf("len mismatch: want %d dates, got %d\nwant: %v\ngot:  %v", len(want), len(got), want, got)
	}
	for i := range want {
		if !want[i].Equal(got[i]) {
			t.Errorf("date[%d]: want %s, got %s", i, want[i].Format("2006-01-02"), got[i].Format("2006-01-02"))
		}
	}
}

// ── every_n_days ──────────────────────────────────────────────────────────────

func TestNextDates_EveryNDays(t *testing.T) {
	tests := []struct {
		name     string
		rule     domain.Rule
		from     time.Time
		maxCount int
		want     []time.Time
	}{
		{
			name: "daily from start",
			rule: domain.Rule{
				Type: domain.TypeEveryNDays, IntervalDays: intPtr(1),
				StartsAt: day(2024, 1, 1),
			},
			from: day(2024, 1, 1), maxCount: 3,
			want: []time.Time{day(2024, 1, 1), day(2024, 1, 2), day(2024, 1, 3)},
		},
		{
			name: "weekly, from between two occurrences advances to next",
			rule: domain.Rule{
				Type: domain.TypeEveryNDays, IntervalDays: intPtr(7),
				StartsAt: day(2024, 1, 1),
			},
			from: day(2024, 1, 9), maxCount: 2, // seq: Jan1, Jan8, Jan15…
			want: []time.Time{day(2024, 1, 15), day(2024, 1, 22)},
		},
		{
			name: "from exactly on a sequence date includes it",
			rule: domain.Rule{
				Type: domain.TypeEveryNDays, IntervalDays: intPtr(7),
				StartsAt: day(2024, 1, 1),
			},
			from: day(2024, 1, 15), maxCount: 2,
			want: []time.Time{day(2024, 1, 15), day(2024, 1, 22)},
		},
		{
			name: "from before startsAt uses startsAt",
			rule: domain.Rule{
				Type: domain.TypeEveryNDays, IntervalDays: intPtr(3),
				StartsAt: day(2024, 2, 1),
			},
			from: day(2024, 1, 1), maxCount: 3,
			want: []time.Time{day(2024, 2, 1), day(2024, 2, 4), day(2024, 2, 7)},
		},
		{
			name: "ends_at truncates results",
			rule: domain.Rule{
				Type: domain.TypeEveryNDays, IntervalDays: intPtr(7),
				StartsAt: day(2024, 1, 1), EndsAt: timePtr(day(2024, 1, 15)),
			},
			from: day(2024, 1, 1), maxCount: 10,
			want: []time.Time{day(2024, 1, 1), day(2024, 1, 8), day(2024, 1, 15)},
		},
		{
			name: "ends_at equals from returns single date",
			rule: domain.Rule{
				Type: domain.TypeEveryNDays, IntervalDays: intPtr(1),
				StartsAt: day(2024, 1, 1), EndsAt: timePtr(day(2024, 1, 1)),
			},
			from: day(2024, 1, 1), maxCount: 5,
			want: []time.Time{day(2024, 1, 1)},
		},
		{
			name: "maxCount=0 returns empty",
			rule: domain.Rule{
				Type: domain.TypeEveryNDays, IntervalDays: intPtr(1),
				StartsAt: day(2024, 1, 1),
			},
			from: day(2024, 1, 1), maxCount: 0,
			want: nil,
		},
		{
			name: "N=30 crosses month boundary correctly",
			rule: domain.Rule{
				Type: domain.TypeEveryNDays, IntervalDays: intPtr(30),
				StartsAt: day(2024, 1, 1),
			},
			from: day(2024, 1, 1), maxCount: 3,
			want: []time.Time{day(2024, 1, 1), day(2024, 1, 31), day(2024, 3, 1)},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := recurrence.NextDates(tt.rule, tt.from, tt.maxCount)
			if len(tt.want) == 0 {
				if len(got) != 0 {
					t.Errorf("want empty, got %v", got)
				}
				return
			}
			assertDates(t, tt.want, got)
		})
	}
}

// ── monthly_on_day ────────────────────────────────────────────────────────────

func TestNextDates_MonthlyOnDay(t *testing.T) {
	tests := []struct {
		name     string
		rule     domain.Rule
		from     time.Time
		maxCount int
		want     []time.Time
	}{
		{
			name: "day=15 normal months",
			rule: domain.Rule{
				Type: domain.TypeMonthlyOnDay, DayOfMonth: intPtr(15),
				StartsAt: day(2024, 1, 1),
			},
			from: day(2024, 1, 1), maxCount: 3,
			want: []time.Time{day(2024, 1, 15), day(2024, 2, 15), day(2024, 3, 15)},
		},
		{
			name: "day=31 skips Feb Apr Jun Sep Nov (5 hits across 8 months)",
			rule: domain.Rule{
				Type: domain.TypeMonthlyOnDay, DayOfMonth: intPtr(31),
				StartsAt: day(2024, 1, 1),
			},
			from: day(2024, 1, 1), maxCount: 5,
			// Jan31, [Feb skip], Mar31, [Apr skip], May31, [Jun skip], Jul31, Aug31
			want: []time.Time{
				day(2024, 1, 31), day(2024, 3, 31), day(2024, 5, 31),
				day(2024, 7, 31), day(2024, 8, 31),
			},
		},
		{
			name: "day=30 skips February (2024 is leap — Feb has 29 days)",
			rule: domain.Rule{
				Type: domain.TypeMonthlyOnDay, DayOfMonth: intPtr(30),
				StartsAt: day(2024, 1, 1),
			},
			from: day(2024, 1, 1), maxCount: 4,
			// Jan30, [Feb skip], Mar30, Apr30
			want: []time.Time{day(2024, 1, 30), day(2024, 3, 30), day(2024, 4, 30), day(2024, 5, 30)},
		},
		{
			name: "day=29 in non-leap year 2023 skips February",
			rule: domain.Rule{
				Type: domain.TypeMonthlyOnDay, DayOfMonth: intPtr(29),
				StartsAt: day(2023, 1, 1),
			},
			from: day(2023, 1, 1), maxCount: 3,
			// Jan29, [Feb skip — 2023 not leap], Mar29
			want: []time.Time{day(2023, 1, 29), day(2023, 3, 29), day(2023, 4, 29)},
		},
		{
			name: "day=29 in leap year 2024 includes February",
			rule: domain.Rule{
				Type: domain.TypeMonthlyOnDay, DayOfMonth: intPtr(29),
				StartsAt: day(2024, 2, 1),
			},
			from: day(2024, 2, 1), maxCount: 3,
			want: []time.Time{day(2024, 2, 29), day(2024, 3, 29), day(2024, 4, 29)},
		},
		{
			name: "from after day-of-month in current month skips to next month",
			rule: domain.Rule{
				Type: domain.TypeMonthlyOnDay, DayOfMonth: intPtr(10),
				StartsAt: day(2024, 1, 1),
			},
			from: day(2024, 1, 15), maxCount: 2,
			want: []time.Time{day(2024, 2, 10), day(2024, 3, 10)},
		},
		{
			name: "ends_at cuts off at month boundary",
			rule: domain.Rule{
				Type: domain.TypeMonthlyOnDay, DayOfMonth: intPtr(1),
				StartsAt: day(2024, 1, 1), EndsAt: timePtr(day(2024, 3, 1)),
			},
			from: day(2024, 1, 1), maxCount: 10,
			want: []time.Time{day(2024, 1, 1), day(2024, 2, 1), day(2024, 3, 1)},
		},
		{
			name: "day=31 in month with 30 days is skipped (April)",
			rule: domain.Rule{
				Type: domain.TypeMonthlyOnDay, DayOfMonth: intPtr(31),
				StartsAt: day(2024, 4, 1),
			},
			from: day(2024, 4, 1), maxCount: 2,
			// Apr has 30 days → skip; May31, Jun has 30 → skip; Jul31
			want: []time.Time{day(2024, 5, 31), day(2024, 7, 31)},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := recurrence.NextDates(tt.rule, tt.from, tt.maxCount)
			assertDates(t, tt.want, got)
		})
	}
}

// ── specific_dates ────────────────────────────────────────────────────────────

func TestNextDates_SpecificDates(t *testing.T) {
	tests := []struct {
		name     string
		rule     domain.Rule
		from     time.Time
		maxCount int
		want     []time.Time
	}{
		{
			name: "returns dates >= from in sorted order",
			rule: domain.Rule{
				Type: domain.TypeSpecificDates,
				SpecificDates: []time.Time{day(2024, 3, 1), day(2024, 1, 15), day(2024, 6, 10)},
				StartsAt: day(2024, 1, 1),
			},
			from: day(2024, 1, 15), maxCount: 5,
			want: []time.Time{day(2024, 1, 15), day(2024, 3, 1), day(2024, 6, 10)},
		},
		{
			name: "dates before from are skipped",
			rule: domain.Rule{
				Type: domain.TypeSpecificDates,
				SpecificDates: []time.Time{day(2024, 1, 1), day(2024, 1, 5), day(2024, 1, 10)},
				StartsAt: day(2024, 1, 1),
			},
			from: day(2024, 1, 6), maxCount: 5,
			want: []time.Time{day(2024, 1, 10)},
		},
		{
			name: "maxCount limits output",
			rule: domain.Rule{
				Type: domain.TypeSpecificDates,
				SpecificDates: []time.Time{day(2024, 1, 1), day(2024, 2, 1), day(2024, 3, 1), day(2024, 4, 1)},
				StartsAt: day(2024, 1, 1),
			},
			from: day(2024, 1, 1), maxCount: 2,
			want: []time.Time{day(2024, 1, 1), day(2024, 2, 1)},
		},
		{
			name: "ends_at filters out later dates",
			rule: domain.Rule{
				Type: domain.TypeSpecificDates,
				SpecificDates: []time.Time{day(2024, 1, 1), day(2024, 6, 1), day(2024, 12, 31)},
				StartsAt: day(2024, 1, 1), EndsAt: timePtr(day(2024, 6, 1)),
			},
			from: day(2024, 1, 1), maxCount: 10,
			want: []time.Time{day(2024, 1, 1), day(2024, 6, 1)},
		},
		{
			name: "all dates before from returns empty",
			rule: domain.Rule{
				Type: domain.TypeSpecificDates,
				SpecificDates: []time.Time{day(2024, 1, 1), day(2024, 2, 1)},
				StartsAt: day(2024, 1, 1),
			},
			from: day(2024, 3, 1), maxCount: 5,
			want: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := recurrence.NextDates(tt.rule, tt.from, tt.maxCount)
			if len(tt.want) == 0 {
				if len(got) != 0 {
					t.Errorf("want empty, got %v", got)
				}
				return
			}
			assertDates(t, tt.want, got)
		})
	}
}

// ── even_days / odd_days ──────────────────────────────────────────────────────

func TestNextDates_EvenOddDays(t *testing.T) {
	tests := []struct {
		name     string
		rule     domain.Rule
		from     time.Time
		maxCount int
		want     []time.Time
	}{
		{
			name: "even_days basic: starts on odd day, skips it",
			rule: domain.Rule{
				Type: domain.TypeEvenDays, StartsAt: day(2024, 1, 1),
			},
			from: day(2024, 1, 1), maxCount: 4, // Jan1(odd), Jan2(even), Jan3(odd), Jan4(even)...
			want: []time.Time{day(2024, 1, 2), day(2024, 1, 4), day(2024, 1, 6), day(2024, 1, 8)},
		},
		{
			name: "even_days: from on even day includes it",
			rule: domain.Rule{
				Type: domain.TypeEvenDays, StartsAt: day(2024, 1, 1),
			},
			from: day(2024, 1, 4), maxCount: 3,
			want: []time.Time{day(2024, 1, 4), day(2024, 1, 6), day(2024, 1, 8)},
		},
		{
			name: "even_days: day 30 is even, day 31 is odd — month boundary",
			rule: domain.Rule{
				Type: domain.TypeEvenDays, StartsAt: day(2024, 1, 29),
			},
			from: day(2024, 1, 29), maxCount: 3,
			// Jan29(odd), Jan30(even), Jan31(odd), Feb1(odd), Feb2(even)
			want: []time.Time{day(2024, 1, 30), day(2024, 2, 2), day(2024, 2, 4)},
		},
		{
			name: "odd_days basic: starts on even day, skips it",
			rule: domain.Rule{
				Type: domain.TypeOddDays, StartsAt: day(2024, 1, 1),
			},
			from: day(2024, 1, 2), maxCount: 4, // Jan2(even), Jan3(odd)...
			want: []time.Time{day(2024, 1, 3), day(2024, 1, 5), day(2024, 1, 7), day(2024, 1, 9)},
		},
		{
			name: "odd_days: day 31 is odd, day 1 of next month is also odd",
			rule: domain.Rule{
				Type: domain.TypeOddDays, StartsAt: day(2024, 1, 30),
			},
			from: day(2024, 1, 30), maxCount: 3,
			// Jan30(even), Jan31(odd), Feb1(odd), Feb2(even), Feb3(odd)
			want: []time.Time{day(2024, 1, 31), day(2024, 2, 1), day(2024, 2, 3)},
		},
		{
			name: "even_days truncated by ends_at",
			rule: domain.Rule{
				Type: domain.TypeEvenDays, StartsAt: day(2024, 1, 1),
				EndsAt: timePtr(day(2024, 1, 6)),
			},
			from: day(2024, 1, 1), maxCount: 100,
			want: []time.Time{day(2024, 1, 2), day(2024, 1, 4), day(2024, 1, 6)},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := recurrence.NextDates(tt.rule, tt.from, tt.maxCount)
			assertDates(t, tt.want, got)
		})
	}
}

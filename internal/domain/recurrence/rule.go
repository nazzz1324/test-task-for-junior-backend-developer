package recurrence

import "time"

type Type string

const (
	TypeEveryNDays    Type = "every_n_days"
	TypeMonthlyOnDay  Type = "monthly_on_day"
	TypeSpecificDates Type = "specific_dates"
	TypeEvenDays      Type = "even_days"
	TypeOddDays       Type = "odd_days"
)

func (t Type) Valid() bool {
	switch t {
	case TypeEveryNDays, TypeMonthlyOnDay, TypeSpecificDates, TypeEvenDays, TypeOddDays:
		return true
	default:
		return false
	}
}

type Rule struct {
	ID              int64
	TaskTitle       string
	TaskDescription string
	Type            Type
	IntervalDays    *int
	DayOfMonth      *int
	SpecificDates   []time.Time
	StartsAt        time.Time
	EndsAt          *time.Time
	CreatedAt       time.Time
}

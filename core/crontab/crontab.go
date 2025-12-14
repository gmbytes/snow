package crontab

import (
	"math"
	"time"
)

type timeRange struct {
	begin int
	end   int
	step  int // 0: only begin value
}

type CronExpression struct {
	year   []*timeRange
	month  []*timeRange
	day    []*timeRange
	week   []*timeRange
	hour   []*timeRange
	minute []*timeRange
	second []*timeRange
}

func (c *CronExpression) Init() {
	if c.year == nil {
		c.year = []*timeRange{{begin: 0, end: math.MaxInt32, step: 1}}
	}
	if c.month == nil {
		c.month = []*timeRange{{begin: 1, end: 12, step: 1}}
	}
	if c.day == nil {
		c.day = []*timeRange{{begin: 1, end: 31, step: 1}}
	}
	if c.week == nil {
		c.week = []*timeRange{{begin: 0, end: 6, step: 1}}
	}
	if c.hour == nil {
		c.hour = []*timeRange{{begin: 0, end: 23, step: 1}}
	}
	if c.minute == nil {
		c.minute = []*timeRange{{begin: 0, end: 59, step: 1}}
	}
	if c.second == nil {
		c.second = []*timeRange{{begin: 0, end: 59, step: 1}}
	}
}

func (c *CronExpression) normalizeUnit(ranges []*timeRange, val int) (carry bool, newVal int) {
	for _, r := range ranges {
		b, e, s := r.begin, r.end, r.step
		if r.step == 0 {
			if val <= b {
				return false, b
			}
			continue
		}

		if val > e {
			continue
		}

		if val <= b {
			return false, b
		}

		nv := val
		if (val-b)%s > 0 {
			nv = ((val-b)/s+1)*s + b
		}
		if nv > e {
			continue
		}

		return false, nv
	}
	return true, ranges[0].begin
}

func (c *CronExpression) Normalize(t time.Time) time.Time {
	year, _, day := t.Date()
	month := int(t.Month())
	hour, minute, second := t.Clock()

	var carry bool
	var oldVal int

	oldVal = year
YEAR:
	carry, year = c.normalizeUnit(c.year, year)
	if carry { // year cannot have carry
		return time.Time{}
	}
	if oldVal != year {
		month, day, hour, minute, second = 1, 1, 0, 0, 0
	}

	oldVal = month
MONTH:
	carry, month = c.normalizeUnit(c.month, month)
	if carry {
		oldVal = year
		year++
		goto YEAR
	} else if oldVal != month {
		day, hour, minute, second = 1, 0, 0, 0
	}

	oldVal = day
DAY:
	carry, day = c.normalizeUnit(c.day, day)
	if carry || time.Date(year, time.Month(month), day, 0, 0, 0, 0, t.Location()).Day() != day {
		oldVal = month
		month++
		goto MONTH
	} else if oldVal != day {
		hour, minute, second = 0, 0, 0
	}

	week := int(time.Date(year, time.Month(month), day, 0, 0, 0, 0, t.Location()).Weekday())
	oldVal = week
	carry, week = c.normalizeUnit(c.week, week)
	if carry {
		nt := time.Date(year, time.Month(month), day+week-oldVal+7, 0, 0, 0, 0, t.Location())
		day = nt.Day()
		if int(nt.Month()) != month {
			oldVal = month
			month++
			goto MONTH
		}
		hour, minute, second = 0, 0, 0
	} else if oldVal != week {
		nt := time.Date(year, time.Month(month), day+week-oldVal, 0, 0, 0, 0, t.Location())
		day = nt.Day()
		if int(nt.Month()) != month {
			oldVal = month
			month++
			goto MONTH
		}
		hour, minute, second = 0, 0, 0
	}

	oldVal = hour
HOUR:
	carry, hour = c.normalizeUnit(c.hour, hour)
	if carry {
		oldVal = day
		day++
		goto DAY
	} else if oldVal != hour {
		minute, second = 0, 0
	}

	oldVal = minute
MINUTE:
	carry, minute = c.normalizeUnit(c.minute, minute)
	if carry {
		oldVal = hour
		hour++
		goto HOUR
	} else if oldVal != minute {
		second = 0
	}

	oldVal = second

	carry, second = c.normalizeUnit(c.second, second)
	if carry {
		oldVal = minute
		minute++
		goto MINUTE
	}

	return time.Date(year, time.Month(month), day, hour, minute, second, 0, t.Location())
}

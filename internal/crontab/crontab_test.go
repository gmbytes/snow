package crontab

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestCronExpression_Normalize_WithSecondConfigured(t *testing.T) {
	expr := &CronExpression{second: []*timeRange{{begin: 0, end: 59, step: 5}}}
	expr.Init()

	assert.Equal(t, time.Date(2020, time.January, 5, 6, 7, 10, 0, time.Local).Unix(),
		expr.Normalize(time.Date(2020, time.January, 5, 6, 7, 9, 0, time.Local)).Unix())

	assert.Equal(t, time.Date(2020, time.January, 5, 6, 7, 10, 0, time.Local).Unix(),
		expr.Normalize(time.Date(2020, time.January, 5, 6, 7, 8, 0, time.Local)).Unix())

	assert.Equal(t, time.Date(2020, time.January, 5, 6, 7, 10, 0, time.Local).Unix(),
		expr.Normalize(time.Date(2020, time.January, 5, 6, 7, 7, 0, time.Local)).Unix())

	assert.Equal(t, time.Date(2020, time.January, 5, 6, 7, 10, 0, time.Local).Unix(),
		expr.Normalize(time.Date(2020, time.January, 5, 6, 7, 6, 0, time.Local)).Unix())

	assert.Equal(t, time.Date(2020, time.January, 5, 6, 7, 5, 0, time.Local).Unix(),
		expr.Normalize(time.Date(2020, time.January, 5, 6, 7, 5, 0, time.Local)).Unix())

	assert.Equal(t, time.Date(2020, time.January, 5, 6, 8, 0, 0, time.Local).Unix(),
		expr.Normalize(time.Date(2020, time.January, 5, 6, 7, 56, 0, time.Local)).Unix())

	assert.Equal(t, time.Date(2001, time.January, 1, 0, 0, 0, 0, time.Local).Unix(),
		expr.Normalize(time.Date(2000, time.December, 31, 23, 59, 56, 0, time.Local)).Unix())

	assert.Equal(t, time.Date(2001, time.January, 1, 0, 0, 0, 0, time.Local).Unix(),
		expr.Normalize(time.Date(2000, time.December, 31, 23, 59, 59, 0, time.Local)).Unix())
}

func TestCronExpression_Normalize_WithMinuteConfigured(t *testing.T) {
	expr := &CronExpression{minute: []*timeRange{{begin: 3, end: 59, step: 7}}}
	expr.Init()

	assert.Equal(t, time.Date(2020, time.January, 5, 6, 3, 0, 0, time.Local).Unix(),
		expr.Normalize(time.Date(2020, time.January, 5, 6, 1, 0, 0, time.Local)).Unix())

	assert.Equal(t, time.Date(2020, time.January, 5, 6, 3, 0, 1, time.Local).Unix(),
		expr.Normalize(time.Date(2020, time.January, 5, 6, 3, 0, 1, time.Local)).Unix())

	assert.Equal(t, time.Date(2020, time.January, 5, 6, 3, 0, 0, time.Local).Unix(),
		expr.Normalize(time.Date(2020, time.January, 5, 6, 2, 0, 0, time.Local)).Unix())

	assert.Equal(t, time.Date(2020, time.January, 5, 6, 3, 0, 0, time.Local).Unix(),
		expr.Normalize(time.Date(2020, time.January, 5, 6, 3, 0, 0, time.Local)).Unix())

	assert.Equal(t, time.Date(2020, time.January, 5, 6, 10, 0, 0, time.Local).Unix(),
		expr.Normalize(time.Date(2020, time.January, 5, 6, 4, 0, 0, time.Local)).Unix())

	assert.Equal(t, time.Date(2020, time.January, 5, 6, 10, 0, 0, time.Local).Unix(),
		expr.Normalize(time.Date(2020, time.January, 5, 6, 10, 0, 0, time.Local)).Unix())

	assert.Equal(t, time.Date(2020, time.January, 5, 6, 17, 0, 0, time.Local).Unix(),
		expr.Normalize(time.Date(2020, time.January, 5, 6, 11, 0, 0, time.Local)).Unix())
}

func TestCronExpression_Normalize_WithHourConfigured(t *testing.T) {
	expr := &CronExpression{hour: []*timeRange{{begin: 3}}}
	expr.Init()

	assert.Equal(t, time.Date(2020, time.January, 5, 3, 0, 0, 0, time.Local).Unix(),
		expr.Normalize(time.Date(2020, time.January, 5, 0, 0, 0, 0, time.Local)).Unix())

	assert.Equal(t, time.Date(2020, time.January, 6, 3, 0, 0, 0, time.Local).Unix(),
		expr.Normalize(time.Date(2020, time.January, 5, 4, 0, 0, 0, time.Local)).Unix())
}

func TestCronExpression_Normalize_WithWeekConfigured(t *testing.T) {
	expr := &CronExpression{week: []*timeRange{{begin: 1, end: 5, step: 2}}}
	expr.Init()

	assert.Equal(t, time.Date(2021, time.February, 10, 0, 0, 0, 0, time.Local).Unix(),
		expr.Normalize(time.Date(2021, time.February, 9, 0, 0, 0, 0, time.Local)).Unix())

	assert.Equal(t, time.Date(2021, time.February, 12, 0, 0, 0, 0, time.Local).Unix(),
		expr.Normalize(time.Date(2021, time.February, 11, 0, 0, 0, 0, time.Local)).Unix())

	assert.Equal(t, time.Date(2021, time.February, 15, 0, 0, 0, 0, time.Local).Unix(),
		expr.Normalize(time.Date(2021, time.February, 13, 0, 0, 0, 0, time.Local)).Unix())

	assert.Equal(t, time.Date(2021, time.March, 1, 0, 0, 0, 0, time.Local).Unix(),
		expr.Normalize(time.Date(2021, time.February, 27, 0, 0, 0, 0, time.Local)).Unix())
}

func TestCronExpression_Normalize_WithDefaultConfiguration(t *testing.T) {
	expr := &CronExpression{}
	expr.Init()

	assert.Equal(t, time.Date(2000, time.December, 31, 23, 59, 59, 0, time.Local).Unix(),
		expr.Normalize(time.Date(2000, time.December, 31, 23, 59, 59, 0, time.Local)).Unix())

	assert.Equal(t, time.Date(2001, time.January, 1, 0, 0, 0, 0, time.Local).Unix(),
		expr.Normalize(time.Date(2000, time.December, 31, 23, 59, 60, 0, time.Local)).Unix())
}

func TestCronExpression_Normalize_WithComplexSituation(t *testing.T) {
	expr := &CronExpression{
		month:  []*timeRange{{begin: 2}},
		day:    []*timeRange{{begin: 29}},
		hour:   []*timeRange{{begin: 1}},
		minute: []*timeRange{{begin: 2}},
		second: []*timeRange{{begin: 3}},
	}
	expr.Init()

	assert.Equal(t, time.Date(2000, time.February, 29, 1, 2, 3, 0, time.Local).Unix(),
		expr.Normalize(time.Date(2000, time.January, 1, 0, 0, 0, 0, time.Local)).Unix())

	assert.Equal(t, time.Date(2000, time.February, 29, 1, 2, 3, 0, time.Local).Unix(),
		expr.Normalize(time.Date(2000, time.February, 29, 0, 0, 0, 0, time.Local)).Unix())

	assert.Equal(t, time.Date(2000, time.February, 29, 1, 2, 3, 0, time.Local).Unix(),
		expr.Normalize(time.Date(2000, time.February, 29, 1, 2, 3, 0, time.Local)).Unix())

	assert.Equal(t, time.Date(2004, time.February, 29, 1, 2, 3, 0, time.Local).Unix(),
		expr.Normalize(time.Date(2000, time.February, 29, 1, 2, 4, 0, time.Local)).Unix())

	assert.Equal(t, time.Date(2004, time.February, 29, 1, 2, 3, 0, time.Local).Unix(),
		expr.Normalize(time.Date(2000, time.February, 30, 0, 0, 0, 0, time.Local)).Unix())
}

func TestParse(t *testing.T) {
	ct, err := Parse("0 2 * * * * *")
	assert.NoError(t, err)
	assert.NotNil(t, ct)

	ct, err = Parse("@daily")
	assert.NoError(t, err)
	assert.NotNil(t, ct)

	ct, err = Parse("@hourly")
	assert.NoError(t, err)
	assert.NotNil(t, ct)

	_, err = Parse("invalid")
	assert.Error(t, err)
}

func TestMustParse(t *testing.T) {
	ct := MustParse("0 2 * * * * *")
	assert.NotNil(t, ct)

	assert.Panics(t, func() {
		MustParse("invalid")
	})
}

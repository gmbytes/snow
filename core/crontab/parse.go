package crontab

import (
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
)

var (
	layoutWildcard            = `^\*$|^\?$`
	layoutValue               = `^(%value%)$`
	layoutRange               = `^(%value%)-(%value%)$`
	layoutWildcardAndInterval = `^\*/(\d+)$`
	layoutValueAndInterval    = `^(%value%)/(\d+)$`
	layoutRangeAndInterval    = `^(%value%)-(%value%)/(\d+)$`
	fieldFinder               = regexp.MustCompile(`\S+`)
	entryFinder               = regexp.MustCompile(`[^,]+`)
)

type descriptorName string

const (
	_descriptorSecond descriptorName = "second"
	_minuteDescriptor descriptorName = "minute"
	_hourDescriptor   descriptorName = "hour"
	_domDescriptor    descriptorName = "day-of-month"
	_monthDescriptor  descriptorName = "month"
	_dowDescriptor    descriptorName = "day-of-week"
	_yearDescriptor   descriptorName = "year"
)

type fieldDescriptor struct {
	name         descriptorName
	min          int
	max          int
	valuePattern string
}

var (
	secondDescriptor = fieldDescriptor{
		name:         _descriptorSecond,
		min:          0,
		max:          59,
		valuePattern: `0?[0-9]|[1-5][0-9]`,
	}
	minuteDescriptor = fieldDescriptor{
		name:         _minuteDescriptor,
		min:          0,
		max:          59,
		valuePattern: `0?[0-9]|[1-5][0-9]`,
	}
	hourDescriptor = fieldDescriptor{
		name:         _hourDescriptor,
		min:          0,
		max:          23,
		valuePattern: `0?[0-9]|1[0-9]|2[0-3]`,
	}
	domDescriptor = fieldDescriptor{
		name:         _domDescriptor,
		min:          1,
		max:          31,
		valuePattern: `0?[1-9]|[12][0-9]|3[01]`,
	}
	monthDescriptor = fieldDescriptor{
		name:         _monthDescriptor,
		min:          1,
		max:          12,
		valuePattern: `0?[1-9]|1[012]|jan|feb|mar|apr|may|jun|jul|aug|sep|oct|nov|dec|january|february|march|april|june|july|august|september|october|november|december`,
	}
	dowDescriptor = fieldDescriptor{
		name:         _dowDescriptor,
		min:          0,
		max:          6,
		valuePattern: `0?[0-7]|sun|mon|tue|wed|thu|fri|sat|sunday|monday|tuesday|wednesday|thursday|friday|saturday`,
	}
	yearDescriptor = fieldDescriptor{
		name:         _yearDescriptor,
		min:          1970,
		max:          math.MaxInt32,
		valuePattern: `[0-9]{4}`,
	}

	cronNormalizer = strings.NewReplacer(
		"@yearly", "0 0 0 1 1 * *",
		"@annually", "0 0 0 1 1 * *",
		"@monthly", "0 0 0 1 * * *",
		"@weekly", "0 0 0 * * 0 *",
		"@daily", "0 0 0 * * * *",
		"@hourly", "0 0 * * * * *")
)

func MustParse(cronLine string) *CronExpression {
	expr, err := Parse(cronLine)
	if err != nil {
		panic(err)
	}
	return expr
}

func Parse(cronLine string) (*CronExpression, error) {
	expr := &CronExpression{}
	expr.Init()

	cronLine = cronNormalizer.Replace(cronLine)
	indices := fieldFinder.FindAllStringIndex(cronLine, -1)

	fieldCount := len(indices)
	if fieldCount < 5 {
		return nil, fmt.Errorf("missing field(s)")
	}

	if fieldCount > 7 {
		fieldCount = 7
	}

	var field = 0
	var err error

	str := ""
	// second field (optional)
	if fieldCount == 7 {
		str = cronLine[indices[field][0]:indices[field][1]]
		err = expr.parseField(str, secondDescriptor)
		if err != nil {
			return nil, err
		}
		field += 1
	}

	// minute field
	str = cronLine[indices[field][0]:indices[field][1]]
	err = expr.parseField(str, minuteDescriptor)
	if err != nil {
		return nil, err
	}
	field += 1

	// hour field
	str = cronLine[indices[field][0]:indices[field][1]]
	err = expr.parseField(str, hourDescriptor)
	if err != nil {
		return nil, err
	}
	field += 1

	// day of month field
	str = cronLine[indices[field][0]:indices[field][1]]
	err = expr.parseField(str, domDescriptor)
	if err != nil {
		return nil, err
	}
	field += 1

	// month field
	str = cronLine[indices[field][0]:indices[field][1]]
	err = expr.parseField(str, monthDescriptor)
	if err != nil {
		return nil, err
	}
	field += 1

	// day of week field
	str = cronLine[indices[field][0]:indices[field][1]]
	err = expr.parseField(str, dowDescriptor)
	if err != nil {
		return nil, err
	}
	field += 1

	// year field
	if field < fieldCount {
		str = cronLine[indices[field][0]:indices[field][1]]
		err = expr.parseField(str, yearDescriptor)
		if err != nil {
			return nil, err
		}
	}
	return expr, nil
}

func (c *CronExpression) parseField(s string, desc fieldDescriptor) error {
	// At least one entry must be present
	indices := entryFinder.FindAllStringIndex(s, -1)
	if len(indices) == 0 {
		return fmt.Errorf("%s field: missing directive", desc.name)
	}

	ranges := make([]*timeRange, 0, len(indices))
	for i := range indices {

		r := &timeRange{
			step: 1,
		}
		snormal := strings.ToLower(s[indices[i][0]:indices[i][1]])
		// `*`
		if c.getOrCompileRegexp(layoutWildcard, desc.valuePattern).MatchString(snormal) {
			r.begin = desc.min
			r.end = desc.max
			ranges = append(ranges, r)
			continue
		}
		// `5`
		if c.getOrCompileRegexp(layoutValue, desc.valuePattern).MatchString(snormal) {
			r.begin = c.parseInt(snormal)
			r.step = 0
			ranges = append(ranges, r)
			continue
		}
		// `5-20`
		pairs := c.getOrCompileRegexp(layoutRange, desc.valuePattern).FindStringSubmatchIndex(snormal)
		if len(pairs) > 0 {
			r.begin = c.parseInt(snormal[pairs[2]:pairs[3]])
			r.end = c.parseInt(snormal[pairs[4]:pairs[5]])
			ranges = append(ranges, r)
			continue
		}
		// `*/2`
		pairs = c.getOrCompileRegexp(layoutWildcardAndInterval, desc.valuePattern).FindStringSubmatchIndex(snormal)
		if len(pairs) > 0 {
			r.begin = desc.min
			r.end = desc.max
			r.step = c.parseInt(snormal[pairs[2]:pairs[3]])
			if r.step < 1 || r.step > desc.max {
				return fmt.Errorf("invalid interval %s", snormal)
			}
			ranges = append(ranges, r)
			continue
		}
		// `5/2`
		pairs = c.getOrCompileRegexp(layoutValueAndInterval, desc.valuePattern).FindStringSubmatchIndex(snormal)
		if len(pairs) > 0 {
			r.begin = c.parseInt(snormal[pairs[2]:pairs[3]])
			r.end = desc.max

			r.step = c.parseInt(snormal[pairs[4]:pairs[5]])
			if r.step < 1 || r.step > desc.max {
				return fmt.Errorf("invalid interval %s", snormal)
			}
			ranges = append(ranges, r)
			continue
		}
		// `5-20/2`
		pairs = c.getOrCompileRegexp(layoutRangeAndInterval, desc.valuePattern).FindStringSubmatchIndex(snormal)
		if len(pairs) > 0 {
			r.begin = c.parseInt(snormal[pairs[2]:pairs[3]])
			r.end = c.parseInt(snormal[pairs[4]:pairs[5]])
			r.step = c.parseInt(snormal[pairs[6]:pairs[7]])
			if r.step < 1 || r.step > desc.max {
				return fmt.Errorf("invalid interval %s", snormal)
			}
			ranges = append(ranges, r)
			continue
		}
	}

	switch desc.name {
	case _descriptorSecond:
		c.second = ranges
	case _minuteDescriptor:
		c.minute = ranges
	case _hourDescriptor:
		c.hour = ranges
	case _domDescriptor:
		c.day = ranges
	case _monthDescriptor:
		c.month = ranges
	case _dowDescriptor:
		c.week = ranges
	case _yearDescriptor:
		c.year = ranges
	}
	return nil
}

func (c *CronExpression) getOrCompileRegexp(layout, value string) *regexp.Regexp {
	c.layoutRegexpLock.Lock()
	defer c.layoutRegexpLock.Unlock()

	layout = strings.Replace(layout, `%value%`, value, -1)
	re := c.layoutRegexp[layout]
	if re == nil {
		re = regexp.MustCompile(layout)
		c.layoutRegexp[layout] = re
	}
	return re
}

func (c *CronExpression) parseInt(s string) int {
	i, err := strconv.Atoi(s)
	if err != nil {
		// 这里使用 panic 是因为在 parseField 中已经通过正则表达式验证了格式
		// 如果正则匹配成功但 Atoi 失败，说明是内部错误，应该 panic
		panic(fmt.Sprintf("internal error: failed to parse number '%s': %v", s, err))
	}
	return i
}

// printRaw is a debug method that prints the internal structure of CronExpression.
// It should only be used for debugging purposes.
// Deprecated: This method is for debugging only and may be removed in future versions.
func (c *CronExpression) printRaw() {
	// second field (optional)
	fmt.Print("second field:")
	for _, r := range c.second {
		fmt.Print(fmt.Sprintf(" begin:%d end:%d step:%d  ", r.begin, r.end, r.step))
	}
	fmt.Print("\t")
	// minute field

	fmt.Print("minute field:")
	for _, r := range c.minute {
		fmt.Print(fmt.Sprintf(" begin:%d end:%d step:%d  ", r.begin, r.end, r.step))
	}
	fmt.Print("\t")

	// hour field
	fmt.Print("hour field:")
	for _, r := range c.hour {
		fmt.Print(fmt.Sprintf(" begin:%d end:%d step:%d  ", r.begin, r.end, r.step))
	}
	fmt.Print("\t")

	// day of month field
	fmt.Print("day month field:")
	for _, r := range c.day {
		fmt.Print(fmt.Sprintf(" begin:%d end:%d step:%d  ", r.begin, r.end, r.step))
	}
	fmt.Print("\t")

	// month field

	fmt.Print("month field:")
	for _, r := range c.month {
		fmt.Print(fmt.Sprintf(" begin:%d end:%d step:%d  ", r.begin, r.end, r.step))
	}
	fmt.Print("\t")

	fmt.Print("day week field:")
	for _, r := range c.week {
		fmt.Print(fmt.Sprintf(" begin:%d end:%d step:%d  ", r.begin, r.end, r.step))
	}
	fmt.Print("\t")

	fmt.Print("year field:")
	for _, r := range c.year {
		fmt.Print(fmt.Sprintf(" begin:%d end:%d step:%d  ", r.begin, r.end, r.step))
	}
	fmt.Println()
}

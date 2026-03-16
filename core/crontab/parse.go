package crontab

import (
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
)

var (
	fieldFinder = regexp.MustCompile(`\S+`)
	entryFinder = regexp.MustCompile(`[^,]+`)
)

// 月份名称映射
var monthNames = map[string]int{
	"jan": 1, "january": 1,
	"feb": 2, "february": 2,
	"mar": 3, "march": 3,
	"apr": 4, "april": 4,
	"may": 5,
	"jun": 6, "june": 6,
	"jul": 7, "july": 7,
	"aug": 8, "august": 8,
	"sep": 9, "september": 9,
	"oct": 10, "october": 10,
	"nov": 11, "november": 11,
	"dec": 12, "december": 12,
}

// 星期名称映射
var weekNames = map[string]int{
	"sun": 0, "sunday": 0,
	"mon": 1, "monday": 1,
	"tue": 2, "tuesday": 2,
	"wed": 3, "wednesday": 3,
	"thu": 4, "thursday": 4,
	"fri": 5, "friday": 5,
	"sat": 6, "saturday": 6,
}

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
	name descriptorName
	min  int
	max  int
}

var (
	secondDescriptor = fieldDescriptor{
		name: _descriptorSecond,
		min:  0,
		max:  59,
	}
	minuteDescriptor = fieldDescriptor{
		name: _minuteDescriptor,
		min:  0,
		max:  59,
	}
	hourDescriptor = fieldDescriptor{
		name: _hourDescriptor,
		min:  0,
		max:  23,
	}
	domDescriptor = fieldDescriptor{
		name: _domDescriptor,
		min:  1,
		max:  31,
	}
	monthDescriptor = fieldDescriptor{
		name: _monthDescriptor,
		min:  1,
		max:  12,
	}
	dowDescriptor = fieldDescriptor{
		name: _dowDescriptor,
		min:  0,
		max:  6,
	}
	yearDescriptor = fieldDescriptor{
		name: _yearDescriptor,
		min:  1970,
		max:  math.MaxInt32,
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
		entry := s[indices[i][0]:indices[i][1]]
		r, err := c.parseFieldEntry(entry, desc)
		if err != nil {
			return err
		}
		ranges = append(ranges, r)
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

func (c *CronExpression) parseFieldEntry(entry string, desc fieldDescriptor) (*timeRange, error) {
	r := &timeRange{step: 1}
	entryLower := strings.ToLower(entry)

	// 快速路径：`*` 或 `?`
	if entryLower == "*" || entryLower == "?" {
		r.begin = desc.min
		r.end = desc.max
		return r, nil
	}

	// 快速路径：`*/N` 格式
	if strings.HasPrefix(entryLower, "*/") {
		stepStr := entryLower[2:]
		step, err := strconv.Atoi(stepStr)
		if err != nil {
			return nil, fmt.Errorf("invalid interval %s", entry)
		}
		if step < 1 || step > desc.max {
			return nil, fmt.Errorf("invalid interval %s", entry)
		}
		r.begin = desc.min
		r.end = desc.max
		r.step = step
		return r, nil
	}

	// 尝试解析 `value/step` 格式
	if idx := strings.IndexByte(entryLower, '/'); idx > 0 {
		valueStr := entryLower[:idx]
		stepStr := entryLower[idx+1:]
		step, err := strconv.Atoi(stepStr)
		if err != nil {
			return nil, fmt.Errorf("invalid interval %s", entry)
		}
		if step < 1 || step > desc.max {
			return nil, fmt.Errorf("invalid interval %s", entry)
		}

		// 检查是否是范围 `begin-end/step`
		if dashIdx := strings.IndexByte(valueStr, '-'); dashIdx > 0 {
			beginStr := valueStr[:dashIdx]
			endStr := valueStr[dashIdx+1:]
			begin, err := c.parseValue(beginStr, desc)
			if err != nil {
				return nil, err
			}
			end, err := c.parseValue(endStr, desc)
			if err != nil {
				return nil, err
			}
			r.begin = begin
			r.end = end
			r.step = step
			return r, nil
		}

		// `value/step` 格式
		begin, err := c.parseValue(valueStr, desc)
		if err != nil {
			return nil, err
		}
		r.begin = begin
		r.end = desc.max
		r.step = step
		return r, nil
	}

	// 尝试解析 `begin-end` 格式
	if idx := strings.IndexByte(entryLower, '-'); idx > 0 {
		beginStr := entryLower[:idx]
		endStr := entryLower[idx+1:]
		begin, err := c.parseValue(beginStr, desc)
		if err != nil {
			return nil, err
		}
		end, err := c.parseValue(endStr, desc)
		if err != nil {
			return nil, err
		}
		r.begin = begin
		r.end = end
		return r, nil
	}

	// 单个值
	value, err := c.parseValue(entryLower, desc)
	if err != nil {
		return nil, err
	}
	r.begin = value
	r.step = 0
	return r, nil
}

func (c *CronExpression) parseValue(s string, desc fieldDescriptor) (int, error) {
	// 尝试直接解析为数字
	if val, err := strconv.Atoi(s); err == nil {
		if val < desc.min || val > desc.max {
			return 0, fmt.Errorf("value %d out of range [%d, %d] for %s", val, desc.min, desc.max, desc.name)
		}
		return val, nil
	}

	// 尝试月份名称
	if desc.name == _monthDescriptor {
		if val, ok := monthNames[s]; ok {
			return val, nil
		}
	}

	// 尝试星期名称
	if desc.name == _dowDescriptor {
		if val, ok := weekNames[s]; ok {
			return val, nil
		}
	}

	// 如果都不匹配，返回错误
	return 0, fmt.Errorf("invalid value %s for %s", s, desc.name)
}

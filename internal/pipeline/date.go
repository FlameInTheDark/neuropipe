package pipeline

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"
)

func evaluateDate(nodeType string, inputs map[string]any, config map[string]any) (map[string]any, error) {
	tz := getTimezone(config)

	switch nodeType {
	case "date:now":
		return evaluateNow(tz)
	case "date:create":
		return evaluateCreate(inputs, config, tz)
	case "date:extract":
		return evaluateExtract(inputs, tz)
	case "date:format":
		return evaluateFormat(inputs, config, tz)
	case "date:parse":
		return evaluateParse(inputs, config, tz)
	case "date:compare":
		return evaluateCompare(inputs)
	case "date:add":
		return evaluateDateMath(inputs, config, tz, 1)
	case "date:subtract":
		return evaluateDateMath(inputs, config, tz, -1)
	case "date:to_unix":
		return evaluateToUnix(inputs, false)
	case "date:to_unix_ms":
		return evaluateToUnix(inputs, true)
	default:
		return nil, fmt.Errorf("unsupported date node %q", nodeType)
	}
}

func getTimezone(config map[string]any) *time.Location {
	tzStr := "local"
	if v, ok := config["timezone"].(string); ok {
		tzStr = strings.ToLower(strings.TrimSpace(v))
	}
	switch tzStr {
	case "utc":
		return time.UTC
	default:
		return time.Local
	}
}

func getNumber(inputs map[string]any, config map[string]any, key string, defaultVal float64) float64 {
	if v, ok := asNumber(inputs[key]); ok {
		return v
	}
	if v, ok := asNumber(config[key]); ok {
		return v
	}
	return defaultVal
}

func evaluateNow(tz *time.Location) (map[string]any, error) {
	now := time.Now().In(tz)
	ms := float64(now.UnixMilli())
	return map[string]any{
		"timestamp": ms,
		"iso":       now.Format(time.RFC3339Nano),
		"local":     now.Format(time.DateTime),
	}, nil
}

func evaluateCreate(inputs map[string]any, config map[string]any, tz *time.Location) (map[string]any, error) {
	year := int(getNumber(inputs, config, "year", float64(time.Now().Year())))
	month := int(getNumber(inputs, config, "month", 1))
	day := int(getNumber(inputs, config, "day", 1))
	hour := int(getNumber(inputs, config, "hour", 0))
	minute := int(getNumber(inputs, config, "minute", 0))
	second := int(getNumber(inputs, config, "second", 0))
	millisecond := int(getNumber(inputs, config, "millisecond", 0))

	if month < 1 || month > 12 {
		return nil, fmt.Errorf("month must be between 1 and 12")
	}

	dt := time.Date(year, time.Month(month), day, hour, minute, second, millisecond*1_000_000, tz)
	ms := float64(dt.UnixMilli())
	return map[string]any{
		"timestamp": ms,
		"iso":       dt.Format(time.RFC3339Nano),
	}, nil
}

func evaluateExtract(inputs map[string]any, tz *time.Location) (map[string]any, error) {
	ts, ok := asNumber(inputs["timestamp"])
	if !ok {
		return nil, fmt.Errorf("extract requires numeric timestamp input")
	}
	if math.IsNaN(ts) || math.IsInf(ts, 0) {
		return nil, fmt.Errorf("timestamp must be a finite number")
	}

	ms := int64(ts)
	dt := time.UnixMilli(ms).In(tz)

	_, week := dt.ISOWeek()
	yearDay := dt.YearDay()

	return map[string]any{
		"year":        float64(dt.Year()),
		"month":       float64(int(dt.Month())),
		"day":         float64(dt.Day()),
		"hour":        float64(dt.Hour()),
		"minute":      float64(dt.Minute()),
		"second":      float64(dt.Second()),
		"millisecond": float64(dt.Nanosecond() / 1_000_000),
		"weekday":     float64(dt.Weekday()),
		"dayOfYear":   float64(yearDay),
		"weekOfYear":  float64(week),
		"iso":         dt.Format(time.RFC3339Nano),
		"unix":        float64(dt.Unix()),
		"unixMs":      float64(ms),
	}, nil
}

func evaluateFormat(inputs map[string]any, config map[string]any, tz *time.Location) (map[string]any, error) {
	ts, ok := asNumber(inputs["timestamp"])
	if !ok {
		return nil, fmt.Errorf("format requires numeric timestamp input")
	}
	if math.IsNaN(ts) || math.IsInf(ts, 0) {
		return nil, fmt.Errorf("timestamp must be a finite number")
	}

	formatStr := "2006-01-02 15:04:05"
	if v, ok := inputs["format"].(string); ok && strings.TrimSpace(v) != "" {
		formatStr = v
	} else if v, ok := config["format"].(string); ok && strings.TrimSpace(v) != "" {
		formatStr = v
	}

	ms := int64(ts)
	dt := time.UnixMilli(ms).In(tz)
	return map[string]any{"text": dt.Format(formatStr)}, nil
}

func evaluateParse(inputs map[string]any, config map[string]any, tz *time.Location) (map[string]any, error) {
	text, ok := inputs["text"].(string)
	if !ok || strings.TrimSpace(text) == "" {
		return nil, fmt.Errorf("parse requires non-empty text input")
	}

	formatStr := ""
	if v, ok := inputs["format"].(string); ok && strings.TrimSpace(v) != "" {
		formatStr = v
	} else if v, ok := config["format"].(string); ok && strings.TrimSpace(v) != "" {
		formatStr = v
	}

	var dt time.Time
	var err error

	if formatStr != "" {
		dt, err = time.ParseInLocation(formatStr, text, tz)
	} else {
		dt, err = tryParseCommonFormats(text, tz)
	}

	if err != nil {
		return nil, fmt.Errorf("parse date: %w", err)
	}

	ms := float64(dt.UnixMilli())
	return map[string]any{
		"timestamp": ms,
		"iso":       dt.Format(time.RFC3339Nano),
	}, nil
}

func tryParseCommonFormats(text string, tz *time.Location) (time.Time, error) {
	formats := []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02T15:04:05.999999999Z07:00",
		"2006-01-02T15:04:05Z07:00",
		"2006-01-02 15:04:05.999999999",
		"2006-01-02 15:04:05",
		"2006-01-02",
		"01/02/2006",
		"02/01/2006",
		"2006/01/02",
		"Jan 2, 2006",
		"January 2, 2006",
		"2 Jan 2006",
		"2 January 2006",
	}

	for _, format := range formats {
		if dt, err := time.ParseInLocation(format, text, tz); err == nil {
			return dt, nil
		}
	}

	return time.Time{}, fmt.Errorf("unable to parse date: %q", text)
}

func evaluateCompare(inputs map[string]any) (map[string]any, error) {
	left, leftOK := asNumber(inputs["left"])
	right, rightOK := asNumber(inputs["right"])
	if !leftOK || !rightOK {
		return nil, fmt.Errorf("compare requires numeric left and right inputs")
	}
	if math.IsNaN(left) || math.IsInf(left, 0) || math.IsNaN(right) || math.IsInf(right, 0) {
		return nil, fmt.Errorf("inputs must be finite numbers")
	}

	diffMs := left - right
	return map[string]any{
		"before":      left < right,
		"after":       left > right,
		"equal":       left == right,
		"diffMs":      diffMs,
		"diffSeconds": diffMs / 1000,
		"diffMinutes": diffMs / 60000,
		"diffHours":   diffMs / 3600000,
		"diffDays":    diffMs / 86400000,
	}, nil
}

func evaluateDateMath(inputs map[string]any, config map[string]any, tz *time.Location, direction int) (map[string]any, error) {
	ts, ok := asNumber(inputs["timestamp"])
	if !ok {
		return nil, fmt.Errorf("date math requires numeric timestamp input")
	}
	if math.IsNaN(ts) || math.IsInf(ts, 0) {
		return nil, fmt.Errorf("timestamp must be a finite number")
	}

	years := int(getNumber(inputs, config, "years", 0)) * direction
	months := int(getNumber(inputs, config, "months", 0)) * direction
	days := int(getNumber(inputs, config, "days", 0)) * direction
	hours := int(getNumber(inputs, config, "hours", 0)) * direction
	minutes := int(getNumber(inputs, config, "minutes", 0)) * direction
	seconds := int(getNumber(inputs, config, "seconds", 0)) * direction
	milliseconds := int(getNumber(inputs, config, "milliseconds", 0)) * direction

	ms := int64(ts)
	dt := time.UnixMilli(ms).In(tz)

	dt = addCalendarMonths(dt, years, months)
	dt = dt.AddDate(0, 0, days)
	dt = dt.Add(time.Duration(hours)*time.Hour +
		time.Duration(minutes)*time.Minute +
		time.Duration(seconds)*time.Second +
		time.Duration(milliseconds)*time.Millisecond)

	newMs := float64(dt.UnixMilli())
	return map[string]any{
		"timestamp": newMs,
		"iso":       dt.Format(time.RFC3339Nano),
	}, nil
}

// addCalendarMonths adds calendar months without overflowing the target
// month: Jan 31 plus one month lands on the last day of February instead of
// spilling into March. Time-of-day and location are preserved.
func addCalendarMonths(dt time.Time, years, months int) time.Time {
	if years == 0 && months == 0 {
		return dt
	}
	year, month, day := dt.Date()
	total := year*12 + int(month) - 1 + years*12 + months
	targetYear := total / 12
	targetMonth := time.Month(total%12 + 1)
	lastDay := time.Date(targetYear, targetMonth+1, 0, 0, 0, 0, 0, time.UTC).Day()
	if day > lastDay {
		day = lastDay
	}
	return time.Date(targetYear, targetMonth, day, dt.Hour(), dt.Minute(), dt.Second(), dt.Nanosecond(), dt.Location())
}

func evaluateToUnix(inputs map[string]any, milliseconds bool) (map[string]any, error) {
	ts, ok := asNumber(inputs["timestamp"])
	if !ok {
		return nil, fmt.Errorf("to_unix requires numeric timestamp input")
	}
	if math.IsNaN(ts) || math.IsInf(ts, 0) {
		return nil, fmt.Errorf("timestamp must be a finite number")
	}

	var value float64
	if milliseconds {
		value = ts
	} else {
		value = ts / 1000
	}

	return map[string]any{"value": value}, nil
}

func configFloat(config map[string]any, key string) float64 {
	if v, ok := config[key].(float64); ok {
		return v
	}
	if v, ok := config[key].(int); ok {
		return float64(v)
	}
	if v, ok := config[key].(int64); ok {
		return float64(v)
	}
	if v, ok := config[key].(string); ok {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			return f
		}
	}
	return 0
}

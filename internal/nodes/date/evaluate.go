// Package date owns the pure behavior of first-party date nodes. It depends
// only on the node invocation contract, not the Blueprint interpreter.
package date

import (
	"fmt"
	"math"
	"strings"
	"time"
)

func location(config map[string]any) *time.Location {
	value, _ := config["timezone"].(string)
	if strings.EqualFold(strings.TrimSpace(value), "utc") {
		return time.UTC
	}
	return time.Local
}

func create(inputs, config map[string]any, timezone *time.Location) (map[string]any, error) {
	year := int(numberOr(inputs, config, "year", float64(time.Now().Year())))
	month := int(numberOr(inputs, config, "month", 1))
	day := int(numberOr(inputs, config, "day", 1))
	hour := int(numberOr(inputs, config, "hour", 0))
	minute := int(numberOr(inputs, config, "minute", 0))
	second := int(numberOr(inputs, config, "second", 0))
	millisecond := int(numberOr(inputs, config, "millisecond", 0))
	if month < 1 || month > 12 {
		return nil, fmt.Errorf("month must be between 1 and 12")
	}
	value := time.Date(year, time.Month(month), day, hour, minute, second, millisecond*1_000_000, timezone)
	return timestampOutput(value), nil
}

func extract(inputs map[string]any, timezone *time.Location) (map[string]any, error) {
	timestamp, err := timestamp(inputs, "extract")
	if err != nil {
		return nil, err
	}
	value := time.UnixMilli(int64(timestamp)).In(timezone)
	_, week := value.ISOWeek()
	return map[string]any{
		"year": float64(value.Year()), "month": float64(value.Month()), "day": float64(value.Day()), "hour": float64(value.Hour()),
		"minute": float64(value.Minute()), "second": float64(value.Second()), "millisecond": float64(value.Nanosecond() / 1_000_000),
		"weekday": float64(value.Weekday()), "dayOfYear": float64(value.YearDay()), "weekOfYear": float64(week),
		"iso": value.Format(time.RFC3339Nano), "unix": float64(value.Unix()), "unixMs": float64(value.UnixMilli()),
	}, nil
}

func format(inputs, config map[string]any, timezone *time.Location) (map[string]any, error) {
	timestamp, err := timestamp(inputs, "format")
	if err != nil {
		return nil, err
	}
	layout := text(inputs, "format")
	if layout == "" {
		layout = text(config, "format")
	}
	if layout == "" {
		layout = "2006-01-02 15:04:05"
	}
	return map[string]any{"text": time.UnixMilli(int64(timestamp)).In(timezone).Format(layout)}, nil
}

func parse(inputs, config map[string]any, timezone *time.Location) (map[string]any, error) {
	value := text(inputs, "text")
	if value == "" {
		return nil, fmt.Errorf("parse requires non-empty text input")
	}
	layout := text(inputs, "format")
	if layout == "" {
		layout = text(config, "format")
	}
	var parsed time.Time
	var err error
	if layout != "" {
		parsed, err = time.ParseInLocation(layout, value, timezone)
	} else {
		parsed, err = parseCommon(value, timezone)
	}
	if err != nil {
		return nil, fmt.Errorf("parse date: %w", err)
	}
	return timestampOutput(parsed), nil
}

func compare(inputs map[string]any) (map[string]any, error) {
	left, leftOK := number(inputs["left"])
	right, rightOK := number(inputs["right"])
	if !leftOK || !rightOK {
		return nil, fmt.Errorf("compare requires numeric left and right inputs")
	}
	if !finite(left) || !finite(right) {
		return nil, fmt.Errorf("inputs must be finite numbers")
	}
	difference := left - right
	return map[string]any{"before": left < right, "after": left > right, "equal": left == right, "diffMs": difference, "diffSeconds": difference / 1_000, "diffMinutes": difference / 60_000, "diffHours": difference / 3_600_000, "diffDays": difference / 86_400_000}, nil
}

func add(inputs, config map[string]any, timezone *time.Location, direction int) (map[string]any, error) {
	timestamp, err := timestamp(inputs, "date math")
	if err != nil {
		return nil, err
	}
	value := time.UnixMilli(int64(timestamp)).In(timezone)
	years, months := int(numberOr(inputs, config, "years", 0))*direction, int(numberOr(inputs, config, "months", 0))*direction
	value = addMonths(value, years, months).AddDate(0, 0, int(numberOr(inputs, config, "days", 0))*direction)
	duration := time.Duration(int(numberOr(inputs, config, "hours", 0))*direction)*time.Hour +
		time.Duration(int(numberOr(inputs, config, "minutes", 0))*direction)*time.Minute +
		time.Duration(int(numberOr(inputs, config, "seconds", 0))*direction)*time.Second +
		time.Duration(int(numberOr(inputs, config, "milliseconds", 0))*direction)*time.Millisecond
	return timestampOutput(value.Add(duration)), nil
}

func toUnix(inputs map[string]any, milliseconds bool) (map[string]any, error) {
	timestamp, err := timestamp(inputs, "to_unix")
	if err != nil {
		return nil, err
	}
	if !milliseconds {
		timestamp /= 1_000
	}
	return map[string]any{"value": timestamp}, nil
}

func timestamp(inputs map[string]any, operation string) (float64, error) {
	value, ok := number(inputs["timestamp"])
	if !ok {
		return 0, fmt.Errorf("%s requires numeric timestamp input", operation)
	}
	if !finite(value) {
		return 0, fmt.Errorf("timestamp must be a finite number")
	}
	return value, nil
}

func timestampOutput(value time.Time) map[string]any {
	return map[string]any{"timestamp": float64(value.UnixMilli()), "iso": value.Format(time.RFC3339Nano)}
}

func numberOr(inputs, config map[string]any, key string, fallback float64) float64 {
	if value, ok := number(inputs[key]); ok {
		return value
	}
	if value, ok := number(config[key]); ok {
		return value
	}
	return fallback
}

func number(value any) (float64, bool) {
	switch typed := value.(type) {
	case float64:
		return typed, true
	case float32:
		return float64(typed), true
	case int:
		return float64(typed), true
	case int64:
		return float64(typed), true
	default:
		return 0, false
	}
}

func finite(value float64) bool { return !math.IsNaN(value) && !math.IsInf(value, 0) }

func text(values map[string]any, key string) string {
	value, _ := values[key].(string)
	return strings.TrimSpace(value)
}

func addMonths(value time.Time, years, months int) time.Time {
	if years == 0 && months == 0 {
		return value
	}
	year, month, day := value.Date()
	total := year*12 + int(month) - 1 + years*12 + months
	targetYear, targetMonth := total/12, time.Month(total%12+1)
	lastDay := time.Date(targetYear, targetMonth+1, 0, 0, 0, 0, 0, time.UTC).Day()
	if day > lastDay {
		day = lastDay
	}
	return time.Date(targetYear, targetMonth, day, value.Hour(), value.Minute(), value.Second(), value.Nanosecond(), value.Location())
}

func parseCommon(value string, timezone *time.Location) (time.Time, error) {
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339, "2006-01-02T15:04:05.999999999Z07:00", "2006-01-02T15:04:05Z07:00", "2006-01-02 15:04:05.999999999", "2006-01-02 15:04:05", "2006-01-02", "01/02/2006", "02/01/2006", "2006/01/02", "Jan 2, 2006", "January 2, 2006", "2 Jan 2006", "2 January 2006"} {
		if parsed, err := time.ParseInLocation(layout, value, timezone); err == nil {
			return parsed, nil
		}
	}
	return time.Time{}, fmt.Errorf("unable to parse date: %q", value)
}

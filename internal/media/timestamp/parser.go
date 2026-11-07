package timestamp

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var (
	sixPartDateTimePattern = regexp.MustCompile(`(?i)(\d{4})[-_]?((?:0[1-9])|(?:1[0-2]))[-_]?((?:0[1-9])|(?:[12][0-9])|(?:3[01]))(?:[T\s_-]?)([0-2][0-9])[-_:]?([0-5][0-9])[-_:]?([0-5][0-9])`)
	eightSixPattern        = regexp.MustCompile(`(?i)(\d{8})[_-](\d{6})`)
)

func ParseMetadataTime(raw string) (*time.Time, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}

	layouts := []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02 15:04:05",
		"2006-01-02 15:04:05 -0700",
		"2006-01-02 15:04:05 MST",
		"2006-01-02T15:04:05.000000-0700",
		"2006-01-02T15:04:05.000000Z0700",
		"2006-01-02T15:04:05",
	}

	for _, layout := range layouts {
		if parsed, err := time.Parse(layout, raw); err == nil {
			t := parsed.UTC()
			return &t, nil
		}
		if parsed, err := time.ParseInLocation(layout, raw, time.UTC); err == nil {
			t := parsed.UTC()
			return &t, nil
		}
	}

	return nil, fmt.Errorf("unsupported metadata time format: %s", raw)
}

func ParseFilenameTime(path string, location *time.Location) (*time.Time, bool) {
	if location == nil {
		location = time.Local
	}

	base := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))

	if match := sixPartDateTimePattern.FindStringSubmatch(base); len(match) == 7 {
		year, _ := strconv.Atoi(match[1])
		month, _ := strconv.Atoi(match[2])
		day, _ := strconv.Atoi(match[3])
		hour, _ := strconv.Atoi(match[4])
		minute, _ := strconv.Atoi(match[5])
		second, _ := strconv.Atoi(match[6])

		parsed := time.Date(year, time.Month(month), day, hour, minute, second, 0, location)
		u := parsed.UTC()
		return &u, true
	}

	if match := eightSixPattern.FindStringSubmatch(base); len(match) == 3 {
		datePart := match[1]
		timePart := match[2]
		parsed, err := time.ParseInLocation("20060102150405", datePart+timePart, location)
		if err == nil {
			u := parsed.UTC()
			return &u, true
		}
	}

	return nil, false
}

package csvimport

import (
	"encoding/csv"
	"io"
	"strconv"
	"strings"
	"time"
)

var requiredColumns = []string{"name", "datetime", "seconds_before", "seconds_after"}

func ParseCuts(reader io.Reader) ParseResult {
	result := ParseResult{
		Rows:   make([]CutRow, 0),
		Issues: make([]Issue, 0),
	}

	csvReader := csv.NewReader(reader)
	csvReader.TrimLeadingSpace = true
	csvReader.FieldsPerRecord = -1

	header, err := csvReader.Read()
	if err == io.EOF {
		result.Issues = append(result.Issues, Issue{Row: 1, Column: "header", Message: "CSV is empty"})
		return result
	}
	if err != nil {
		result.Issues = append(result.Issues, Issue{Row: 1, Column: "header", Message: "invalid CSV header"})
		return result
	}

	normalizedHeader := make([]string, len(header))
	indices := map[string]int{}
	for idx, value := range header {
		normalized := strings.ToLower(strings.TrimSpace(strings.TrimPrefix(value, "\ufeff")))
		normalizedHeader[idx] = normalized
		if normalized != "" {
			if _, exists := indices[normalized]; !exists {
				indices[normalized] = idx
			}
		}
	}

	for _, column := range requiredColumns {
		if _, ok := indices[column]; !ok {
			result.Issues = append(result.Issues, Issue{Row: 1, Column: column, Message: "missing required column"})
		}
	}
	if len(result.Issues) > 0 {
		return result
	}

	rowNumber := 1
	for {
		record, readErr := csvReader.Read()
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			issueRow := rowNumber + 1
			if parseErr, ok := readErr.(*csv.ParseError); ok && parseErr.Line > 0 {
				issueRow = parseErr.Line
			}
			result.Issues = append(result.Issues, Issue{Row: issueRow, Column: "row", Message: "invalid CSV row"})
			break
		}

		rowNumber++
		if isBlankRecord(record) {
			continue
		}
		if len(record) != len(normalizedHeader) {
			result.Issues = append(result.Issues, Issue{Row: rowNumber, Column: "row", Message: "row has an unexpected number of columns"})
			continue
		}

		nameValue := strings.TrimSpace(record[indices["name"]])
		datetimeValue := strings.TrimSpace(record[indices["datetime"]])
		secondsBeforeRaw := strings.TrimSpace(record[indices["seconds_before"]])
		secondsAfterRaw := strings.TrimSpace(record[indices["seconds_after"]])

		rowIssues := make([]Issue, 0)
		if nameValue == "" {
			rowIssues = append(rowIssues, Issue{Row: rowNumber, Column: "name", Message: "name is required"})
		}
		if datetimeValue == "" {
			rowIssues = append(rowIssues, Issue{Row: rowNumber, Column: "datetime", Message: "datetime is required"})
		} else if _, err := parseDateTime(datetimeValue); err != nil {
			rowIssues = append(rowIssues, Issue{Row: rowNumber, Column: "datetime", Message: "datetime has an invalid format", Value: datetimeValue})
		}

		secondsBefore, beforeErr := parseNonNegativeInt(secondsBeforeRaw)
		if beforeErr != nil {
			rowIssues = append(rowIssues, Issue{Row: rowNumber, Column: "seconds_before", Message: "seconds_before must be a non-negative integer", Value: secondsBeforeRaw})
		}
		secondsAfter, afterErr := parseNonNegativeInt(secondsAfterRaw)
		if afterErr != nil {
			rowIssues = append(rowIssues, Issue{Row: rowNumber, Column: "seconds_after", Message: "seconds_after must be a non-negative integer", Value: secondsAfterRaw})
		}
		if beforeErr == nil && afterErr == nil && secondsBefore == 0 && secondsAfter == 0 {
			rowIssues = append(rowIssues, Issue{Row: rowNumber, Column: "seconds_before", Message: "seconds_before and seconds_after cannot both be 0"})
		}

		if len(rowIssues) > 0 {
			result.Issues = append(result.Issues, rowIssues...)
			continue
		}

		result.Rows = append(result.Rows, CutRow{
			Row:           rowNumber,
			Name:          nameValue,
			DateTime:      datetimeValue,
			SecondsBefore: secondsBefore,
			SecondsAfter:  secondsAfter,
		})
	}

	if len(result.Rows) == 0 && len(result.Issues) == 0 {
		result.Issues = append(result.Issues, Issue{Row: 1, Column: "row", Message: "CSV has no data rows"})
	}

	return result
}

func parseNonNegativeInt(raw string) (int, error) {
	value, err := strconv.Atoi(raw)
	if err != nil || value < 0 {
		return 0, strconv.ErrSyntax
	}
	return value, nil
}

func parseDateTime(raw string) (time.Time, error) {
	layoutsWithLocation := []string{
		"2006-01-02T15:04",
		"2006-01-02T15:04:05",
		"2006-01-02 15:04",
		"2006-01-02 15:04:05",
	}
	layoutsWithZone := []string{time.RFC3339Nano, time.RFC3339}

	for _, layout := range layoutsWithZone {
		if value, err := time.Parse(layout, raw); err == nil {
			return value.In(time.Local), nil
		}
	}
	for _, layout := range layoutsWithLocation {
		if value, err := time.ParseInLocation(layout, raw, time.Local); err == nil {
			return value, nil
		}
	}

	return time.Time{}, strconv.ErrSyntax
}

func isBlankRecord(record []string) bool {
	for _, value := range record {
		if strings.TrimSpace(value) != "" {
			return false
		}
	}
	return true
}

// Package sku generates labels matching ebay-bulk-upload's daily packing SKUs.
package sku

import (
	"encoding/csv"
	"fmt"
	"io"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
	_ "time/tzdata"
)

const MaxBatch = 100000

var pattern = regexp.MustCompile(`^(\d{4}-\d{2}-\d{2})-([0-9]{4,})$`)

type Label struct {
	Date     string
	Sequence int
}

func (l Label) String() string { return fmt.Sprintf("%s-%04d", l.Date, l.Sequence) }
func DateValid(date string) bool {
	t, err := time.Parse("2006-01-02", date)
	return err == nil && t.Format("2006-01-02") == date
}
func PacificToday(now time.Time) string {
	loc, _ := time.LoadLocation("America/Los_Angeles")
	return now.In(loc).Format("2006-01-02")
}
func Parse(s string) (Label, error) {
	m := pattern.FindStringSubmatch(strings.TrimSpace(s))
	if m == nil || !DateValid(m[1]) {
		return Label{}, fmt.Errorf("invalid daily SKU: %q", s)
	}
	n, err := strconv.Atoi(m[2])
	if err != nil || n < 1 {
		return Label{}, fmt.Errorf("invalid sequence in %q", s)
	}
	l := Label{m[1], n}
	if l.String() != strings.TrimSpace(s) {
		return Label{}, fmt.Errorf("SKU padding does not match listing format: %q", s)
	}
	return l, nil
}

// Build returns each requested SKU once, in numerical order. Imported mode never
// invents labels for gaps in the exported listings.
func Build(date string, start, end int, imported []Label, onlyImported bool) ([]Label, error) {
	if !DateValid(date) {
		return nil, fmt.Errorf("choose a real listing date in YYYY-MM-DD format")
	}
	if start < 1 || end < start {
		return nil, fmt.Errorf("start must be at least 1 and end must be at least start")
	}
	if onlyImported {
		found := map[int]bool{}
		for _, l := range imported {
			if l.Date == date && l.Sequence >= start && l.Sequence <= end {
				found[l.Sequence] = true
			}
		}
		ns := make([]int, 0, len(found))
		for n := range found {
			ns = append(ns, n)
		}
		sort.Ints(ns)
		if len(ns) == 0 {
			return nil, fmt.Errorf("no imported SKUs match this date and range")
		}
		if len(ns) > MaxBatch {
			return nil, fmt.Errorf("split jobs into at most %d labels", MaxBatch)
		}
		labels := make([]Label, len(ns))
		for i, n := range ns {
			labels[i] = Label{date, n}
		}
		return labels, nil
	}
	if end-start >= MaxBatch {
		return nil, fmt.Errorf("split jobs into at most %d labels", MaxBatch)
	}
	labels := make([]Label, end-start+1)
	for i := range labels {
		labels[i] = Label{date, start + i}
	}
	return labels, nil
}
func ImportCSV(r io.Reader) ([]Label, int, error) {
	reader := csv.NewReader(r)
	reader.FieldsPerRecord = -1
	header, err := reader.Read()
	if err != nil {
		return nil, 0, fmt.Errorf("read CSV header: %w", err)
	}
	index := -1
	for i, h := range header {
		if strings.EqualFold(strings.TrimSpace(strings.TrimPrefix(h, "\ufeff")), "Custom label (SKU)") {
			index = i
			break
		}
	}
	if index < 0 {
		return nil, 0, fmt.Errorf("CSV needs the Custom label (SKU) column from your listing export")
	}
	labels := []Label{}
	seen := map[string]bool{}
	ignored := 0
	for {
		row, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, ignored, fmt.Errorf("read CSV: %w", err)
		}
		if index >= len(row) {
			ignored++
			continue
		}
		l, err := Parse(row[index])
		if err != nil {
			ignored++
			continue
		}
		if !seen[l.String()] {
			seen[l.String()] = true
			labels = append(labels, l)
		}
	}
	sort.Slice(labels, func(i, j int) bool {
		if labels[i].Date != labels[j].Date {
			return labels[i].Date < labels[j].Date
		}
		return labels[i].Sequence < labels[j].Sequence
	})
	if len(labels) == 0 {
		return nil, ignored, fmt.Errorf("no daily date-and-sequence SKUs found; regenerate old PACK-/CARD- listings first")
	}
	return labels, ignored, nil
}

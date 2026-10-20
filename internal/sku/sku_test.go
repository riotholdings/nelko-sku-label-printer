package sku

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestRangeAndFormat(t *testing.T) {
	ls, err := Build("2026-10-20", 1, 25, nil, false)
	if err != nil || len(ls) != 25 || ls[0].String() != "2026-10-20-0001" || ls[24].String() != "2026-10-20-0025" {
		t.Fatalf("%v %v", ls, err)
	}
	l, err := Parse("2026-10-20-10000")
	if err != nil || l.Sequence != 10000 {
		t.Fatal(l, err)
	}
}
func TestInvalid(t *testing.T) {
	for _, s := range []string{"2026-02-30-0001", "2026-10-20-0000", "2026-10-20-1", "PACK-abc"} {
		if _, err := Parse(s); err == nil {
			t.Fatal(s)
		}
	}
	for _, r := range [][2]int{{0, 2}, {4, 3}, {1, MaxBatch + 1}} {
		if _, err := Build("2026-10-20", r[0], r[1], nil, false); err == nil {
			t.Fatal(r)
		}
	}
}
func TestCSV(t *testing.T) {
	csv := "\ufeffTitle,Custom label (SKU),Description\r\nCard,2026-10-20-0003,\"line1\nline2\"\r\nPack,2026-10-20-0001,hi\r\nRepeat,2026-10-20-0001,hi\r\nOld,PACK-abc,hi\r\nOther,2026-10-21-0001,hi\r\n"
	labels, ignored, err := ImportCSV(strings.NewReader(csv))
	if err != nil || len(labels) != 3 || ignored != 1 {
		t.Fatal(labels, ignored, err)
	}
	selected, err := Build("2026-10-20", 1, 3, labels, true)
	if err != nil || len(selected) != 2 || selected[1].Sequence != 3 {
		t.Fatal(selected, err)
	}
	if _, _, err := ImportCSV(strings.NewReader("Title\nCard\n")); err == nil {
		t.Fatal("missing header accepted")
	}
}
func TestPacificDate(t *testing.T) {
	now, _ := time.Parse(time.RFC3339, "2026-10-21T01:00:00Z")
	if PacificToday(now) != "2026-10-20" {
		t.Fatal(PacificToday(now))
	}
}
func TestQueueCopies(t *testing.T) {
	labels, _ := Build("2026-10-20", 1, 2, nil, false)
	out := []string{}
	n, err := Run(context.Background(), labels, 2, 0, func(l Label) error { out = append(out, l.String()); return nil }, nil)
	if err != nil || n != 4 || out[0] != out[1] || out[2] != out[3] {
		t.Fatal(out, n, err)
	}
}
func TestQueueStopsWithoutRetry(t *testing.T) {
	labels, _ := Build("2026-10-20", 1, 3, nil, false)
	calls := 0
	n, err := Run(context.Background(), labels, 1, 0, func(Label) error {
		calls++
		if calls == 2 {
			return errors.New("disconnected")
		}
		return nil
	}, nil)
	if n != 1 || calls != 2 || err == nil {
		t.Fatal(n, calls, err)
	}
}
func TestCancellation(t *testing.T) {
	labels, _ := Build("2026-10-20", 1, 3, nil, false)
	ctx, cancel := context.WithCancel(context.Background())
	n, err := Run(ctx, labels, 1, time.Hour, func(Label) error { cancel(); return nil }, nil)
	if n != 1 || !errors.Is(err, context.Canceled) {
		t.Fatal(n, err)
	}
}

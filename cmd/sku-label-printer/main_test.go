package main

import (
	"fyne.io/fyne/v2"
	ftest "fyne.io/fyne/v2/test"
	"image/png"
	"nelko-print/internal/sku"
	"os"
	"testing"
)

func TestUIAndImportedDaySelection(t *testing.T) {
	a := ftest.NewApp()
	defer a.Quit()
	w := a.NewWindow("SKU labels")
	defer w.Close()
	u := &ui{app: a, window: w}
	w.SetContent(u.build())
	w.Resize(fyne.NewSize(840, 1000))
	w.Show()
	u.imported = []sku.Label{{Date: "2026-10-20", Sequence: 1}, {Date: "2026-10-20", Sequence: 3}, {Date: "2026-10-21", Sequence: 7}}
	u.only.SetChecked(true)
	u.date.SetText("2026-10-21")
	labels, copies, err := u.labels()
	if err != nil || copies != 1 || len(labels) != 1 || labels[0].Sequence != 7 {
		t.Fatal(labels, copies, err)
	}
	u.date.SetText("2026-10-20")
	labels, _, err = u.labels()
	if err != nil || len(labels) != 2 || u.end.Text != "3" {
		t.Fatal(labels, err)
	}
	u.updatePreview()
	if u.preview.Image == nil {
		t.Fatal("no preview")
	}
	if output := os.Getenv("SKU_SCREENSHOT_PATH"); output != "" {
		f, err := os.Create(output)
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		if err := png.Encode(f, w.Canvas().Capture()); err != nil {
			t.Fatal(err)
		}
	}
}

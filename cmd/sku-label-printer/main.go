package main

import (
	"context"
	"errors"
	"fmt"
	"image/png"
	"math"
	"os"
	"strconv"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/storage"
	"fyne.io/fyne/v2/widget"
	"nelko-print/internal/imaging"
	"nelko-print/internal/printer"
	"nelko-print/internal/sku"
	"nelko-print/internal/tspl"
)

const version = "1.0.0"

type ui struct {
	app                                                             fyne.App
	window                                                          fyne.Window
	port                                                            *printer.Printer
	date                                                            *widget.SelectEntry
	start, end, copies, delay                                       *widget.Entry
	ports                                                           *widget.SelectEntry
	connect, refresh, print, cancel, importBtn, testBtn, previewBtn *widget.Button
	size                                                            *widget.Select
	density                                                         *widget.Slider
	only                                                            *widget.Check
	summary, status                                                 *widget.Label
	progress                                                        *widget.ProgressBar
	preview                                                         *canvas.Image
	imported                                                        []sku.Label
	busy                                                            bool
	stop                                                            context.CancelFunc
}

func main() {
	a := app.NewWithID("holdings.riot.nelko-sku-printer")
	u := &ui{app: a, window: a.NewWindow("Riot Holdings - Nelko SKU Labels v" + version)}
	u.window.Resize(fyne.NewSize(840, 720))
	u.window.SetContent(u.build())
	u.window.SetCloseIntercept(func() {
		if u.busy {
			u.status.SetText("Cancel and wait for the current send before closing.")
			if u.stop != nil {
				u.stop()
			}
			return
		}
		if u.port != nil {
			_ = u.port.Close()
		}
		u.window.SetCloseIntercept(nil)
		u.window.Close()
	})
	// Optional PNG preview for build/visual checks; does not connect or print.
	if len(os.Args) == 3 && os.Args[1] == "--preview-png" {
		img, err := imaging.RenderSKU("2026-10-20", 1, 96, 284)
		if err != nil {
			panic(err)
		}
		f, err := os.Create(os.Args[2])
		if err != nil {
			panic(err)
		}
		defer f.Close()
		if err := png.Encode(f, imaging.RotatePreviewForDisplay(img)); err != nil {
			panic(err)
		}
		return
	}
	u.window.ShowAndRun()
}
func entry(value string) *widget.Entry { e := widget.NewEntry(); e.SetText(value); return e }
func (u *ui) build() fyne.CanvasObject {
	u.status = widget.NewLabel("Pair your P21 in Windows Bluetooth settings, then select its outgoing COM port.")
	u.status.Wrapping = fyne.TextWrapWord
	u.ports = widget.NewSelectEntry([]string{})
	u.ports.SetPlaceHolder("COM3 (outgoing Bluetooth port)")
	u.ports.SetText(u.app.Preferences().String("port"))
	u.refresh = widget.NewButton("Refresh ports", u.refreshPorts)
	u.connect = widget.NewButton("Connect", u.connectPort)
	u.date = widget.NewSelectEntry([]string{})
	loc, _ := time.LoadLocation("America/Los_Angeles")
	today := time.Now().In(loc)
	dates := []string{}
	for i := -14; i <= 30; i++ {
		dates = append(dates, today.AddDate(0, 0, i).Format("2006-01-02"))
	}
	u.date.SetOptions(dates)
	u.date.SetText(sku.PacificToday(time.Now()))
	u.start = entry("1")
	u.end = entry("10")
	u.copies = entry("1")
	u.delay = entry("1.0")
	u.size = widget.NewSelect([]string{}, nil)
	sizes := []string{}
	for _, s := range tspl.AllSizes {
		sizes = append(sizes, s.Name)
	}
	u.size.Options = sizes
	u.size.SetSelected(tspl.Label14x40.Name)
	u.density = widget.NewSlider(0, 15)
	u.density.Step = 1
	u.density.SetValue(10)
	u.summary = widget.NewLabel("")
	u.summary.Wrapping = fyne.TextWrapWord
	u.progress = widget.NewProgressBar()
	u.preview = canvas.NewImageFromImage(nil)
	u.preview.FillMode = canvas.ImageFillContain
	u.preview.SetMinSize(fyne.NewSize(330, 140))
	u.only = widget.NewCheck("Use only SKUs present in imported CSV (skip gaps)", func(bool) { u.updateSummary() })
	u.importBtn = widget.NewButton("Import listing CSV...", u.importCSV)
	u.previewBtn = widget.NewButton("Preview first label", u.updatePreview)
	u.print = widget.NewButton("Print batch", func() { u.begin(false) })
	u.print.Importance = widget.HighImportance
	u.testBtn = widget.NewButton("Print one test label", func() { u.begin(true) })
	u.cancel = widget.NewButton("Cancel remaining labels", func() {
		if u.stop != nil {
			u.stop()
			u.status.SetText("Cancelling after the current transmission; buffered labels may still print.")
		}
	})
	u.cancel.Disable()
	changed := func(string) { u.updateSummary() }
	u.date.OnChanged = func(string) {
		if len(u.imported) > 0 {
			u.setImportedRange()
		}
		u.updateSummary()
	}
	u.start.OnChanged = changed
	u.end.OnChanged = changed
	u.copies.OnChanged = changed
	form := widget.NewForm(widget.NewFormItem("Listing date (Pacific)", u.date), widget.NewFormItem("Start number", u.start), widget.NewFormItem("End number (inclusive)", u.end), widget.NewFormItem("Copies of each SKU", u.copies), widget.NewFormItem("Pause between labels (seconds)", u.delay), widget.NewFormItem("Label stock", u.size), widget.NewFormItem("Density", u.density))
	note := widget.NewLabel("Date + sequence match ebay-bulk-upload. Import CSV to list available dates; no server login or API key is needed.")
	note.Wrapping = fyne.TextWrapWord
	content := container.NewVBox(widget.NewLabelWithStyle("Batch SKU labels", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}), note, container.NewBorder(nil, nil, widget.NewLabel("Printer port"), container.NewHBox(u.refresh, u.connect), u.ports), widget.NewSeparator(), u.importBtn, u.only, form, u.summary, u.previewBtn, u.preview, container.NewHBox(u.testBtn, u.print, u.cancel), u.progress, u.status)
	u.refreshPorts()
	u.updateSummary()
	return container.NewVScroll(content)
}
func (u *ui) labelSize() tspl.LabelSize {
	for _, s := range tspl.AllSizes {
		if s.Name == u.size.Selected {
			return s
		}
	}
	return tspl.Label14x40
}
func number(text, name string) (int, error) {
	if text == "" || strings.Trim(text, "0123456789") != "" {
		return 0, fmt.Errorf("%s must be a positive whole number", name)
	}
	n, err := strconv.Atoi(text)
	if err != nil || n < 1 {
		return 0, fmt.Errorf("%s must be a positive whole number", name)
	}
	return n, nil
}
func (u *ui) labels() ([]sku.Label, int, error) {
	start, err := number(u.start.Text, "Start")
	if err != nil {
		return nil, 0, err
	}
	end, err := number(u.end.Text, "End")
	if err != nil {
		return nil, 0, err
	}
	copies, err := number(u.copies.Text, "Copies")
	if err != nil {
		return nil, 0, err
	}
	if copies > 100 {
		return nil, 0, fmt.Errorf("copies must be 1-100")
	}
	labels, err := sku.Build(u.date.Text, start, end, u.imported, u.only.Checked)
	if err == nil && len(labels) > sku.MaxBatch/copies {
		err = fmt.Errorf("split jobs into at most %d physical labels", sku.MaxBatch)
	}
	return labels, copies, err
}
func (u *ui) updateSummary() {
	if u.summary == nil {
		return
	}
	labels, copies, err := u.labels()
	if err != nil {
		u.summary.SetText(err.Error())
		return
	}
	u.summary.SetText(fmt.Sprintf("%s through %s | %d SKUs | %d physical labels", labels[0], labels[len(labels)-1], len(labels), len(labels)*copies))
}
func (u *ui) updatePreview() {
	labels, _, err := u.labels()
	if err != nil {
		dialog.ShowError(err, u.window)
		return
	}
	s := u.labelSize()
	img, err := imaging.RenderSKU(labels[0].Date, labels[0].Sequence, s.PixelW, s.PixelH)
	if err != nil {
		dialog.ShowError(err, u.window)
		return
	}
	u.preview.Image = imaging.RotatePreviewForDisplay(img)
	u.preview.Refresh()
}
func (u *ui) refreshPorts() {
	ports, err := printer.ListSerialPorts()
	if err != nil {
		u.status.SetText("Port scan: " + err.Error())
		return
	}
	u.ports.SetOptions(ports)
}
func (u *ui) connectPort() {
	if u.busy {
		return
	}
	if u.port != nil {
		_ = u.port.Close()
		u.port = nil
		u.connect.SetText("Connect")
		u.status.SetText("Disconnected")
		return
	}
	port := strings.TrimSpace(u.ports.Text)
	if port == "" {
		dialog.ShowError(fmt.Errorf("select or enter the outgoing Bluetooth COM port"), u.window)
		return
	}
	u.connect.Disable()
	u.status.SetText("Opening " + port + "...")
	go func() {
		p, err := printer.Connect(port)
		fyne.Do(func() {
			u.connect.Enable()
			if err != nil {
				dialog.ShowError(err, u.window)
				u.status.SetText("Connection failed")
				return
			}
			u.port = p
			u.connect.SetText("Disconnect")
			u.app.Preferences().SetString("port", port)
			u.status.SetText("Port open: " + port + ". Print one test label before the batch.")
		})
	}()
}
func (u *ui) importCSV() {
	d := dialog.NewFileOpen(func(reader fyne.URIReadCloser, err error) {
		if err != nil {
			dialog.ShowError(err, u.window)
			return
		}
		if reader == nil {
			return
		}
		defer reader.Close()
		labels, ignored, err := sku.ImportCSV(reader)
		if err != nil {
			dialog.ShowError(err, u.window)
			return
		}
		u.imported = labels
		dates := []string{}
		for _, l := range labels {
			if len(dates) == 0 || dates[len(dates)-1] != l.Date {
				dates = append(dates, l.Date)
			}
		}
		u.date.SetOptions(dates)
		u.date.SetText(dates[len(dates)-1])
		u.setImportedRange()
		u.only.SetChecked(true)
		u.status.SetText(fmt.Sprintf("Imported %d distinct SKUs; %d non-daily/invalid rows ignored.", len(labels), ignored))
		u.updateSummary()
	}, u.window)
	d.SetFilter(storage.NewExtensionFileFilter([]string{".csv"}))
	d.Show()
}
func (u *ui) setImportedRange() {
	min, max := 0, 0
	for _, l := range u.imported {
		if l.Date == u.date.Text {
			if min == 0 || l.Sequence < min {
				min = l.Sequence
			}
			if l.Sequence > max {
				max = l.Sequence
			}
		}
	}
	if min > 0 {
		u.start.SetText(strconv.Itoa(min))
		u.end.SetText(strconv.Itoa(max))
	}
}
func (u *ui) setBusy(b bool) {
	u.busy = b
	for _, w := range []fyne.Disableable{u.date, u.start, u.end, u.copies, u.delay, u.size, u.density, u.only, u.importBtn, u.previewBtn, u.ports, u.refresh, u.connect, u.testBtn, u.print} {
		if b {
			w.Disable()
		} else {
			w.Enable()
		}
	}
	if b {
		u.cancel.Enable()
	} else {
		u.cancel.Disable()
	}
}
func (u *ui) begin(test bool) {
	if u.busy {
		return
	}
	if u.port == nil {
		dialog.ShowError(printer.ErrNotConnected, u.window)
		return
	}
	labels, copies, err := u.labels()
	if err != nil {
		dialog.ShowError(err, u.window)
		return
	}
	seconds, err := strconv.ParseFloat(u.delay.Text, 64)
	if err != nil || math.IsNaN(seconds) || seconds < 0.2 || seconds > 60 {
		dialog.ShowError(fmt.Errorf("pause must be 0.2 to 60 seconds"), u.window)
		return
	}
	if test {
		labels = labels[:1]
		copies = 1
	}
	size, density, p := u.labelSize(), int(u.density.Value), u.port
	dialog.ShowConfirm("Print SKU labels", fmt.Sprintf("Send %d label(s), from %s through %s, to %s? Stock: %s. A test label is an extra physical label.", len(labels)*copies, labels[0], labels[len(labels)-1], p.PortName(), size.Name), func(ok bool) {
		if !ok {
			return
		}
		ctx, cancel := context.WithCancel(context.Background())
		u.stop = cancel
		u.setBusy(true)
		u.progress.SetValue(0)
		go func() {
			sent, err := sku.Run(ctx, labels, copies, time.Duration(seconds*float64(time.Second)), func(l sku.Label) error {
				img, err := imaging.RenderSKU(l.Date, l.Sequence, size.PixelW, size.PixelH)
				if err != nil {
					return err
				}
				bitmap := imaging.ToMonochrome(img, size.PixelW, size.PixelH, 128, true)
				return p.Print(tspl.BuildPrintJob(size, density, bitmap, 1))
			}, func(n, total int) {
				fyne.Do(func() {
					u.progress.SetValue(float64(n) / float64(total))
					u.status.SetText(fmt.Sprintf("Sent %d/%d labels to printer", n, total))
				})
			})
			cancel()
			fyne.Do(func() {
				u.stop = nil
				u.setBusy(false)
				if err != nil {
					u.status.SetText(fmt.Sprintf("Stopped after %d sends. Check the last physical label before resuming.", sent))
					if !errors.Is(err, context.Canceled) {
						dialog.ShowError(err, u.window)
					}
				} else {
					u.status.SetText(fmt.Sprintf("Batch sent: %d labels. Verify the physical output; the printer does not confirm each label.", sent))
				}
			})
		}()
	}, u.window)
}

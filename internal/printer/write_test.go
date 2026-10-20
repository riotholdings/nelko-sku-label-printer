package printer

import (
	"bytes"
	"errors"
	"go.bug.st/serial"
	"io"
	"testing"
	"time"
)

type partialWriter struct{ bytes.Buffer }

func (w *partialWriter) Write(p []byte) (int, error) {
	if len(p) > 2 {
		p = p[:2]
	}
	return w.Buffer.Write(p)
}

type stalledWriter struct{}

func (stalledWriter) Write([]byte) (int, error) { return 0, nil }
func TestWriteAll(t *testing.T) {
	w := &partialWriter{}
	if err := writeAll(w, []byte("complete-job")); err != nil || w.String() != "complete-job" {
		t.Fatal(w.String(), err)
	}
	if !errors.Is(writeAll(stalledWriter{}, []byte("x")), io.ErrShortWrite) {
		t.Fatal("zero write accepted")
	}
}

type burstWriter struct {
	bytes.Buffer
	sizes []int
}

func (w *burstWriter) Write(p []byte) (int, error) {
	w.sizes = append(w.sizes, len(p))
	return w.Buffer.Write(p)
}
func TestPacedBitmapTransfer(t *testing.T) {
	// Include every byte value: binary data must survive chunk boundaries exactly.
	data := make([]byte, 3408)
	for i := range data {
		data[i] = byte(i)
	}
	w := &burstWriter{}
	pauses := 0
	err := writePaced(w, data, func(d time.Duration) {
		if d != 20*time.Millisecond {
			t.Fatal(d)
		}
		pauses++
	})
	if err != nil || !bytes.Equal(w.Bytes(), data) {
		t.Fatal("bitmap changed", err)
	}
	for _, n := range w.sizes {
		if n > 128 {
			t.Fatal("oversized Bluetooth burst", n)
		}
	}
	if pauses != len(w.sizes)-1 {
		t.Fatal(pauses, w.sizes)
	}
	partial := &partialWriter{}
	if err := writePaced(partial, data, func(time.Duration) {}); err != nil || !bytes.Equal(partial.Bytes(), data) {
		t.Fatal("lost partial write", err)
	}
	if !errors.Is(writePaced(stalledWriter{}, data, func(time.Duration) {}), io.ErrShortWrite) {
		t.Fatal("stalled transfer accepted")
	}
}

type drainPort struct {
	serial.Port
	buffer  bytes.Buffer
	drained bool
}

func (p *drainPort) Write(data []byte) (int, error) { return p.buffer.Write(data) }
func (p *drainPort) Drain() error                   { p.drained = true; return nil }
func TestPrintDrainsBeforeReturning(t *testing.T) {
	port := &drainPort{}
	p := &Printer{port: port}
	data := []byte("complete bitmap and PRINT")
	if err := p.Print(data); err != nil {
		t.Fatal(err)
	}
	if !port.drained || !bytes.Equal(port.buffer.Bytes(), append([]byte("\x1b!o"), data...)) {
		t.Fatal("print did not finish transport")
	}
}

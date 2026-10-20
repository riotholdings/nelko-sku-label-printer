package printer

import (
	"bytes"
	"errors"
	"io"
	"testing"
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

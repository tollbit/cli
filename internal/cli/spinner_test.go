package cli

import (
	"bytes"
	"strings"
	"testing"
	"time"
)

func TestStartSpinner_disabledNoOutput(t *testing.T) {
	var buf bytes.Buffer
	stop := startSpinner(&buf, false, "Fetching")
	stop()
	if buf.Len() != 0 {
		t.Fatalf("disabled spinner wrote %q", buf.String())
	}
}

func TestStartSpinner_nonTTYWriterNoOutput(t *testing.T) {
	var buf bytes.Buffer
	stop := startSpinner(&buf, true, "Fetching")
	time.Sleep(50 * time.Millisecond)
	stop()
	if buf.Len() != 0 {
		t.Fatalf("non-TTY spinner wrote %q", buf.String())
	}
}

func TestRunSpinner_writesLabelAndBraille(t *testing.T) {
	var buf bytes.Buffer
	stop := runSpinner(&buf, "Fetching", 10*time.Millisecond)
	time.Sleep(35 * time.Millisecond)
	stop()

	out := buf.String()
	if !strings.Contains(out, "Fetching") {
		t.Fatalf("expected label in output, got %q", out)
	}
	found := false
	for _, frame := range spinnerFrames {
		if strings.Contains(out, frame) {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected a braille frame in output, got %q", out)
	}
	if !strings.Contains(out, "\033[K") {
		t.Fatalf("expected clear sequence on stop, got %q", out)
	}
}

func TestRunSpinner_stopIdempotent(t *testing.T) {
	var buf bytes.Buffer
	stop := runSpinner(&buf, "Fetching", time.Hour)
	stop()
	stop() // must not panic or hang
}

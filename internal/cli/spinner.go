package cli

import (
	"fmt"
	"io"
	"os"
	"sync"
	"time"

	"github.com/mattn/go-isatty"
)

var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

const spinnerInterval = 80 * time.Millisecond

// startSpinner writes an in-place braille spinner to w when enabled and w is a TTY.
// Returns a stop function that clears the line; always safe to call (including as defer).
func startSpinner(w io.Writer, enabled bool, label string) (stop func()) {
	if !enabled || !writerIsTTY(w) {
		return func() {}
	}
	return runSpinner(w, label, spinnerInterval)
}

func writerIsTTY(w io.Writer) bool {
	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	fd := f.Fd()
	return isatty.IsTerminal(fd) || isatty.IsCygwinTerminal(fd)
}

func runSpinner(w io.Writer, label string, interval time.Duration) (stop func()) {
	done := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(1)

	go func() {
		defer wg.Done()
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		i := 0
		writeFrame := func() {
			frame := spinnerFrames[i%len(spinnerFrames)]
			i++
			fmt.Fprintf(w, "\r%s %s", label, frame)
		}
		writeFrame()
		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				writeFrame()
			}
		}
	}()

	var once sync.Once
	return func() {
		once.Do(func() {
			close(done)
			wg.Wait()
			// Clear the spinner line so subsequent output is not interleaved.
			fmt.Fprintf(w, "\r\033[K")
		})
	}
}

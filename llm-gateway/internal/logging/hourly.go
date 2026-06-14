package logging

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type HourlyWriter struct {
	dir  string
	mu   sync.Mutex
	hour string
	file *os.File
}

func NewHourlyWriter(dir string) (*HourlyWriter, error) {
	if dir == "" {
		dir = "logs"
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	return &HourlyWriter{dir: dir}, nil
}

func (w *HourlyWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	hour := time.Now().Format("20060102-15")
	if w.file == nil || w.hour != hour {
		if err := w.rotate(hour); err != nil {
			return 0, err
		}
	}
	return w.file.Write(p)
}

func (w *HourlyWriter) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.file == nil {
		return nil
	}
	err := w.file.Close()
	w.file = nil
	return err
}

func (w *HourlyWriter) rotate(hour string) error {
	if w.file != nil {
		_ = w.file.Close()
		w.file = nil
	}
	path := filepath.Join(w.dir, fmt.Sprintf("%s.log", hour))
	file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	w.hour = hour
	w.file = file
	return nil
}

func MultiOutput(dir string, fallback io.Writer) (*HourlyWriter, io.Writer, error) {
	writer, err := NewHourlyWriter(dir)
	if err != nil {
		return nil, nil, err
	}
	return writer, io.MultiWriter(fallback, writer), nil
}

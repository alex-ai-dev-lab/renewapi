package helper

import (
	"bufio"
	"fmt"
	"io"
	"strings"

	"github.com/tidwall/gjson"
)

// SSEScanner joins multiline JSON data fields and supplies an event type when
// the JSON omits it. Complete single-line payloads remain immediate for legacy
// upstreams that omit blank separators. The buffer limit applies to whole frames.
type SSEScanner struct {
	scanner   *bufio.Scanner
	onLine    func(string) bool
	pending   strings.Builder
	eventType string
	data      string
	err       error
	finished  bool
}

func NewSSEScanner(reader io.Reader) *SSEScanner {
	return newSSEScanner(NewStreamScanner(reader), nil)
}

func newSSEScanner(scanner *bufio.Scanner, onLine func(string) bool) *SSEScanner {
	return &SSEScanner{scanner: scanner, onLine: onLine}
}

func (s *SSEScanner) emit(data string) bool {
	s.data = applySSEEventTypeFallback(data, s.eventType)
	s.eventType = ""
	return true
}

func (s *SSEScanner) flush() bool {
	if s.pending.Len() == 0 {
		s.eventType = ""
		return false
	}
	data := s.pending.String()
	s.pending.Reset()
	return s.emit(data)
}

func (s *SSEScanner) Scan() bool {
	if s.finished {
		return false
	}
	for s.scanner.Scan() {
		line := s.scanner.Text()
		if s.onLine != nil && !s.onLine(line) {
			s.finished = true
			return false
		}
		if line == "" {
			if s.flush() {
				return true
			}
			continue
		}
		if eventType, ok := ParseSSEField(line, "event"); ok {
			s.eventType = eventType
			continue
		}
		data, ok := ParseSSEField(line, "data")
		if !ok {
			if line == "[DONE]" {
				return s.emit(line)
			}
			continue
		}
		if data == "" {
			continue
		}
		jsonStart := strings.HasPrefix(data, "{") || strings.HasPrefix(data, "[")
		if s.pending.Len() == 0 && (!jsonStart || gjson.Valid(data) || data == "[DONE]") {
			return s.emit(data)
		}
		if s.pending.Len()+len(data)+1 > getScannerBufferSize() {
			s.err = fmt.Errorf("SSE frame exceeds maximum buffer size")
			s.finished = true
			return false
		}
		if s.pending.Len() > 0 {
			s.pending.WriteByte('\n')
		}
		s.pending.WriteString(data)
	}
	s.finished = true
	if s.scanner.Err() != nil {
		return false
	}
	return s.flush()
}

func (s *SSEScanner) Text() string { return s.data }

func (s *SSEScanner) Err() error {
	if s.err != nil {
		return s.err
	}
	return s.scanner.Err()
}

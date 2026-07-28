package common

import (
	"fmt"
	"strings"
	"sync"
	"time"
)

type StreamEndReason string

const (
	StreamEndReasonNone             StreamEndReason = ""
	StreamEndReasonDone             StreamEndReason = "done"
	StreamEndReasonTimeout          StreamEndReason = "timeout"
	StreamEndReasonFirstByteTimeout StreamEndReason = "first_byte_timeout"
	StreamEndReasonClientGone       StreamEndReason = "client_gone"
	StreamEndReasonScannerErr       StreamEndReason = "scanner_error"
	StreamEndReasonHandlerStop      StreamEndReason = "handler_stop"
	StreamEndReasonEOF              StreamEndReason = "eof"
	StreamEndReasonPanic            StreamEndReason = "panic"
	StreamEndReasonPingFail         StreamEndReason = "ping_fail"
	StreamEndReasonWriteError       StreamEndReason = "write_error"
)

type StreamSemanticEnd string

const (
	StreamSemanticNone       StreamSemanticEnd = ""
	StreamSemanticCompleted  StreamSemanticEnd = "completed"
	StreamSemanticFailed     StreamSemanticEnd = "failed"
	StreamSemanticIncomplete StreamSemanticEnd = "incomplete"
)

type StreamTerminationPolicy string

const (
	StreamTerminationPolicyDefault         StreamTerminationPolicy = "default"
	StreamTerminationPolicyOpenAIResponses StreamTerminationPolicy = "openai_responses"
)

type StreamAttemptCode string

const (
	StreamAttemptOK              StreamAttemptCode = "ok"
	StreamAttemptEmptyResponse   StreamAttemptCode = "empty_response"
	StreamAttemptBadResponseBody StreamAttemptCode = "bad_response_body"
	StreamAttemptIncomplete      StreamAttemptCode = "incomplete_stream"
	StreamAttemptFailed          StreamAttemptCode = "failed"
	StreamAttemptWriteError      StreamAttemptCode = "write_error"
	StreamAttemptClientGone      StreamAttemptCode = "client_gone"
	StreamAttemptTimeout         StreamAttemptCode = "timeout"
)

type StreamAttemptOutcome struct {
	Code                  StreamAttemptCode `json:"code"`
	TransportEnd          StreamEndReason   `json:"transport_end"`
	SemanticEnd           StreamSemanticEnd `json:"semantic_end"`
	RawFrameCount         int               `json:"raw_frames"`
	ValidEventCount       int               `json:"valid_events"`
	InvalidEventCount     int               `json:"invalid_events"`
	ForwardedEventCount   int               `json:"forwarded_events"`
	TerminalSeen          bool              `json:"terminal_seen"`
	ClientCommitted       bool              `json:"client_committed"`
	RetryableBeforeCommit bool              `json:"retryable_precommit"`
	Summary               string            `json:"summary"`
}

const maxStreamErrorEntries = 20

type StreamErrorEntry struct {
	Message   string
	Timestamp time.Time
}

type StreamStatus struct {
	mu sync.Mutex

	EndReason StreamEndReason
	EndError  error
	endSet    bool

	TransportEnd StreamEndReason
	SemanticEnd  StreamSemanticEnd
	Policy       StreamTerminationPolicy

	RawFrameCount       int
	ValidEventCount     int
	InvalidEventCount   int
	ForwardedEventCount int
	TerminalSeen        bool
	ClientCommitted     bool

	Errors     []StreamErrorEntry
	ErrorCount int
}

func NewStreamStatus() *StreamStatus {
	return &StreamStatus{Policy: StreamTerminationPolicyDefault}
}

func NewStreamStatusFromExisting(existing *StreamStatus) *StreamStatus {
	next := NewStreamStatus()
	if existing == nil {
		return next
	}
	existing.mu.Lock()
	defer existing.mu.Unlock()
	next.Policy = existing.Policy
	next.ErrorCount = existing.ErrorCount
	if len(existing.Errors) > 0 {
		next.Errors = append(next.Errors, existing.Errors...)
	}
	return next
}

// SetEndReason preserves the historical first-writer behavior for provider
// handlers. Transport readers use SetTransportEnd so EOF cannot race a
// semantic terminal observed by the handler.
func (s *StreamStatus) SetEndReason(reason StreamEndReason, err error) {
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.endSet {
		return
	}
	s.EndReason = reason
	s.EndError = err
	s.endSet = true
}

func (s *StreamStatus) SetPolicy(policy StreamTerminationPolicy) {
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if policy == "" {
		policy = StreamTerminationPolicyDefault
	}
	s.Policy = policy
}

func (s *StreamStatus) SetTransportEnd(reason StreamEndReason, err error) {
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if streamEndPriority(reason) >= streamEndPriority(s.TransportEnd) {
		s.TransportEnd = reason
		if err != nil || s.EndError == nil {
			s.EndError = err
		}
	}
}

func streamEndPriority(reason StreamEndReason) int {
	switch reason {
	case StreamEndReasonClientGone, StreamEndReasonWriteError:
		return 100
	case StreamEndReasonPanic, StreamEndReasonScannerErr, StreamEndReasonPingFail:
		return 90
	case StreamEndReasonTimeout, StreamEndReasonFirstByteTimeout:
		return 80
	case StreamEndReasonDone:
		return 20
	case StreamEndReasonEOF:
		return 10
	default:
		return 0
	}
}

func (s *StreamStatus) ObserveRawFrame() {
	if s == nil {
		return
	}
	s.mu.Lock()
	s.RawFrameCount++
	s.mu.Unlock()
}

func (s *StreamStatus) AcceptEvent(_ string) {
	if s == nil {
		return
	}
	s.mu.Lock()
	s.ValidEventCount++
	s.mu.Unlock()
}

func (s *StreamStatus) RejectEvent(err error) {
	if s == nil {
		return
	}
	s.mu.Lock()
	s.InvalidEventCount++
	if err != nil {
		s.recordErrorLocked(err.Error())
	}
	s.mu.Unlock()
}

func (s *StreamStatus) MarkTerminal(success bool, err error) {
	if s == nil {
		return
	}
	s.mu.Lock()
	s.TerminalSeen = true
	if success {
		s.SemanticEnd = StreamSemanticCompleted
	} else {
		s.SemanticEnd = StreamSemanticFailed
		if err != nil {
			s.EndError = err
		}
	}
	s.mu.Unlock()
}

func (s *StreamStatus) MarkForwarded() {
	if s == nil {
		return
	}
	s.mu.Lock()
	s.ForwardedEventCount++
	s.ClientCommitted = true
	s.mu.Unlock()
}

func (s *StreamStatus) MarkClientCommitted() {
	if s == nil {
		return
	}
	s.mu.Lock()
	s.ClientCommitted = true
	s.mu.Unlock()
}

func (s *StreamStatus) IsClientCommitted() bool {
	if s == nil {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.ClientCommitted
}

func (s *StreamStatus) RecordError(msg string) {
	if s == nil {
		return
	}
	s.mu.Lock()
	s.recordErrorLocked(msg)
	s.mu.Unlock()
}

func (s *StreamStatus) recordErrorLocked(msg string) {
	s.ErrorCount++
	if len(s.Errors) < maxStreamErrorEntries {
		s.Errors = append(s.Errors, StreamErrorEntry{Message: msg, Timestamp: time.Now()})
	}
}

func (s *StreamStatus) HasErrors() bool {
	if s == nil {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.ErrorCount > 0
}

func (s *StreamStatus) TotalErrorCount() int {
	if s == nil {
		return 0
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.ErrorCount
}

// Finalize runs only after scanner and handler workers have stopped.
func (s *StreamStatus) Finalize() StreamAttemptOutcome {
	if s == nil {
		return StreamAttemptOutcome{Code: StreamAttemptOK}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.Policy == StreamTerminationPolicyOpenAIResponses &&
		s.SemanticEnd == StreamSemanticNone && s.ValidEventCount > 0 {
		s.SemanticEnd = StreamSemanticIncomplete
	}
	if !s.endSet {
		switch {
		case s.SemanticEnd == StreamSemanticFailed:
			s.EndReason = StreamEndReasonHandlerStop
		case s.SemanticEnd == StreamSemanticCompleted:
			s.EndReason = StreamEndReasonDone
		case s.TransportEnd != StreamEndReasonNone:
			s.EndReason = s.TransportEnd
		default:
			s.EndReason = StreamEndReasonEOF
		}
		s.endSet = true
	}
	return s.outcomeLocked()
}

func (s *StreamStatus) Outcome() StreamAttemptOutcome {
	if s == nil {
		return StreamAttemptOutcome{Code: StreamAttemptOK}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.outcomeLocked()
}

func (s *StreamStatus) outcomeLocked() StreamAttemptOutcome {
	out := StreamAttemptOutcome{
		TransportEnd:        s.TransportEnd,
		SemanticEnd:         s.SemanticEnd,
		RawFrameCount:       s.RawFrameCount,
		ValidEventCount:     s.ValidEventCount,
		InvalidEventCount:   s.InvalidEventCount,
		ForwardedEventCount: s.ForwardedEventCount,
		TerminalSeen:        s.TerminalSeen,
		ClientCommitted:     s.ClientCommitted,
	}
	switch {
	case s.TransportEnd == StreamEndReasonClientGone:
		out.Code = StreamAttemptClientGone
	case s.TransportEnd == StreamEndReasonWriteError || (s.TransportEnd == StreamEndReasonPingFail && s.ClientCommitted):
		out.Code = StreamAttemptWriteError
	case s.TransportEnd == StreamEndReasonScannerErr || s.TransportEnd == StreamEndReasonPanic:
		out.Code = StreamAttemptBadResponseBody
	case s.SemanticEnd == StreamSemanticFailed || s.EndReason == StreamEndReasonHandlerStop:
		out.Code = StreamAttemptFailed
	case s.TransportEnd == StreamEndReasonTimeout || s.TransportEnd == StreamEndReasonFirstByteTimeout:
		out.Code = StreamAttemptTimeout
	case s.Policy == StreamTerminationPolicyOpenAIResponses && s.InvalidEventCount > 0 && s.ValidEventCount == 0:
		out.Code = StreamAttemptBadResponseBody
	case s.Policy == StreamTerminationPolicyOpenAIResponses && s.ValidEventCount == 0:
		out.Code = StreamAttemptEmptyResponse
	case s.Policy == StreamTerminationPolicyOpenAIResponses && (!s.TerminalSeen || s.SemanticEnd != StreamSemanticCompleted):
		out.Code = StreamAttemptIncomplete
	case s.RawFrameCount == 0 && s.TransportEnd == StreamEndReasonEOF:
		out.Code = StreamAttemptEmptyResponse
	default:
		out.Code = StreamAttemptOK
	}
	out.RetryableBeforeCommit = out.Code != StreamAttemptOK &&
		out.Code != StreamAttemptClientGone &&
		out.Code != StreamAttemptWriteError &&
		!out.ClientCommitted
	out.Summary = fmt.Sprintf("code=%s transport=%s semantic=%s raw=%d valid=%d invalid=%d forwarded=%d terminal=%t committed=%t",
		out.Code, out.TransportEnd, out.SemanticEnd, out.RawFrameCount, out.ValidEventCount,
		out.InvalidEventCount, out.ForwardedEventCount, out.TerminalSeen, out.ClientCommitted)
	return out
}

func (s *StreamStatus) IsNormalEnd() bool {
	if s == nil {
		return true
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.Policy == StreamTerminationPolicyOpenAIResponses {
		return s.outcomeLocked().Code == StreamAttemptOK
	}
	return s.EndReason == StreamEndReasonDone ||
		s.EndReason == StreamEndReasonEOF ||
		s.EndReason == StreamEndReasonHandlerStop
}

func (s *StreamStatus) Summary() string {
	if s == nil {
		return "StreamStatus<nil>"
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	b := &strings.Builder{}
	fmt.Fprintf(b, "reason=%s transport=%s semantic=%s raw=%d valid=%d invalid=%d forwarded=%d terminal=%t committed=%t",
		s.EndReason, s.TransportEnd, s.SemanticEnd, s.RawFrameCount, s.ValidEventCount,
		s.InvalidEventCount, s.ForwardedEventCount, s.TerminalSeen, s.ClientCommitted)
	if s.EndError != nil {
		fmt.Fprintf(b, " end_error=%q", s.EndError.Error())
	}
	if s.ErrorCount > 0 {
		fmt.Fprintf(b, " soft_errors=%d", s.ErrorCount)
	}
	return b.String()
}

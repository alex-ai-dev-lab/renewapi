package common

import (
	"fmt"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestStreamStatus_SetEndReason_FirstWins(t *testing.T) {
	t.Parallel()
	s := NewStreamStatus()

	s.SetEndReason(StreamEndReasonDone, nil)
	s.SetEndReason(StreamEndReasonTimeout, nil)
	s.SetEndReason(StreamEndReasonClientGone, fmt.Errorf("context canceled"))

	assert.Equal(t, StreamEndReasonDone, s.EndReason)
	assert.Nil(t, s.EndError)
}

func TestStreamStatus_SetEndReason_WithError(t *testing.T) {
	t.Parallel()
	s := NewStreamStatus()

	expectedErr := fmt.Errorf("read: connection reset")
	s.SetEndReason(StreamEndReasonScannerErr, expectedErr)

	assert.Equal(t, StreamEndReasonScannerErr, s.EndReason)
	assert.Equal(t, expectedErr, s.EndError)
}

func TestStreamStatus_SetEndReason_NilSafe(t *testing.T) {
	t.Parallel()
	var s *StreamStatus
	s.SetEndReason(StreamEndReasonDone, nil)
}

func TestStreamStatus_SetEndReason_Concurrent(t *testing.T) {
	t.Parallel()
	s := NewStreamStatus()

	reasons := []StreamEndReason{
		StreamEndReasonDone,
		StreamEndReasonTimeout,
		StreamEndReasonClientGone,
		StreamEndReasonScannerErr,
		StreamEndReasonHandlerStop,
		StreamEndReasonEOF,
		StreamEndReasonPanic,
		StreamEndReasonPingFail,
		StreamEndReasonWriteError,
		StreamEndReasonFirstByteTimeout,
	}

	var wg sync.WaitGroup
	for _, r := range reasons {
		wg.Add(1)
		go func(reason StreamEndReason) {
			defer wg.Done()
			s.SetEndReason(reason, nil)
		}(r)
	}
	wg.Wait()

	assert.NotEqual(t, StreamEndReasonNone, s.EndReason)
}

func TestStreamStatus_RecordError_Basic(t *testing.T) {
	t.Parallel()
	s := NewStreamStatus()

	s.RecordError("bad json")
	s.RecordError("another bad json")
	s.RecordError("client gone")

	assert.True(t, s.HasErrors())
	assert.Equal(t, 3, s.TotalErrorCount())
	assert.Len(t, s.Errors, 3)
}

func TestStreamStatus_RecordError_CapAtMax(t *testing.T) {
	t.Parallel()
	s := NewStreamStatus()

	for i := 0; i < 30; i++ {
		s.RecordError(fmt.Sprintf("error_%d", i))
	}

	assert.Equal(t, maxStreamErrorEntries, len(s.Errors))
	assert.Equal(t, 30, s.TotalErrorCount())
}

func TestStreamStatus_RecordError_NilSafe(t *testing.T) {
	t.Parallel()
	var s *StreamStatus
	s.RecordError("should not panic")
}

func TestStreamStatus_RecordError_Concurrent(t *testing.T) {
	t.Parallel()
	s := NewStreamStatus()

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			s.RecordError(fmt.Sprintf("error_%d", idx))
		}(i)
	}
	wg.Wait()

	assert.Equal(t, 100, s.TotalErrorCount())
	assert.LessOrEqual(t, len(s.Errors), maxStreamErrorEntries)
}

func TestNewStreamStatusFromExisting_PreservesErrors(t *testing.T) {
	t.Parallel()
	existing := NewStreamStatus()
	existing.RecordError("pre-existing error")
	existing.SetEndReason(StreamEndReasonDone, nil)

	next := NewStreamStatusFromExisting(existing)

	assert.Equal(t, StreamEndReasonNone, next.EndReason)
	assert.Equal(t, 1, next.TotalErrorCount())
	assert.True(t, next.HasErrors())
}

func TestStreamStatus_HasErrors_Empty(t *testing.T) {
	t.Parallel()
	s := NewStreamStatus()
	assert.False(t, s.HasErrors())
	assert.Equal(t, 0, s.TotalErrorCount())
}

func TestStreamStatus_HasErrors_NilSafe(t *testing.T) {
	t.Parallel()
	var s *StreamStatus
	assert.False(t, s.HasErrors())
	assert.Equal(t, 0, s.TotalErrorCount())
}

func TestStreamStatus_IsNormalEnd(t *testing.T) {
	t.Parallel()
	tests := []struct {
		reason StreamEndReason
		normal bool
	}{
		{StreamEndReasonDone, true},
		{StreamEndReasonEOF, true},
		{StreamEndReasonHandlerStop, true},
		{StreamEndReasonTimeout, false},
		{StreamEndReasonFirstByteTimeout, false},
		{StreamEndReasonClientGone, false},
		{StreamEndReasonScannerErr, false},
		{StreamEndReasonPanic, false},
		{StreamEndReasonPingFail, false},
		{StreamEndReasonNone, false},
	}
	for _, tt := range tests {
		s := NewStreamStatus()
		s.SetEndReason(tt.reason, nil)
		assert.Equal(t, tt.normal, s.IsNormalEnd(), "reason=%s", tt.reason)
	}
}

func TestStreamStatus_IsNormalEnd_NilSafe(t *testing.T) {
	t.Parallel()
	var s *StreamStatus
	assert.True(t, s.IsNormalEnd())
}

func TestStreamStatus_Summary(t *testing.T) {
	t.Parallel()

	s := NewStreamStatus()
	s.SetEndReason(StreamEndReasonDone, nil)
	summary := s.Summary()
	assert.Contains(t, summary, "reason=done")
	assert.NotContains(t, summary, "soft_errors")

	s2 := NewStreamStatus()
	s2.SetEndReason(StreamEndReasonTimeout, nil)
	s2.RecordError("bad json")
	s2.RecordError("write failed")
	summary2 := s2.Summary()
	assert.Contains(t, summary2, "reason=timeout")
	assert.Contains(t, summary2, "soft_errors=2")
}

func TestStreamStatus_Summary_NilSafe(t *testing.T) {
	t.Parallel()
	var s *StreamStatus
	assert.Equal(t, "StreamStatus<nil>", s.Summary())
}

func TestStreamStatus_OpenAIResponsesOutcomes(t *testing.T) {
	tests := []struct {
		name  string
		build func(*StreamStatus)
		code  StreamAttemptCode
		retry bool
	}{
		{
			name: "empty eof",
			build: func(status *StreamStatus) {
				status.SetTransportEnd(StreamEndReasonEOF, nil)
			},
			code:  StreamAttemptEmptyResponse,
			retry: true,
		},
		{
			name: "malformed only",
			build: func(status *StreamStatus) {
				status.ObserveRawFrame()
				status.RejectEvent(fmt.Errorf("bad json"))
				status.SetTransportEnd(StreamEndReasonEOF, nil)
			},
			code:  StreamAttemptBadResponseBody,
			retry: true,
		},
		{
			name: "valid without terminal",
			build: func(status *StreamStatus) {
				status.ObserveRawFrame()
				status.AcceptEvent("response.output_text.delta")
				status.SetTransportEnd(StreamEndReasonEOF, nil)
			},
			code:  StreamAttemptIncomplete,
			retry: true,
		},
		{
			name: "completed then eof",
			build: func(status *StreamStatus) {
				status.ObserveRawFrame()
				status.AcceptEvent("response.completed")
				status.MarkTerminal(true, nil)
				status.SetTransportEnd(StreamEndReasonEOF, nil)
			},
			code: StreamAttemptOK,
		},
		{
			name: "downstream write failure",
			build: func(status *StreamStatus) {
				status.SetTransportEnd(StreamEndReasonWriteError, fmt.Errorf("broken pipe"))
			},
			code: StreamAttemptWriteError,
		},
		{
			name: "handler failure wins over eof",
			build: func(status *StreamStatus) {
				status.ObserveRawFrame()
				status.AcceptEvent("response.failed")
				status.MarkTerminal(false, fmt.Errorf("failed"))
				status.SetEndReason(StreamEndReasonHandlerStop, fmt.Errorf("failed"))
				status.SetTransportEnd(StreamEndReasonEOF, nil)
			},
			code:  StreamAttemptFailed,
			retry: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			status := NewStreamStatus()
			status.SetPolicy(StreamTerminationPolicyOpenAIResponses)
			tt.build(status)
			outcome := status.Finalize()
			assert.Equal(t, tt.code, outcome.Code)
			assert.Equal(t, tt.retry, outcome.RetryableBeforeCommit)
		})
	}
}

func TestStreamStatus_CommitDisablesRetry(t *testing.T) {
	status := NewStreamStatus()
	status.SetPolicy(StreamTerminationPolicyOpenAIResponses)
	status.ObserveRawFrame()
	status.AcceptEvent("response.output_text.delta")
	status.MarkClientCommitted()
	status.SetTransportEnd(StreamEndReasonEOF, nil)

	outcome := status.Finalize()
	assert.Equal(t, StreamAttemptIncomplete, outcome.Code)
	assert.False(t, outcome.RetryableBeforeCommit)
}

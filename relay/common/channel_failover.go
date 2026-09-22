package common

import (
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/types"
)

const MaxChannelSwitches = 5
const ChannelFailoverContextKey = "relay_channel_failover"

type ChannelAttemptRecord struct {
	ChannelID         int    `json:"channel_id"`
	ChannelName       string `json:"channel_name"`
	Priority          int64  `json:"priority"`
	Attempt           int    `json:"attempt"`
	SwitchCount       int    `json:"switch_count"`
	StatusCode        int    `json:"status_code"`
	RealError         string `json:"real_error,omitempty"`
	TimeoutStage      string `json:"timeout_stage,omitempty"`
	ElapsedMS         int64  `json:"elapsed_ms"`
	UpstreamRequestID string `json:"upstream_request_id,omitempty"`
}

// ChannelFailoverState 的生命周期覆盖整个请求，模型、分组和恢复路由不能重置预算。
type ChannelFailoverState struct {
	AttemptCount        int
	SwitchCount         int
	MaxSwitches         int
	AttemptedChannelIDs map[int]bool
	LastInternalError   *types.NewAPIError
	AttemptRecords      []ChannelAttemptRecord
	startedAt           time.Time
}

func NewChannelFailoverState() *ChannelFailoverState {
	return &ChannelFailoverState{MaxSwitches: MaxChannelSwitches, AttemptedChannelIDs: make(map[int]bool)}
}

func (s *ChannelFailoverState) CanAttempt() bool {
	return s != nil && s.AttemptCount < s.MaxSwitches+1
}

func (s *ChannelFailoverState) Begin(channelID int, name string, priority int64) bool {
	if !s.CanAttempt() || s.AttemptedChannelIDs[channelID] {
		return false
	}
	s.AttemptedChannelIDs[channelID] = true
	s.AttemptCount++
	s.SwitchCount = s.AttemptCount - 1
	s.startedAt = time.Now()
	s.AttemptRecords = append(s.AttemptRecords, ChannelAttemptRecord{
		ChannelID: channelID, ChannelName: name, Priority: priority,
		Attempt: s.AttemptCount, SwitchCount: s.SwitchCount,
	})
	return true
}

func (s *ChannelFailoverState) Finish(err *types.NewAPIError, upstreamRequestID, timeoutStage string, secrets ...string) {
	if s == nil || len(s.AttemptRecords) == 0 {
		return
	}
	record := &s.AttemptRecords[len(s.AttemptRecords)-1]
	record.ElapsedMS = time.Since(s.startedAt).Milliseconds()
	record.UpstreamRequestID = upstreamRequestID
	record.TimeoutStage = timeoutStage
	record.StatusCode = 200
	if err != nil {
		s.LastInternalError = err
		record.StatusCode = err.StatusCode
		record.RealError = common.RedactErrorCredentials(err.Error(), secrets...)
	}
}

// SnapshotSuccess 在适配器生成结算日志时补齐本次成功记录，历史失败记录保持独立。
func (s *ChannelFailoverState) SnapshotSuccess(upstreamRequestID string) []ChannelAttemptRecord {
	if s == nil {
		return nil
	}
	records := append([]ChannelAttemptRecord(nil), s.AttemptRecords...)
	if len(records) > 0 && records[len(records)-1].StatusCode == 0 {
		records[len(records)-1].StatusCode = 200
		records[len(records)-1].ElapsedMS = time.Since(s.startedAt).Milliseconds()
		records[len(records)-1].UpstreamRequestID = upstreamRequestID
	}
	return records
}

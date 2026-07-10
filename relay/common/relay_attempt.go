package common

import "github.com/QuantumNous/new-api/types"

// RelayAttemptSnapshot isolates request-format routing state from retries.
// Client-facing RelayFormat is intentionally not mutable and is not captured.
type RelayAttemptSnapshot struct {
	RelayMode               int
	RequestURLPath          string
	FinalRequestRelayFormat types.RelayFormat
	RequestConversionChain  []types.RelayFormat
	UpstreamRequestBodySize int64
}

func (info *RelayInfo) SnapshotAttempt() RelayAttemptSnapshot {
	if info == nil {
		return RelayAttemptSnapshot{}
	}
	return RelayAttemptSnapshot{
		RelayMode:               info.RelayMode,
		RequestURLPath:          info.RequestURLPath,
		FinalRequestRelayFormat: info.FinalRequestRelayFormat,
		RequestConversionChain:  append([]types.RelayFormat(nil), info.RequestConversionChain...),
		UpstreamRequestBodySize: info.UpstreamRequestBodySize,
	}
}

func (info *RelayInfo) RestoreAttempt(snapshot RelayAttemptSnapshot) {
	if info == nil {
		return
	}
	info.RelayMode = snapshot.RelayMode
	info.RequestURLPath = snapshot.RequestURLPath
	info.FinalRequestRelayFormat = snapshot.FinalRequestRelayFormat
	info.RequestConversionChain = append([]types.RelayFormat(nil), snapshot.RequestConversionChain...)
	info.UpstreamRequestBodySize = snapshot.UpstreamRequestBodySize
}

package requestguard

import (
	"crypto/sha256"
	"strings"
	"time"
)

const internalRequestHeader = "X-RequestGuard-Internal"

type DecisionKind string

const (
	DecisionAllow       DecisionKind = "allow"
	DecisionFlag        DecisionKind = "flag"
	DecisionBlock       DecisionKind = "block"
	DecisionUnavailable DecisionKind = "unavailable"
	DecisionInvalid     DecisionKind = "invalid"
)

type Segment struct {
	Role string `json:"role"`
	Text string `json:"text"`
}

type Snapshot struct {
	Segments  []Segment `json:"segments"`
	RuneCount int       `json:"rune_count"`
	Truncated bool      `json:"truncated"`
	Digest    [32]byte  `json:"-"`
}

func (s Snapshot) Text() string {
	var builder strings.Builder
	for i, segment := range s.Segments {
		if i > 0 {
			builder.WriteString("\n")
		}
		if segment.Role != "" {
			builder.WriteString("[")
			builder.WriteString(segment.Role)
			builder.WriteString("]\n")
		}
		builder.WriteString(segment.Text)
	}
	return builder.String()
}

func limitSnapshot(snapshot Snapshot, limit int) Snapshot {
	if limit <= 0 || snapshot.RuneCount <= limit {
		return snapshot
	}
	builder := newSnapshotBuilder(limit)
	for _, segment := range snapshot.Segments {
		if !builder.append(segment.Role, segment.Text) {
			break
		}
	}
	limited := builder.snapshot()
	limited.Truncated = true
	return limited
}

type Result struct {
	Kind          DecisionKind  `json:"kind"`
	Categories    []string      `json:"categories,omitempty"`
	ReasonCode    string        `json:"reason_code"`
	Confidence    *float64      `json:"confidence,omitempty"`
	EndpointID    string        `json:"endpoint_id,omitempty"`
	Model         string        `json:"model,omitempty"`
	Latency       time.Duration `json:"-"`
	PolicyVersion string        `json:"policy_version,omitempty"`
	HTTPStatus    int           `json:"http_status,omitempty"`
	ErrorClass    string        `json:"error_class,omitempty"`
}

type RequestMeta struct {
	RequestID   string
	UserID      int
	TokenID     int
	Group       string
	Protocol    string
	Model       string
	Mode        string
	RequestHost string
}

func digestSegments(segments []Segment) [32]byte {
	hash := sha256.New()
	for _, segment := range segments {
		_, _ = hash.Write([]byte(segment.Role))
		_, _ = hash.Write([]byte{0})
		_, _ = hash.Write([]byte(segment.Text))
		_, _ = hash.Write([]byte{0xff})
	}
	var digest [32]byte
	copy(digest[:], hash.Sum(nil))
	return digest
}

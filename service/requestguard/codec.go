package requestguard

import (
	"errors"
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/operation_setting"
)

var ErrInvalidResponse = errors.New("invalid RequestGuard response")

type guardMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type codec interface {
	Messages(snapshot Snapshot) []guardMessage
	Decode(content string) (Result, error)
	PolicyVersion() string
}

func codecFor(name string) (codec, error) {
	switch name {
	case operation_setting.RequestGuardCodecQwen3Guard:
		return qwen3GuardCodec{}, nil
	case operation_setting.RequestGuardCodecJSONPolicy:
		return jsonPolicyCodec{}, nil
	default:
		return nil, fmt.Errorf("unsupported codec %q", name)
	}
}

type qwen3GuardCodec struct{}

func (qwen3GuardCodec) Messages(snapshot Snapshot) []guardMessage {
	return []guardMessage{{Role: "user", Content: snapshot.Text()}}
}

func (qwen3GuardCodec) PolicyVersion() string { return "qwen3guard-v1" }

func (qwen3GuardCodec) Decode(content string) (Result, error) {
	lines := nonEmptyLines(content)
	if len(lines) == 0 {
		return Result{}, ErrInvalidResponse
	}
	safety := strings.ToLower(strings.TrimSpace(lines[0]))
	safety = strings.TrimSpace(strings.TrimPrefix(safety, "safety:"))
	var kind DecisionKind
	switch safety {
	case "safe":
		kind = DecisionAllow
	case "unsafe":
		kind = DecisionBlock
	default:
		return Result{}, fmt.Errorf("%w: unknown safety label", ErrInvalidResponse)
	}

	categories := []string{}
	for _, line := range lines[1:] {
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 || !strings.EqualFold(strings.TrimSpace(parts[0]), "categories") {
			return Result{}, fmt.Errorf("%w: unexpected qwen3guard line", ErrInvalidResponse)
		}
		for _, category := range strings.Split(parts[1], ",") {
			category = strings.TrimSpace(category)
			if category != "" && !strings.EqualFold(category, "none") {
				categories = append(categories, category)
			}
		}
	}
	reason := "safe"
	if kind == DecisionBlock {
		reason = "unsafe"
	}
	return Result{Kind: kind, Categories: categories, ReasonCode: reason, PolicyVersion: qwen3GuardCodec{}.PolicyVersion()}, nil
}

type jsonPolicyCodec struct{}

func (jsonPolicyCodec) Messages(snapshot Snapshot) []guardMessage {
	const policy = "Classify the supplied client-controlled content. Return only one JSON object with exactly these fields: decision (allow, flag, or block), categories (array of strings), reason_code (short stable string), confidence (optional number from 0 to 1). Do not return markdown or prose."
	return []guardMessage{
		{Role: "system", Content: policy},
		{Role: "user", Content: snapshot.Text()},
	}
}

func (jsonPolicyCodec) PolicyVersion() string { return "json-policy-v1" }

func (jsonPolicyCodec) Decode(content string) (Result, error) {
	var fields map[string]any
	trimmed := strings.TrimSpace(content)
	if !strings.HasPrefix(trimmed, "{") || !strings.HasSuffix(trimmed, "}") {
		return Result{}, fmt.Errorf("%w: expected JSON object", ErrInvalidResponse)
	}
	if err := common.UnmarshalJsonStr(trimmed, &fields); err != nil {
		return Result{}, fmt.Errorf("%w: %v", ErrInvalidResponse, err)
	}
	for key := range fields {
		switch key {
		case "decision", "categories", "reason_code", "confidence":
		default:
			return Result{}, fmt.Errorf("%w: unexpected field %q", ErrInvalidResponse, key)
		}
	}
	for _, required := range []string{"decision", "categories", "reason_code"} {
		if _, ok := fields[required]; !ok {
			return Result{}, fmt.Errorf("%w: %s is required", ErrInvalidResponse, required)
		}
	}
	var payload struct {
		Decision   string   `json:"decision"`
		Categories []string `json:"categories"`
		ReasonCode string   `json:"reason_code"`
		Confidence *float64 `json:"confidence"`
	}
	if err := common.UnmarshalJsonStr(trimmed, &payload); err != nil {
		return Result{}, fmt.Errorf("%w: %v", ErrInvalidResponse, err)
	}
	var kind DecisionKind
	switch strings.ToLower(strings.TrimSpace(payload.Decision)) {
	case "allow":
		kind = DecisionAllow
	case "flag":
		kind = DecisionFlag
	case "block":
		kind = DecisionBlock
	default:
		return Result{}, fmt.Errorf("%w: unsupported decision", ErrInvalidResponse)
	}
	if payload.Confidence != nil && (*payload.Confidence < 0 || *payload.Confidence > 1) {
		return Result{}, fmt.Errorf("%w: confidence out of range", ErrInvalidResponse)
	}
	reason := strings.TrimSpace(payload.ReasonCode)
	if reason == "" {
		return Result{}, fmt.Errorf("%w: reason_code is required", ErrInvalidResponse)
	}
	for i := range payload.Categories {
		payload.Categories[i] = strings.TrimSpace(payload.Categories[i])
	}
	return Result{
		Kind: kind, Categories: payload.Categories, ReasonCode: reason,
		Confidence: payload.Confidence, PolicyVersion: jsonPolicyCodec{}.PolicyVersion(),
	}, nil
}

func nonEmptyLines(value string) []string {
	lines := strings.Split(strings.TrimSpace(value), "\n")
	result := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			result = append(result, line)
		}
	}
	return result
}

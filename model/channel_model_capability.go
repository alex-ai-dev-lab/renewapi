package model

import (
	"errors"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm/clause"
)

const (
	ChannelCapabilityResponsesCompaction = "responses.compaction"

	ChannelCapabilityStatusUnknown     = 0
	ChannelCapabilityStatusSupported   = 1
	ChannelCapabilityStatusUnsupported = 2
)

// ChannelModelCapability stores observed protocol capabilities. Operator
// configuration remains in Channel.Settings and takes precedence; this table
// is deliberately limited to runtime/probe evidence.
type ChannelModelCapability struct {
	ChannelId  int    `json:"channel_id" gorm:"primaryKey;autoIncrement:false;index"`
	ModelName  string `json:"model_name" gorm:"type:varchar(255);primaryKey;autoIncrement:false;index"`
	Capability string `json:"capability" gorm:"type:varchar(64);primaryKey;autoIncrement:false;index"`

	Status             int    `json:"status" gorm:"default:0;index"`
	CapabilityValue    string `json:"capability_value" gorm:"type:varchar(32)"`
	LegacyStatus       int    `json:"legacy_status" gorm:"default:0"`
	NativeStatus       int    `json:"native_status" gorm:"default:0"`
	ContinuationStatus int    `json:"continuation_status" gorm:"default:0"`
	NativeStreamStatus int    `json:"native_stream_status" gorm:"default:0"`
	NativeStream       bool   `json:"native_stream" gorm:"default:false"`
	Continuation       bool   `json:"continuation" gorm:"default:false"`
	RouteFingerprint   string `json:"route_fingerprint" gorm:"type:varchar(64);index"`
	Source             string `json:"source" gorm:"type:varchar(16)"`

	VerifiedAt     int64  `json:"verified_at" gorm:"bigint;default:0;index"`
	NextProbeAt    int64  `json:"next_probe_at" gorm:"bigint;default:0;index"`
	BlockedUntil   int64  `json:"blocked_until" gorm:"bigint;default:0;index"`
	FailureCount   int    `json:"failure_count" gorm:"default:0"`
	LastStatusCode int    `json:"last_status_code" gorm:"default:0"`
	LastError      string `json:"last_error" gorm:"type:text"`

	CreatedTime int64 `json:"created_time" gorm:"bigint"`
	UpdatedTime int64 `json:"updated_time" gorm:"bigint;index"`
}

var channelModelCapabilityCache atomic.Pointer[sync.Map]

func normalizeChannelModelCapabilityRecord(record ChannelModelCapability) ChannelModelCapability {
	record.ModelName = strings.ToLower(strings.TrimSpace(record.ModelName))
	record.Capability = strings.ToLower(strings.TrimSpace(record.Capability))
	record.CapabilityValue = strings.ToLower(strings.TrimSpace(record.CapabilityValue))
	record.Source = strings.ToLower(strings.TrimSpace(record.Source))
	return record
}

func channelModelCapabilityCacheMap() *sync.Map {
	if cache := channelModelCapabilityCache.Load(); cache != nil {
		return cache
	}
	fresh := &sync.Map{}
	if channelModelCapabilityCache.CompareAndSwap(nil, fresh) {
		return fresh
	}
	return channelModelCapabilityCache.Load()
}

func channelModelCapabilityCacheKey(channelID int, modelName, capability string) string {
	return strconv.Itoa(channelID) + "\x00" +
		strings.ToLower(strings.TrimSpace(modelName)) + "\x00" +
		strings.ToLower(strings.TrimSpace(capability))
}

func ReloadChannelModelCapabilityCacheWithError() error {
	next := &sync.Map{}
	if DB == nil {
		return errors.New("database is not initialized")
	}
	var records []ChannelModelCapability
	if err := DB.Find(&records).Error; err != nil {
		return err
	}
	for _, record := range records {
		record = normalizeChannelModelCapabilityRecord(record)
		if record.ChannelId > 0 && record.ModelName != "" && record.Capability != "" {
			next.Store(channelModelCapabilityCacheKey(record.ChannelId, record.ModelName, record.Capability), record)
		}
	}
	channelModelCapabilityCache.Store(next)
	return nil
}

func ReloadChannelModelCapabilityCache() {
	if err := ReloadChannelModelCapabilityCacheWithError(); err != nil {
		common.SysError("failed to reload channel model capability cache: " + err.Error())
	}
}

func GetChannelModelCapability(channelID int, modelName, capability string) (ChannelModelCapability, bool) {
	modelName = strings.ToLower(strings.TrimSpace(modelName))
	capability = strings.ToLower(strings.TrimSpace(capability))
	if channelID <= 0 || modelName == "" || capability == "" {
		return ChannelModelCapability{}, false
	}
	if common.MemoryCacheEnabled {
		value, ok := channelModelCapabilityCacheMap().Load(channelModelCapabilityCacheKey(channelID, modelName, capability))
		if !ok {
			return ChannelModelCapability{}, false
		}
		record, ok := value.(ChannelModelCapability)
		return record, ok
	}
	if DB == nil {
		return ChannelModelCapability{}, false
	}
	var record ChannelModelCapability
	err := DB.Where("channel_id = ? AND model_name = ? AND capability = ?", channelID, modelName, capability).
		First(&record).Error
	if err != nil {
		return ChannelModelCapability{}, false
	}
	return normalizeChannelModelCapabilityRecord(record), true
}

func UpsertChannelModelCapability(record ChannelModelCapability) error {
	record = normalizeChannelModelCapabilityRecord(record)
	if DB == nil || record.ChannelId <= 0 || record.ModelName == "" || record.Capability == "" {
		return nil
	}
	now := common.GetTimestamp()
	if record.CreatedTime == 0 {
		record.CreatedTime = now
	}
	record.UpdatedTime = now
	err := DB.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "channel_id"}, {Name: "model_name"}, {Name: "capability"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"status", "capability_value", "legacy_status", "native_status",
			"continuation_status", "native_stream_status", "native_stream", "continuation",
			"route_fingerprint", "source", "verified_at", "next_probe_at",
			"blocked_until", "failure_count", "last_status_code", "last_error", "updated_time",
		}),
	}).Create(&record).Error
	if err == nil {
		channelModelCapabilityCacheMap().Store(
			channelModelCapabilityCacheKey(record.ChannelId, record.ModelName, record.Capability),
			record,
		)
	}
	return err
}

func ListChannelModelCapabilities(channelID int, capability string) ([]ChannelModelCapability, error) {
	capability = strings.ToLower(strings.TrimSpace(capability))
	if DB == nil || channelID <= 0 || capability == "" {
		return nil, nil
	}
	var records []ChannelModelCapability
	err := DB.Where("channel_id = ? AND capability = ?", channelID, capability).
		Order("model_name asc").Find(&records).Error
	for i := range records {
		records[i] = normalizeChannelModelCapabilityRecord(records[i])
	}
	return records, err
}

func DeleteChannelModelCapabilities(channelID int) error {
	if DB == nil || channelID <= 0 {
		return nil
	}
	if err := DB.Where("channel_id = ?", channelID).Delete(&ChannelModelCapability{}).Error; err != nil {
		return err
	}
	ReloadChannelModelCapabilityCache()
	return nil
}

package model

import "gorm.io/gorm"

type DatabasePoolStats struct {
	Available          bool  `json:"available"`
	MaxOpenConnections int   `json:"max_open_connections"`
	OpenConnections    int   `json:"open_connections"`
	InUse              int   `json:"in_use"`
	Idle               int   `json:"idle"`
	WaitCount          int64 `json:"wait_count"`
	WaitDurationMs     int64 `json:"wait_duration_ms"`
	MaxIdleClosed      int64 `json:"max_idle_closed"`
	MaxIdleTimeClosed  int64 `json:"max_idle_time_closed"`
	MaxLifetimeClosed  int64 `json:"max_lifetime_closed"`
}

type DatabaseStats struct {
	Main              DatabasePoolStats `json:"main"`
	Log               DatabasePoolStats `json:"log"`
	LogSharedWithMain bool              `json:"log_shared_with_main"`
}

func GetDatabaseStats() DatabaseStats {
	return DatabaseStats{
		Main:              databasePoolStats(DB),
		Log:               databasePoolStats(LOG_DB),
		LogSharedWithMain: DB != nil && LOG_DB == DB,
	}
}

func databasePoolStats(db *gorm.DB) DatabasePoolStats {
	if db == nil {
		return DatabasePoolStats{}
	}
	sqlDB, err := db.DB()
	if err != nil {
		return DatabasePoolStats{}
	}
	stats := sqlDB.Stats()
	return DatabasePoolStats{
		Available:          true,
		MaxOpenConnections: stats.MaxOpenConnections,
		OpenConnections:    stats.OpenConnections,
		InUse:              stats.InUse,
		Idle:               stats.Idle,
		WaitCount:          stats.WaitCount,
		WaitDurationMs:     stats.WaitDuration.Milliseconds(),
		MaxIdleClosed:      stats.MaxIdleClosed,
		MaxIdleTimeClosed:  stats.MaxIdleTimeClosed,
		MaxLifetimeClosed:  stats.MaxLifetimeClosed,
	}
}

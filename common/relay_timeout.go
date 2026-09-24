package common

import (
	"strconv"
	"strings"
)

// GetRelayFirstByteTimeout reads the saved override under the option map lock.
// The environment-derived value remains the fallback until an override is saved.
func GetRelayFirstByteTimeout() int {
	OptionMapRWMutex.RLock()
	value := OptionMap["RelayFirstByteTimeout"]
	OptionMapRWMutex.RUnlock()
	if seconds, err := strconv.Atoi(strings.TrimSpace(value)); err == nil && seconds >= 1 && seconds <= 86400 {
		return seconds
	}
	if RelayFirstByteTimeout > 0 {
		return RelayFirstByteTimeout
	}
	return 15
}

package common

import "testing"

func TestRelayTimeoutSavedOverride(t *testing.T) {
	oldMap, oldTimeout := OptionMap, RelayFirstByteTimeout
	t.Cleanup(func() { OptionMap, RelayFirstByteTimeout = oldMap, oldTimeout })
	OptionMap = map[string]string{}
	RelayFirstByteTimeout = 45
	if GetRelayFirstByteTimeout() != 45 {
		t.Fatal("environment fallback lost")
	}
	OptionMap["RelayFirstByteTimeout"] = " 120 "
	if GetRelayFirstByteTimeout() != 120 {
		t.Fatal("saved override ignored")
	}
	OptionMap["RelayFirstByteTimeout"] = "invalid"
	if GetRelayFirstByteTimeout() != 45 {
		t.Fatal("invalid override must fall back")
	}
}

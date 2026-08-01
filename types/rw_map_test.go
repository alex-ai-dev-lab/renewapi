package types

import "testing"

func TestLoadFromJsonStringKeepsExistingDataOnInvalidJSON(t *testing.T) {
	m := NewRWMap[string, int]()
	if err := LoadFromJsonString(m, `{"old":1}`); err != nil {
		t.Fatalf("initial load failed: %v", err)
	}
	if err := LoadFromJsonString(m, `{"broken":}`); err == nil {
		t.Fatal("expected invalid JSON to be rejected")
	}
	if value, ok := m.Get("old"); !ok || value != 1 {
		t.Fatalf("existing data was lost after invalid load: %v %v", value, ok)
	}
}

func TestLoadFromJsonStringWithCallbackRunsAfterUnlock(t *testing.T) {
	m := NewRWMap[string, int]()
	callbackCalled := false
	if err := LoadFromJsonStringWithCallback(m, `{"new":2}`, func() {
		callbackCalled = true
		if value, ok := m.Get("new"); !ok || value != 2 {
			t.Fatalf("callback cannot observe committed map: %v %v", value, ok)
		}
	}); err != nil {
		t.Fatalf("load failed: %v", err)
	}
	if !callbackCalled {
		t.Fatal("success callback was not called")
	}
}

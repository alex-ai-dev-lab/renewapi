package setting

import (
	"reflect"
	"testing"
)

func TestUpdateModelRequestRateLimitGroupInvalidJSONKeepsExisting(t *testing.T) {
	ModelRequestRateLimitMutex.Lock()
	original := ModelRequestRateLimitGroup
	ModelRequestRateLimitGroup = map[string][2]int{
		"default": [2]int{10, 5},
	}
	ModelRequestRateLimitMutex.Unlock()
	defer func() {
		ModelRequestRateLimitMutex.Lock()
		ModelRequestRateLimitGroup = original
		ModelRequestRateLimitMutex.Unlock()
	}()

	if err := UpdateModelRequestRateLimitGroupByJSONString(`{"broken":[1,]}`); err == nil {
		t.Fatal("expected invalid JSON to be rejected")
	}

	ModelRequestRateLimitMutex.RLock()
	got := ModelRequestRateLimitGroup
	ModelRequestRateLimitMutex.RUnlock()
	if !reflect.DeepEqual(got, map[string][2]int{"default": [2]int{10, 5}}) {
		t.Fatalf("rate-limit group changed after invalid update: %#v", got)
	}
}

func TestValidateModelRequestRateLimitSettings(t *testing.T) {
	if err := ValidateModelRequestRateLimitSettings(0, 10, 5); err == nil {
		t.Fatal("expected non-positive duration to be rejected")
	}
	if err := ValidateModelRequestRateLimitSettings(1, -1, 5); err == nil {
		t.Fatal("expected negative total limit to be rejected")
	}
	if err := ValidateModelRequestRateLimitSettings(1, 10, -1); err == nil {
		t.Fatal("expected negative successful limit to be rejected")
	}
	if err := ValidateModelRequestRateLimitSettings(1, 10, 5); err != nil {
		t.Fatalf("valid scalar limits rejected: %v", err)
	}
}

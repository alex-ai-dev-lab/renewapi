package ratio_setting

import "testing"

func TestExplicitCompletionRatioOverridesHardcodedDefault(t *testing.T) {
	original := completionRatioMap.ReadAll()
	completionRatioMap.Clear()
	t.Cleanup(func() {
		completionRatioMap.Clear()
		completionRatioMap.AddAll(original)
	})

	tests := []struct {
		name  string
		ratio float64
	}{
		{name: "gpt-5.6-sol", ratio: 6},
		{name: "mistral-medium-latest", ratio: 5},
	}

	for _, test := range tests {
		completionRatioMap.Set(test.name, test.ratio)

		if got := GetCompletionRatio(test.name); got != test.ratio {
			t.Fatalf("GetCompletionRatio(%q) = %v, want %v", test.name, got, test.ratio)
		}

		info := GetCompletionRatioInfo(test.name)
		if info.Ratio != test.ratio || info.Locked {
			t.Fatalf("GetCompletionRatioInfo(%q) = %+v, want ratio %v and unlocked", test.name, info, test.ratio)
		}
	}
}

func TestCompletionRatioFallsBackToHardcodedDefault(t *testing.T) {
	original := completionRatioMap.ReadAll()
	completionRatioMap.Clear()
	t.Cleanup(func() {
		completionRatioMap.Clear()
		completionRatioMap.AddAll(original)
	})

	const model = "gpt-5.6-sol"
	if got := GetCompletionRatio(model); got != 8 {
		t.Fatalf("GetCompletionRatio(%q) = %v, want hardcoded fallback 8", model, got)
	}

	info := GetCompletionRatioInfo(model)
	if info.Ratio != 8 || !info.Locked {
		t.Fatalf("GetCompletionRatioInfo(%q) = %+v, want ratio 8 and locked", model, info)
	}
}

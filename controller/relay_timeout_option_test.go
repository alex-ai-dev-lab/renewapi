package controller

import "testing"

func TestRelayTimeoutOptionValidation(t *testing.T) {
	for _, value := range []string{"1", "60", "86400"} {
		if err := validateOptionValues(map[string]string{"RelayFirstByteTimeout": value}); err != nil {
			t.Fatalf("valid timeout %q: %v", value, err)
		}
	}
	for _, value := range []string{"0", "-1", "1.5", "86401", "NaN", ""} {
		if err := validateOptionValues(map[string]string{"RelayFirstByteTimeout": value}); err == nil {
			t.Fatalf("accepted invalid timeout %q", value)
		}
	}
}

func TestRelayTimeoutWhitespaceIsCanonicalized(t *testing.T) {
	values, err := normalizeOptionValues(map[string]string{"RelayFirstByteTimeout": " 60 "})
	if err != nil {
		t.Fatal(err)
	}
	if err := validateOptionValues(values); err != nil {
		t.Fatal(err)
	}
	if values["RelayFirstByteTimeout"] != "60" {
		t.Fatalf("noncanonical timeout: %q", values["RelayFirstByteTimeout"])
	}
}

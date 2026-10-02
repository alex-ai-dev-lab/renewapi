package common

import (
	"os"
	"reflect"
	"testing"
)

func TestModelMappingSharedContract(t *testing.T) {
	data, err := os.ReadFile("testdata/model-mapping-contract.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixtures []struct {
		Name       string              `json:"name"`
		Raw        string              `json:"raw"`
		Valid      bool                `json:"valid"`
		Version    int                 `json:"version"`
		Sources    []string            `json:"sources"`
		Candidates map[string][]string `json:"candidates"`
	}
	if err := Unmarshal(data, &fixtures); err != nil {
		t.Fatal(err)
	}
	for _, fixture := range fixtures {
		t.Run(fixture.Name, func(t *testing.T) {
			err := ValidateModelMapping(fixture.Raw)
			if (err == nil) != fixture.Valid {
				t.Fatalf("valid=%v, got %v", fixture.Valid, err)
			}
			if !fixture.Valid {
				return
			}
			config, err := ParseModelMappingConfig(fixture.Raw)
			if err != nil {
				t.Fatal(err)
			}
			if config.Version != fixture.Version || !reflect.DeepEqual(config.Sources(), fixture.Sources) {
				t.Fatalf("unexpected config: %+v", config)
			}
			for source, expected := range fixture.Candidates {
				actual, err := config.Candidates(source)
				if err != nil || !reflect.DeepEqual(candidateModels(actual), expected) {
					t.Fatalf("%q: expected %v, got %v (%v)", source, expected, actual, err)
				}
			}
		})
	}
}

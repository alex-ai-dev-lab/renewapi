package common

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
)

func candidateModels(candidates []ModelMappingCandidate) []string {
	models := make([]string, len(candidates))
	for index, candidate := range candidates {
		models[index] = candidate.Model
	}
	return models
}

func TestModelMappingRulesPreserveLegacyChains(t *testing.T) {
	config, err := ParseModelMappingConfig(`{"alias":["middle","direct"],"middle":"terminal","self":"self"}`)
	if err != nil {
		t.Fatal(err)
	}
	if config.IsRules() {
		t.Fatal("legacy config was converted into direct rules")
	}
	candidates, err := config.Candidates("alias")
	if err != nil {
		t.Fatal(err)
	}
	if got := candidateModels(candidates); !reflect.DeepEqual(got, []string{"terminal", "direct"}) {
		t.Fatalf("legacy candidates = %v", got)
	}
	self, err := config.Candidates("self")
	if err != nil || !reflect.DeepEqual(candidateModels(self), []string{"self"}) {
		t.Fatalf("self mapping: %v %v", self, err)
	}
	if err := ValidateModelMapping(`{"A":"B","B":"A"}`); err == nil {
		t.Fatal("legacy cycle was accepted")
	}
}

func TestModelMappingRulesDirectPriorityAndSharedTargets(t *testing.T) {
	raw := `{"version":2,"rules":[
		{"id":"low","from":"one","to":"B","priority":80},
		{"id":"first","from":"one","to":"A","priority":100},
		{"id":"shared","from":"two","to":"A","priority":100},
		{"id":"tie","from":"one","to":"C","priority":100},
		{"id":"duplicate","from":"one","to":"A","priority":20},
		{"id":"off","from":"disabled","to":"D","enabled":false}
	]}`
	config, err := ParseModelMappingConfig(raw)
	if err != nil {
		t.Fatal(err)
	}
	if !config.IsRules() {
		t.Fatal("v2 rules not recognized")
	}
	if got := config.Sources(); !reflect.DeepEqual(got, []string{"one", "two"}) {
		t.Fatalf("sources = %v", got)
	}
	candidates, err := config.Candidates("one")
	if err != nil {
		t.Fatal(err)
	}
	if got := candidateModels(candidates); !reflect.DeepEqual(got, []string{"A", "C", "B"}) {
		t.Fatalf("ordered candidates = %v", got)
	}
	if candidates[0].RuleID != "first" || candidates[1].RuleID != "tie" {
		t.Fatalf("rule identity/order was lost: %+v", candidates)
	}
	shared, err := config.Candidates("two")
	if err != nil || !reflect.DeepEqual(candidateModels(shared), []string{"A"}) {
		t.Fatalf("shared target: %v %v", shared, err)
	}
	missing, err := config.Candidates("disabled")
	if err != nil || len(missing) != 0 {
		t.Fatalf("disabled source became active: %v %v", missing, err)
	}
}

func TestModelMappingRulesAllowCrossAndDefaultFields(t *testing.T) {
	config, err := ParseModelMappingConfig(`{"version":2,"rules":[{"id":"a","from":"A","to":"B"},{"id":"b","from":"B","to":"A"}]}`)
	if err != nil {
		t.Fatal(err)
	}
	if !config.Rules[0].Enabled || config.Rules[0].Priority != 0 {
		t.Fatal("missing fields have incorrect defaults")
	}
	for source, target := range map[string]string{"A": "B", "B": "A"} {
		got, err := config.Candidates(source)
		if err != nil || !reflect.DeepEqual(candidateModels(got), []string{target}) {
			t.Fatalf("direct %s: %+v %v", source, got, err)
		}
	}
	if err := ValidateModelMapping(`{"version":2,"rules":[]}`); err != nil {
		t.Fatal(err)
	}
}

func TestModelMappingRulesAcceptIntegralJSONNumbers(t *testing.T) {
	config, err := ParseModelMappingConfig(`{"version":2.0,"rules":[{"id":"r","from":"a","to":"b","priority":1e2}]}`)
	if err != nil || config.Rules[0].Priority != 100 {
		t.Fatalf("integral JSON numbers must match frontend number semantics: %v %v", config, err)
	}
}

func TestModelMappingRulesEnvelopeDoesNotReserveModelNames(t *testing.T) {
	config, err := ParseModelMappingConfig(`{"version":"v-model","rules":["model-a","model-b"]}`)
	if err != nil {
		t.Fatal(err)
	}
	if config.IsRules() {
		t.Fatal("legacy model names mistaken for envelope")
	}
	got, err := config.Candidates("rules")
	if err != nil || !reflect.DeepEqual(candidateModels(got), []string{"model-a", "model-b"}) {
		t.Fatalf("%v %v", got, err)
	}
}

func TestModelMappingRulesRejectInvalidWrites(t *testing.T) {
	invalid := []string{
		`null`, `[]`, `{"a":[]}`, `{"a":""}`, `{"a":null}`, `{"a":["x",3]}`,
		`{" a ":"x","a":"y"}`, `{"version":3,"rules":[]}`, `{"version":2.5,"rules":[]}`,
		`{"version":2}`, `{"version":2,"rules":null}`, `{"version":2,"rules":{},"extra":true}`,
		`{"version":2,"rules":[],"extra":true}`,
		`{"version":2,"rules":[{"from":"a","to":"b"}]}`,
		`{"version":2,"rules":[{"id":"x","from":" ","to":"b"}]}`,
		`{"version":2,"rules":[{"id":"x","from":"a","to":"","enabled":false}]}`,
		`{"version":2,"rules":[{"id":"x","from":"a","to":"b","priority":1.2}]}`,
		`{"version":2,"rules":[{"id":"x","from":"a","to":"b","priority":2147483648}]}`,
		`{"version":2,"rules":[{"id":"x","from":"a","to":"b","priority":null}]}`,
		`{"version":2,"rules":[{"id":"x","from":"a","to":"b","enabled":null}]}`,
		`{"version":2,"rules":[{"id":"x","from":"a","to":"b","enabled":"true"}]}`,
		`{"version":2,"rules":[{"id":"x","from":"a","to":"b","priorty":20}]}`,
		`{"version":2,"rules":[{"id":"same","from":"a","to":"b"},{"id":"same","from":"a","to":"c"}]}`,
	}
	for _, raw := range invalid {
		t.Run(raw, func(t *testing.T) {
			if err := ValidateModelMapping(raw); err == nil {
				t.Fatalf("accepted invalid mapping: %s", raw)
			}
		})
	}
	for _, raw := range []string{"", "{}", `{"a":"b"}`, `{"a":["b","c"]}`, `{"version":2,"rules":[]}`} {
		if err := ValidateModelMapping(raw); err != nil {
			t.Fatalf("valid mapping rejected: %s: %v", raw, err)
		}
	}
}

func TestModelMappingRulesLimitsAreBounded(t *testing.T) {
	if err := ValidateModelMapping(strings.Repeat(" ", MaxModelMappingBytes+1)); err == nil {
		t.Fatal("unbounded input accepted")
	}
	longName := strings.Repeat("名", 256)
	if err := ValidateModelMapping(fmt.Sprintf(`{"%s":"target"}`, longName)); err == nil {
		t.Fatal("oversized source accepted")
	}
	chain := make(map[string]string)
	for index := 0; index < 34; index++ {
		chain[fmt.Sprintf("m%d", index)] = fmt.Sprintf("m%d", index+1)
	}
	raw, err := Marshal(chain)
	if err != nil {
		t.Fatal(err)
	}
	if err := ValidateModelMapping(string(raw)); err == nil {
		t.Fatal("deep chain accepted")
	}
	wide := make([]string, 129)
	for index := range wide {
		wide[index] = fmt.Sprintf("target-%d", index)
	}
	raw, err = Marshal(map[string][]string{"a": wide})
	if err != nil {
		t.Fatal(err)
	}
	if err := ValidateModelMapping(string(raw)); err == nil {
		t.Fatal("unbounded candidates accepted")
	}
	branch := make(map[string][]string)
	for index := 0; index < 15; index++ {
		for _, prefix := range []string{"a", "b"} {
			branch[fmt.Sprintf("%s%d", prefix, index)] = []string{fmt.Sprintf("a%d", index+1), fmt.Sprintf("b%d", index+1)}
		}
	}
	raw, err = Marshal(branch)
	if err != nil {
		t.Fatal(err)
	}
	if err := ValidateModelMapping(string(raw)); err == nil {
		t.Fatal("excessive expansion work accepted")
	}
}

func TestModelMappingRulesRejectDuplicateJSONMembers(t *testing.T) {
	for _, raw := range []string{
		`{"a":"first","a":"second"}`,
		`{"version":2,"version":2,"rules":[]}`,
		`{"version":2,"rules":[{"id":"r","from":"a","from":"b","to":"c"}]}`,
	} {
		if err := ValidateModelMapping(raw); err == nil {
			t.Fatalf("duplicate JSON members accepted: %s", raw)
		}
	}
	legacy, err := ParseModelMapping(`{"a":"first","a":"second"}`)
	if err != nil || !reflect.DeepEqual(legacy["a"], []string{"second"}) {
		t.Fatalf("persisted legacy read compatibility changed: %v %v", legacy, err)
	}
}

func TestModelMappingRulesChangeProtectsLegacyAndV2(t *testing.T) {
	legacyNull, err := ParseModelMappingConfig("null")
	if err != nil || len(legacyNull.Sources()) != 0 {
		t.Fatalf("legacy null read compatibility: %v %v", legacyNull, err)
	}
	if err := ValidateModelMappingChange("null", "null"); err != nil {
		t.Fatal("unchanged legacy blocked unrelated update")
	}
	if err := ValidateModelMappingChange(`{"a":"b"}`, `{"a":"c"}`); err != nil {
		t.Fatal(err)
	}
	v2 := `{"version":2,"rules":[{"id":"r","from":"a","to":"b"}]}`
	if err := ValidateModelMappingChange(v2, `{"a":"b"}`); err == nil {
		t.Fatal("silent downgrade accepted")
	}
	if err := ValidateModelMappingChange(v2, ""); err == nil {
		t.Fatal("old client cleared v2 without explicit v2 clear")
	}
	if err := ValidateModelMappingChange(v2, `{"version":2,"rules":[]}`); err != nil {
		t.Fatal(err)
	}
}

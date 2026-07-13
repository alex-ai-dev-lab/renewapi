package adminconfig

type Source string

const (
	SourceGlobal  Source = "global"
	SourceGroup   Source = "group"
	SourceChannel Source = "channel"
	SourceRequest Source = "request"
)

type Layer struct {
	Source   Source `json:"source"`
	SourceID string `json:"source_id,omitempty"`
	Present  bool   `json:"present"`
	Value    any    `json:"value,omitempty"`
}

type EffectiveValue struct {
	Key      string  `json:"key"`
	Value    any     `json:"value,omitempty"`
	Source   Source  `json:"source"`
	SourceID string  `json:"source_id,omitempty"`
	Chain    []Layer `json:"chain"`
	Masked   bool    `json:"masked,omitempty"`
}

// Resolve applies the runtime precedence order once and keeps the full source
// chain for management-only inspection. Later layers override earlier ones.
func Resolve(key string, masked bool, layers ...Layer) EffectiveValue {
	result := EffectiveValue{
		Key:    key,
		Chain:  append([]Layer(nil), layers...),
		Masked: masked,
	}
	for _, layer := range layers {
		if !layer.Present {
			continue
		}
		result.Source = layer.Source
		result.SourceID = layer.SourceID
		if !masked {
			result.Value = layer.Value
		}
	}
	if masked {
		for i := range result.Chain {
			result.Chain[i].Value = nil
		}
	}
	return result
}

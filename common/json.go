package common

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
)

type RawMessage = json.RawMessage

func Unmarshal(data []byte, v any) error {
	return json.Unmarshal(data, v)
}

func UnmarshalJsonStr(data string, v any) error {
	return json.Unmarshal(StringToByteSlice(data), v)
}

func DecodeJson(reader io.Reader, v any) error {
	return json.NewDecoder(reader).Decode(v)
}

// WalkJsonArray decodes one raw element at a time and stops immediately when
// visit returns false. Callers can therefore enforce their own bounded work
// without materializing an entire JSON array.
func WalkJsonArray(data []byte, visit func(RawMessage) bool) error {
	if visit == nil {
		return errors.New("JSON array visitor is nil")
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	token, err := decoder.Token()
	if err != nil {
		return err
	}
	delimiter, ok := token.(json.Delim)
	if !ok || delimiter != '[' {
		return errors.New("JSON value is not an array")
	}
	for decoder.More() {
		var item RawMessage
		if err := decoder.Decode(&item); err != nil {
			return err
		}
		if !visit(item) {
			return nil
		}
	}
	_, err = decoder.Token()
	return err
}

func Marshal(v any) ([]byte, error) {
	return json.Marshal(v)
}

func GetJsonType(data json.RawMessage) string {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 {
		return "unknown"
	}
	firstChar := trimmed[0]
	switch firstChar {
	case '{':
		return "object"
	case '[':
		return "array"
	case '"':
		return "string"
	case 't', 'f':
		return "boolean"
	case 'n':
		return "null"
	default:
		return "number"
	}
}

// JsonRawMessageToString returns JSON strings as their decoded value and other JSON values as raw text.
func JsonRawMessageToString(data json.RawMessage) string {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
		return ""
	}
	if trimmed[0] != '"' {
		return string(trimmed)
	}
	var value string
	if err := Unmarshal(trimmed, &value); err != nil {
		return string(trimmed)
	}
	return value
}

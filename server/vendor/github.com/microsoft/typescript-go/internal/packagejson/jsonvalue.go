package packagejson

import (
	"fmt"

	"github.com/go-json-experiment/json"
	"github.com/go-json-experiment/json/jsontext"
	"github.com/microsoft/typescript-go/internal/collections"
)

type JSONValueType int8

const (
	JSONValueTypeNotPresent JSONValueType = iota
	JSONValueTypeNull
	JSONValueTypeString
	JSONValueTypeNumber
	JSONValueTypeBoolean
	JSONValueTypeArray
	JSONValueTypeObject
)

func (t JSONValueType) String() string {
	switch t {
	case JSONValueTypeNull:
		return "null"
	case JSONValueTypeString:
		return "string"
	case JSONValueTypeNumber:
		return "number"
	case JSONValueTypeBoolean:
		return "boolean"
	case JSONValueTypeArray:
		return "array"
	case JSONValueTypeObject:
		return "object"
	default:
		return fmt.Sprintf("unknown(%d)", t)
	}
}

type JSONValue struct {
	Type  JSONValueType
	Value any
}

func (v *JSONValue) IsFalsy() bool {
	switch v.Type {
	case JSONValueTypeNotPresent, JSONValueTypeNull:
		return true
	case JSONValueTypeString:
		return v.Value == ""
	case JSONValueTypeNumber:
		return v.Value == 0
	case JSONValueTypeBoolean:
		return !v.Value.(bool)
	default:
		return false
	}
}

func (v JSONValue) AsObject() *collections.OrderedMap[string, JSONValue] {
	if v.Type != JSONValueTypeObject {
		panic(fmt.Sprintf("expected object, got %v", v.Type))
	}
	return v.Value.(*collections.OrderedMap[string, JSONValue])
}

func (v JSONValue) AsArray() []JSONValue {
	if v.Type != JSONValueTypeArray {
		panic(fmt.Sprintf("expected array, got %v", v.Type))
	}
	return v.Value.([]JSONValue)
}

var _ json.UnmarshalerFrom = (*JSONValue)(nil)

func (v *JSONValue) UnmarshalJSONFrom(dec *jsontext.Decoder) error {
	return unmarshalJSONValueV2[JSONValue](v, dec)
}

func unmarshalJSONValue[T any](v *JSONValue, data []byte) error {
	if string(data) == "null" {
		*v = JSONValue{Type: JSONValueTypeNull}
	} else if data[0] == '"' {
		v.Type = JSONValueTypeString
		return json.Unmarshal(data, &v.Value)
	} else if data[0] == '[' {
		var elements []T
		if err := json.Unmarshal(data, &elements); err != nil {
			return err
		}
		v.Type = JSONValueTypeArray
		v.Value = elements
	} else if data[0] == '{' {
		var object collections.OrderedMap[string, T]
		if err := json.Unmarshal(data, &object); err != nil {
			return err
		}
		v.Type = JSONValueTypeObject
		v.Value = &object
	} else if string(data) == "true" {
		v.Type = JSONValueTypeBoolean
		v.Value = true
	} else if string(data) == "false" {
		v.Type = JSONValueTypeBoolean
		v.Value = false
	} else {
		v.Type = JSONValueTypeNumber
		return json.Unmarshal(data, &v.Value)
	}
	return nil
}

func unmarshalJSONValueV2[T any](v *JSONValue, dec *jsontext.Decoder) error {
	switch dec.PeekKind() {
	case 'n': // jsontext.Null.Kind()
		if _, err := dec.ReadToken(); err != nil {
			return err
		}
		v.Value = nil
		v.Type = JSONValueTypeNull
		return nil
	case '"':
		v.Type = JSONValueTypeString
		if err := json.UnmarshalDecode(dec, &v.Value); err != nil {
			return err
		}
	case '[':
		if _, err := dec.ReadToken(); err != nil {
			return err
		}
		var elements []T
		for dec.PeekKind() != jsontext.EndArray.Kind() {
			var element T
			if err := json.UnmarshalDecode(dec, &element); err != nil {
				return err
			}
			elements = append(elements, element)
		}
		if _, err := dec.ReadToken(); err != nil {
			return err
		}
		v.Type = JSONValueTypeArray
		v.Value = elements
	case '{':
		var object collections.OrderedMap[string, T]
		if err := json.UnmarshalDecode(dec, &object); err != nil {
			return err
		}
		v.Type = JSONValueTypeObject
		v.Value = &object
	case 't', 'f': // jsontext.True.Kind(), jsontext.False.Kind()
		v.Type = JSONValueTypeBoolean
		if err := json.UnmarshalDecode(dec, &v.Value); err != nil {
			return err
		}
	default:
		v.Type = JSONValueTypeNumber
		if err := json.UnmarshalDecode(dec, &v.Value); err != nil {
			return err
		}
	}
	return nil
}

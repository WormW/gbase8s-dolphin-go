package mysql

import (
	"bytes"
	"encoding/hex"
	"testing"
)

func TestDecodeGBaseBytea(t *testing.T) {
	payload := []byte(`{"name":"example","binary":"` + string([]byte{0x00, 0x27, 0x5c, 0xff}) + `"}`)
	encoded := []byte(`\x` + hex.EncodeToString(payload))
	field := mysqlField{fieldType: fieldTypeMediumBLOB, charSet: 33}
	if actual := decodeGBaseBytea(field, encoded); !bytes.Equal(actual, payload) {
		t.Fatalf("decodeGBaseBytea() = %x, want %x", actual, payload)
	}
}

func TestDecodeGBaseByteaLeavesOtherValuesUnchanged(t *testing.T) {
	tests := []struct {
		name  string
		field mysqlField
		value []byte
	}{
		{name: "not blob", field: mysqlField{fieldType: fieldTypeVarString, charSet: binaryCollationID}, value: []byte(`\x616263`)},
		{name: "invalid hex", field: mysqlField{fieldType: fieldTypeBLOB, charSet: binaryCollationID}, value: []byte(`\xzz`)},
		{name: "odd length", field: mysqlField{fieldType: fieldTypeBLOB, charSet: binaryCollationID}, value: []byte(`\x1`)},
		{name: "ordinary text", field: mysqlField{fieldType: fieldTypeBLOB, charSet: binaryCollationID}, value: []byte(`plain`)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if actual := decodeGBaseBytea(tt.field, tt.value); !bytes.Equal(actual, tt.value) {
				t.Fatalf("decodeGBaseBytea() = %x, want unchanged %x", actual, tt.value)
			}
		})
	}
}

func TestDecodeGBaseByteaEmptyValue(t *testing.T) {
	field := mysqlField{fieldType: fieldTypeBLOB, charSet: binaryCollationID}
	if actual := decodeGBaseBytea(field, []byte(`\x`)); len(actual) != 0 {
		t.Fatalf("decodeGBaseBytea() returned %x, want empty", actual)
	}
}

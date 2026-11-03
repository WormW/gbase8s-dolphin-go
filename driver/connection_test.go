package mysql

import (
	"database/sql/driver"
	"encoding/hex"
	"testing"
)

func TestInterpolateParamsByteSliceUsesByteaDecode(t *testing.T) {
	tests := []struct {
		name  string
		value []byte
	}{
		{name: "json", value: []byte(`{"name":"example","value":"'\\"}`)},
		{name: "binary", value: []byte{0x00, 0x01, 0x27, 0x5c, 0xff}},
		{name: "empty", value: []byte{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mc := &mysqlConn{buf: newBuffer(), maxAllowedPacket: maxPacketSize}
			query, err := mc.interpolateParams("SELECT ?", []driver.Value{tt.value})
			if err != nil {
				t.Fatalf("interpolateParams() error = %v", err)
			}
			expected := "SELECT decode('" + hex.EncodeToString(tt.value) + "','hex')"
			if query != expected {
				t.Fatalf("interpolateParams() = %q, want %q", query, expected)
			}
		})
	}
}

func TestInterpolateParamsNilByteSliceRemainsNull(t *testing.T) {
	mc := &mysqlConn{buf: newBuffer(), maxAllowedPacket: maxPacketSize}
	query, err := mc.interpolateParams("SELECT ?", []driver.Value{[]byte(nil)})
	if err != nil {
		t.Fatalf("interpolateParams() error = %v", err)
	}
	if query != "SELECT NULL" {
		t.Fatalf("interpolateParams() = %q, want SELECT NULL", query)
	}
}

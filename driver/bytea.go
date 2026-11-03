package mysql

import "encoding/hex"

func decodeGBaseBytea(field mysqlField, buf []byte) []byte {
	if len(buf) < 2 || buf[0] != '\\' || buf[1] != 'x' || len(buf)%2 != 0 {
		return buf
	}
	switch field.fieldType {
	case fieldTypeTinyBLOB, fieldTypeMediumBLOB, fieldTypeLongBLOB, fieldTypeBLOB:
	default:
		return buf
	}

	decoded := make([]byte, hex.DecodedLen(len(buf)-2))
	if _, err := hex.Decode(decoded, buf[2:]); err != nil {
		return buf
	}
	return decoded
}

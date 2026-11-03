package mysql

import (
	"database/sql/driver"
	"fmt"
	"strconv"
	"strings"
)

func shouldQueryLastInsertID(query string, result mysqlResult) bool {
	if len(result.insertIds) != 1 || result.insertIds[0] != 0 {
		return false
	}
	fields := strings.Fields(query)
	return len(fields) > 0 && strings.EqualFold(fields[0], "INSERT")
}

func (mc *mysqlConn) queryLastInsertID() (int64, error) {
	rows, err := mc.query("SELECT LAST_INSERT_ID()", nil)
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	dest := make([]driver.Value, 1)
	if err := rows.Next(dest); err != nil {
		return 0, err
	}
	return parseLastInsertID(dest[0])
}

func parseLastInsertID(value driver.Value) (int64, error) {
	switch v := value.(type) {
	case int64:
		return v, nil
	case []byte:
		return strconv.ParseInt(string(v), 10, 64)
	case string:
		return strconv.ParseInt(v, 10, 64)
	default:
		return 0, fmt.Errorf("LAST_INSERT_ID() returned %T", value)
	}
}

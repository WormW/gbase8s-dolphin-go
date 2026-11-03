package mysql

import "testing"

func TestShouldQueryLastInsertID(t *testing.T) {
	tests := []struct {
		name   string
		query  string
		result mysqlResult
		want   bool
	}{
		{name: "single insert with zero id", query: "INSERT INTO example_table VALUES (1)", result: mysqlResult{insertIds: []int64{0}}, want: true},
		{name: "insert with lowercase and whitespace", query: "  insert\ninto example_table values (1)", result: mysqlResult{insertIds: []int64{0}}, want: true},
		{name: "insert with assigned id", query: "INSERT INTO example_table VALUES (1)", result: mysqlResult{insertIds: []int64{1}}, want: false},
		{name: "update", query: "UPDATE example_table SET id = 1", result: mysqlResult{insertIds: []int64{0}}, want: false},
		{name: "multiple statements", query: "INSERT INTO a VALUES (1); INSERT INTO b VALUES (2)", result: mysqlResult{insertIds: []int64{0, 0}}, want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := shouldQueryLastInsertID(tt.query, tt.result); got != tt.want {
				t.Fatalf("shouldQueryLastInsertID() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParseLastInsertID(t *testing.T) {
	for _, value := range []interface{}{int64(1), []byte("1"), "1"} {
		insertID, err := parseLastInsertID(value)
		if err != nil {
			t.Fatalf("parseLastInsertID(%T) error = %v", value, err)
		}
		if insertID != 1 {
			t.Fatalf("parseLastInsertID(%T) = %d, want 1", value, insertID)
		}
	}
}

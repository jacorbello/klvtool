package record

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestValueJSONMarshaling(t *testing.T) {
	tests := []struct {
		name string
		v    Value
		want string
	}{
		{"int", IntValue(-42), `-42`},
		{"uint", UintValue(99), `99`},
		{"float", FloatValue(3.5), `3.5`},
		{"string", StringValue("REAPER"), `"REAPER"`},
		{"bool_true", BoolValue(true), `true`},
		{"bool_false", BoolValue(false), `false`},
		{"bytes_base64", BytesValue{0x01, 0x02, 0xff}, `"AQL/"`},
		{"enum", EnumValue{Code: 1, Label: "No Icing Detected"}, `{"code":1,"label":"No Icing Detected"}`},
		{"nested", NestedValue{SpecHint: "MISB ST 0102", Raw: []byte{0xde, 0xad}}, `{"specHint":"MISB ST 0102","raw":"3q0="}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b, err := json.Marshal(tt.v)
			if err != nil {
				t.Fatalf("Marshal error: %v", err)
			}
			if string(b) != tt.want {
				t.Errorf("Marshal = %s, want %s", string(b), tt.want)
			}
		})
	}
}

func TestTimeValueJSONMarshaling(t *testing.T) {
	tv := TimeValue(time.Date(2023, 3, 2, 12, 34, 56, 789000000, time.UTC))
	b, err := json.Marshal(tv)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}
	want := `"2023-03-02T12:34:56.789Z"`
	if string(b) != want {
		t.Errorf("Marshal = %s, want %s", string(b), want)
	}
}

func TestRecordJSONShape(t *testing.T) {
	rec := Record{
		Schema:      "urn:misb:KLV:bin:0601.19",
		LSVersion:   19,
		TotalLength: 12,
		Checksum:    ChecksumInfo{Expected: 0x1111, Computed: 0x1111, Valid: true},
		Items: []Item{
			{Tag: 5, Name: "Platform Heading Angle", Value: FloatValue(159.97)},
		},
	}
	b, err := json.Marshal(rec)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}
	got := string(b)
	if !strings.Contains(got, `"schema":"urn:misb:KLV:bin:0601.19"`) {
		t.Errorf("missing schema: %s", got)
	}
	if !strings.Contains(got, `"lsVersion":19`) {
		t.Errorf("missing lsVersion: %s", got)
	}
	if !strings.Contains(got, `"checksum":{"expected":4369,"computed":4369,"valid":true}`) {
		t.Errorf("missing checksum: %s", got)
	}
	if !strings.Contains(got, `"value":159.97`) {
		t.Errorf("missing item value: %s", got)
	}
}

package bootstrap

import (
	"encoding/json"
	"reflect"
	"testing"
)

func decode(t *testing.T, s string) any {
	t.Helper()
	var v any
	if err := json.Unmarshal([]byte(s), &v); err != nil {
		t.Fatal(err)
	}
	return v
}

func TestMismatches(t *testing.T) {
	tests := []struct {
		name    string
		desired string
		actual  string
		want    []string
	}{
		{name: "identical", desired: `{"a":1}`, actual: `{"a":1}`, want: nil},
		{name: "extra keys in actual are ignored", desired: `{"a":1}`, actual: `{"a":1,"node_id":"x"}`, want: nil},
		{name: "missing key", desired: `{"a":1,"b":2}`, actual: `{"a":1}`, want: []string{"$.b"}},
		{name: "different scalar", desired: `{"a":"read"}`, actual: `{"a":"write"}`, want: []string{"$.a"}},
		{name: "array length differs", desired: `{"l":[1,2]}`, actual: `{"l":[1]}`, want: []string{"$.l"}},
		{name: "nested element differs", desired: `{"l":[{"x":1},{"x":2}]}`, actual: `{"l":[{"x":1},{"x":3}]}`, want: []string{"$.l[1].x"}},
		{name: "type differs", desired: `{"m":{"k":1}}`, actual: `{"m":"k"}`, want: []string{"$.m"}},
		{name: "empty array vs missing key", desired: `{"l":[]}`, actual: `{}`, want: []string{"$.l"}},
		{name: "several mismatches are sorted by key", desired: `{"b":1,"a":1}`, actual: `{"b":2,"a":2}`, want: []string{"$.a", "$.b"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Mismatches(decode(t, tt.desired), decode(t, tt.actual))
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Mismatches = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNormalizeTurnsStructsIntoGenericJSON(t *testing.T) {
	got, err := normalize(struct {
		N int      `json:"n"`
		L []string `json:"l"`
	}{N: 2, L: []string{"a"}})
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]any{"n": float64(2), "l": []any{"a"}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("normalize = %#v, want %#v", got, want)
	}
}

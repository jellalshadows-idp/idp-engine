package claims

import "testing"

func TestDiagnosticString(t *testing.T) {
	tests := []struct {
		d    Diagnostic
		want string
	}{
		{Diagnostic{File: "claims/groups/a.yaml", Line: 3, Message: "bad"}, "claims/groups/a.yaml:3: bad"},
		{Diagnostic{File: "config/platform.yaml", Message: "file not found"}, "config/platform.yaml: file not found"},
	}
	for _, tt := range tests {
		if got := tt.d.String(); got != tt.want {
			t.Errorf("String() = %q, want %q", got, tt.want)
		}
	}
}

func TestDiagnosticAnnotationEscapes(t *testing.T) {
	d := Diagnostic{File: "claims/a,b.yaml", Line: 4, Message: "100% wrong:\nsecond line"}
	want := "::error file=claims/a%2Cb.yaml,line=4::100%25 wrong:%0Asecond line"
	if got := d.Annotation(); got != want {
		t.Errorf("Annotation() = %q, want %q", got, want)
	}
	noLine := Diagnostic{File: "config/platform.yaml", Message: "file not found"}
	if got := noLine.Annotation(); got != "::error file=config/platform.yaml::file not found" {
		t.Errorf("Annotation() without line = %q", got)
	}
}

func TestSortDiagnostics(t *testing.T) {
	ds := []Diagnostic{{File: "b", Line: 1, Message: "x"}, {File: "a", Line: 9, Message: "y"}, {File: "a", Line: 2, Message: "z"}}
	sortDiagnostics(ds)
	if ds[0].File != "a" || ds[0].Line != 2 || ds[1].Line != 9 || ds[2].File != "b" {
		t.Errorf("sorted = %v", ds)
	}
}

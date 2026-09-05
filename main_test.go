package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestValidate(t *testing.T) {
	dir := t.TempDir()

	file := filepath.Join(dir, "notadir")
	if err := os.WriteFile(file, nil, 0o644); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name    string
		cfg     config
		wantErr bool
	}{
		{"no out", config{}, false},
		{"existing dir", config{out: filepath.Join(dir, "report.html")}, false},
		{"bare filename", config{out: "report.html"}, false},
		{"missing dir", config{out: filepath.Join(dir, "gone", "report.html")}, true},
		{"parent is a file", config{out: filepath.Join(file, "report.html")}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validate(tt.cfg)
			if (err != nil) != tt.wantErr {
				t.Fatalf("validate(%+v) = %v, wantErr %v", tt.cfg, err, tt.wantErr)
			}
		})
	}
}

package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestParseArguments(t *testing.T) {
	tests := []struct {
		name      string
		arguments []string
		want      cliOptions
		wantError string
	}{
		{name: "default", want: cliOptions{directory: "."}},
		{name: "directory", arguments: []string{"/music"}, want: cliOptions{directory: "/music", explicit: true}},
		{name: "help", arguments: []string{"-h"}, want: cliOptions{help: true}},
		{name: "too many", arguments: []string{"one", "two"}, wantError: "yalnızca bir müzik klasörü"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var output bytes.Buffer
			got, err := parseArguments(test.arguments, &output)
			if test.wantError != "" {
				if err == nil || !strings.Contains(err.Error(), test.wantError) {
					t.Fatalf("parseArguments() error = %v", err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got != test.want {
				t.Fatalf("parseArguments() = %#v, want %#v", got, test.want)
			}
		})
	}
}

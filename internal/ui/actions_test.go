package ui

import (
	"testing"
	"yt-tui/internal/config"
)

func TestParseActionValues(t *testing.T) {
	tests := []struct {
		name string
		cmd  config.ActionCommand
		want []string
	}{
		{
			name: "values array",
			cmd:  config.ActionCommand{Values: []string{"Sprint 1", "Sprint 2"}},
			want: []string{"Sprint 1", "Sprint 2"},
		},
		{
			name: "single value",
			cmd:  config.ActionCommand{Value: "Sprint 5"},
			want: []string{"Sprint 5"},
		},
		{
			name: "comma separated",
			cmd:  config.ActionCommand{Value: "Sprint 1, Sprint 2"},
			want: []string{"Sprint 1", "Sprint 2"},
		},
		{
			name: "empty value clears",
			cmd:  config.ActionCommand{Value: ""},
			want: []string{},
		},
		{
			name: "values preferred over value",
			cmd:  config.ActionCommand{Value: "ignored", Values: []string{"Sprint 3"}},
			want: []string{"Sprint 3"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := parseActionValues(tc.cmd)
			if len(got) != len(tc.want) {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
			for i := range tc.want {
				if got[i] != tc.want[i] {
					t.Fatalf("got %v, want %v", got, tc.want)
				}
			}
		})
	}
}

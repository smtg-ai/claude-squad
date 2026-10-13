package app

import (
	"claude-squad/config"
	"reflect"
	"testing"
)

func TestProfilesWithProgram(t *testing.T) {
	claude := config.Profile{Name: "claude", Program: "claude"}
	aider := config.Profile{Name: "Aider", Program: "aider"}
	flag := config.Profile{Name: "claude --dangerously-skip-permissions", Program: "claude --dangerously-skip-permissions"}

	tests := []struct {
		name     string
		profiles []config.Profile
		program  string
		want     []config.Profile
	}{
		{"no flag", []config.Profile{claude, aider}, "", []config.Profile{claude, aider}},
		{"flag matches default", []config.Profile{claude, aider}, "claude", []config.Profile{claude, aider}},
		{"flag differs from default", []config.Profile{claude}, flag.Program, []config.Profile{flag, claude}},
		{"flag matches non-default profile", []config.Profile{claude, aider}, "aider", []config.Profile{aider, claude}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := profilesWithProgram(tt.profiles, tt.program); !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("got %v, want %v", got, tt.want)
			}
		})
	}
}

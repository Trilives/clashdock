package main

import (
	"testing"

	"github.com/Trilives/clashdock/internal/usermode"
)

func TestUserModeApplies(t *testing.T) {
	user := usermode.Info{Mode: usermode.ModeUser}
	service := usermode.Info{Mode: usermode.ModeService}
	tests := []struct {
		name string
		args []string
		info usermode.Info
		want bool
	}{
		{"explicit user entry in service context", []string{"user"}, service, true},
		{"legacy run alias", []string{"run"}, service, true},
		{"legacy --portable alias", []string{"--portable"}, service, true},
		{"detected user mode, no args", nil, user, true},
		{"detected user mode, pause", []string{"pause"}, user, true},
		{"detected user mode, uninstall", []string{"uninstall"}, user, true},
		{"init stays in full mode", []string{"init"}, user, false},
		{"healthcheck stays in full mode", []string{"healthcheck"}, user, false},
		{"service context, no args", nil, service, false},
		{"service context, pause", []string{"pause"}, service, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := userModeApplies(tt.args, tt.info); got != tt.want {
				t.Fatalf("userModeApplies(%v, mode=%v) = %v, want %v", tt.args, tt.info.Mode, got, tt.want)
			}
		})
	}
}

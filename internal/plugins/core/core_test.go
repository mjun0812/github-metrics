package core_test

import (
	"context"
	"testing"

	"github.com/mjun0812/github-metrics/internal/plugins"
	"github.com/mjun0812/github-metrics/internal/plugins/core"
)

func TestCore_Run_ResolvesAsiaTokyoTimezone(t *testing.T) {
	t.Parallel()

	pc := &plugins.PluginContext{
		Inputs: map[string]any{"config_timezone": "Asia/Tokyo"},
		Data:   plugins.NewData(),
	}
	if _, err := core.Plugin.Run(context.Background(), pc); err != nil {
		t.Fatalf("core.Run: %v", err)
	}
	if pc.Data.Config.Timezone.Name != "Asia/Tokyo" {
		t.Fatalf("Timezone.Name = %q, want Asia/Tokyo", pc.Data.Config.Timezone.Name)
	}
	if pc.Data.Config.Timezone.Error != nil {
		t.Fatalf("Timezone.Error = %v, want nil", pc.Data.Config.Timezone.Error)
	}
}

func TestCore_Run_FallsBackToUTCOnInvalidIANA(t *testing.T) {
	t.Parallel()

	pc := &plugins.PluginContext{
		Inputs: map[string]any{"config_timezone": "Not/AZone"},
		Data:   plugins.NewData(),
	}
	if _, err := core.Plugin.Run(context.Background(), pc); err != nil {
		t.Fatalf("core.Run: %v", err)
	}
	if pc.Data.Config.Timezone.Name != "UTC" {
		t.Fatalf("Timezone.Name = %q, want UTC", pc.Data.Config.Timezone.Name)
	}
	if pc.Data.Config.Timezone.Error == nil {
		t.Fatalf("Timezone.Error = nil, want non-nil for invalid IANA")
	}
}

func TestCore_Run_DefaultsTimezoneToUTC(t *testing.T) {
	t.Parallel()

	pc := &plugins.PluginContext{Inputs: map[string]any{}, Data: plugins.NewData()}
	if _, err := core.Plugin.Run(context.Background(), pc); err != nil {
		t.Fatalf("core.Run: %v", err)
	}
	if pc.Data.Config.Timezone.Name != "UTC" {
		t.Fatalf("Timezone.Name = %q, want UTC", pc.Data.Config.Timezone.Name)
	}
}

func TestCore_Run_InitializesComputedRepositoriesLanguages(t *testing.T) {
	t.Parallel()

	pc := &plugins.PluginContext{Data: plugins.NewData()}
	if _, err := core.Plugin.Run(context.Background(), pc); err != nil {
		t.Fatalf("core.Run: %v", err)
	}
	if pc.Data.Computed.Repositories.Languages == nil {
		t.Fatalf("Computed.Repositories.Languages should be non-nil after core.Run")
	}
}

func TestCore_Run_NilDataIsTolerated(t *testing.T) {
	t.Parallel()

	pc := &plugins.PluginContext{} // nil Data
	_, err := core.Plugin.Run(context.Background(), pc)
	if err != nil {
		t.Fatalf("core.Run with nil Data should not error, got %v", err)
	}
}

func TestCore_BooleanAndDisplayAndDebugFlags(t *testing.T) {
	t.Parallel()

	pc := &plugins.PluginContext{
		Inputs: map[string]any{
			"config_animations": "no",
			"config_base64":     true,
			"config_display":    "regular",
			"debug_flags":       []string{"a", "b"},
		},
		Data: plugins.NewData(),
	}
	if _, err := core.Plugin.Run(context.Background(), pc); err != nil {
		t.Fatalf("core.Run: %v", err)
	}
	if pc.Data.Config.Animations {
		t.Errorf("Animations should be false")
	}
	if !pc.Data.Config.Base64 {
		t.Errorf("Base64 should be true")
	}
	if pc.Data.Config.Display != "regular" {
		t.Errorf("Display = %q", pc.Data.Config.Display)
	}
	if len(pc.Data.Config.DebugFlags) != 2 {
		t.Errorf("DebugFlags len = %d", len(pc.Data.Config.DebugFlags))
	}
}

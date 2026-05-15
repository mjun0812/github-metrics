package engine_test

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/mjun0812/github-metrics/internal/config"
	"github.com/mjun0812/github-metrics/internal/engine"
	"github.com/mjun0812/github-metrics/internal/plugins"
)

func TestMarshal_TopLevelShape(t *testing.T) {
	t.Parallel()

	data := plugins.NewData()
	data.Account = plugins.AccountUser
	data.User = &plugins.User{Login: "octocat", Name: "The Octocat", AvatarURL: "https://example/x.png"}
	data.Computed.Repositories.Count = 250
	data.Computed.Repositories.Stargazers = 150
	data.Computed.Repositories.Forks = 13

	body, err := engine.Marshal(data)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if !json.Valid(body) {
		t.Fatalf("Marshal produced invalid JSON: %s", body)
	}
	var got map[string]any
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("decode: %v", err)
	}

	wantTopKeys := []string{"account", "user", "config", "computed", "plugins", "errors"}
	for _, k := range wantTopKeys {
		if _, ok := got[k]; !ok {
			t.Errorf("top-level key %q missing", k)
		}
	}
	if got["account"] != "user" {
		t.Errorf("account = %v", got["account"])
	}
	user, _ := got["user"].(map[string]any)
	if user["login"] != "octocat" {
		t.Errorf("user.login = %v", user["login"])
	}
	computed, _ := got["computed"].(map[string]any)
	repos, _ := computed["repositories"].(map[string]any)
	if repos["count"].(float64) != 250 {
		t.Errorf("computed.repositories.count = %v", repos["count"])
	}
}

func TestMarshal_TimezoneError(t *testing.T) {
	t.Parallel()

	data := plugins.NewData()
	data.Config.Timezone.Name = "UTC"
	data.Config.Timezone.Error = errors.New("invalid timezone: Not/AZone")

	body, _ := engine.Marshal(data)
	if !strings.Contains(string(body), `"invalid timezone: Not/AZone"`) {
		t.Fatalf("timezone error not surfaced into errors[]: %s", body)
	}
}

func TestMarshal_TokenNeverLeaks(t *testing.T) {
	t.Parallel()

	data := plugins.NewData()
	data.SetPlugin("traffic", config.NewToken("ghp_secret_value"))

	body, _ := engine.Marshal(data)
	if strings.Contains(string(body), "ghp_secret_value") {
		t.Fatalf("Token raw value leaked into JSON: %s", body)
	}
	if !strings.Contains(string(body), `"(provided)"`) {
		t.Fatalf("expected (provided) placeholder, got: %s", body)
	}
}

func TestMarshal_TimeAsRFC3339(t *testing.T) {
	t.Parallel()

	data := plugins.NewData()
	data.SetPlugin("ts", time.Date(2025, 1, 1, 0, 30, 0, 0, time.UTC))

	body, _ := engine.Marshal(data)
	if !strings.Contains(string(body), `"2025-01-01T00:30:00Z"`) {
		t.Fatalf("time not serialized as RFC 3339: %s", body)
	}
}

func TestMarshal_NilDataReturnsEmptyEnvelope(t *testing.T) {
	t.Parallel()

	body, err := engine.Marshal(nil)
	if err != nil {
		t.Fatalf("Marshal(nil): %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got["account"] != "" {
		t.Errorf("expected empty account for nil data, got %v", got["account"])
	}
}

func TestMarshal_NonStringKeyMapBecomesPairs(t *testing.T) {
	t.Parallel()

	data := plugins.NewData()
	data.SetPlugin("by-id", map[int]string{
		3: "c",
		1: "a",
		2: "b",
	})
	body, _ := engine.Marshal(data)
	var got struct {
		Plugins map[string]any `json:"plugins"`
	}
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	pairs, ok := got.Plugins["by-id"].([]any)
	if !ok {
		t.Fatalf("expected []pair for non-string-key map, got %T", got.Plugins["by-id"])
	}
	if len(pairs) != 3 {
		t.Fatalf("len = %d", len(pairs))
	}
	first := pairs[0].(map[string]any)
	if first["key"].(float64) != 1 {
		t.Fatalf("pairs not sorted by key (expected 1 first): %v", pairs)
	}
}

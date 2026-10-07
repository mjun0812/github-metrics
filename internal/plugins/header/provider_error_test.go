package header_test

import (
	"context"
	"errors"
	"testing"

	"github.com/mjun0812/github-metrics/internal/dataprovider/dataprovidertest"
	"github.com/mjun0812/github-metrics/internal/plugins"
	"github.com/mjun0812/github-metrics/internal/plugins/header"
	"github.com/mjun0812/github-metrics/internal/testutil/mocks"
)

// TestHeader_ProviderErrorRecordedOnData verifies a plugin-local Provider
// failure is threaded onto the shared Data accumulator (#781) so
// engine.collectPluginErrors surfaces it in the run log and honours
// plugins_errors_fatal, instead of it living only on Result.Error.
func TestHeader_ProviderErrorRecordedOnData(t *testing.T) {
	sentinel := errors.New("resource limit")
	mock := dataprovidertest.NewCountingMock()
	mock.ProfileFn = func(context.Context) (*plugins.Profile, error) { return nil, sentinel }

	pc := mocks.NewPluginContext(t, mocks.WithInputs(map[string]any{
		"user":          "octocat",
		"chrome_header": "yes",
	}))
	pc.Provider = mock

	got, err := header.Plugin.Run(context.Background(), pc)
	if err != nil {
		t.Fatalf("Run must not propagate plugin-local errors, got %v", err)
	}
	r := got.(*header.Result)
	if !errors.Is(r.Error, sentinel) {
		t.Errorf("Result.Error = %v, want wraps sentinel", r.Error)
	}
	errs := pc.Data.SnapshotErrors()
	if len(errs) != 1 || !errors.Is(errs[0], sentinel) {
		t.Errorf("Data.SnapshotErrors() = %v, want one entry wrapping sentinel", errs)
	}
}

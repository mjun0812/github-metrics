package classic_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	xerrors "github.com/mjun0812/github-metrics/internal/errors"
	"github.com/mjun0812/github-metrics/internal/plugins"
	"github.com/mjun0812/github-metrics/internal/templates"
	"github.com/mjun0812/github-metrics/internal/templates/classic"
	"github.com/mjun0812/github-metrics/internal/templates/classic/partials"
)

func TestClassic_Check_UserSVG(t *testing.T) {
	t.Parallel()
	if err := classic.Template.Check(nil, "user", "svg"); err != nil {
		t.Fatalf("Check(user,svg): %v", err)
	}
}

func TestClassic_Check_OrganizationJSON(t *testing.T) {
	t.Parallel()
	if err := classic.Template.Check(nil, "organization", "json"); err != nil {
		t.Fatalf("Check(organization,json): %v", err)
	}
}

func TestClassic_Check_RepositoryRejected(t *testing.T) {
	t.Parallel()
	err := classic.Template.Check(nil, "repository", "svg")
	if err == nil {
		t.Fatal("Check(repository,svg) should fail")
	}
	var ie *xerrors.InputError
	if !errors.As(err, &ie) {
		t.Fatalf("expected *xerrors.InputError, got %T", err)
	}
	if ie.Field != "account" {
		t.Errorf("InputError.Field = %q, want account", ie.Field)
	}
}

func TestClassic_Check_PDFUnsupported(t *testing.T) {
	t.Parallel()
	err := classic.Template.Check(nil, "user", "pdf")
	if err == nil {
		t.Fatal("Check(user,pdf) should fail")
	}
	var ufe *xerrors.UnsupportedFormatError
	if !errors.As(err, &ufe) {
		t.Fatalf("expected *xerrors.UnsupportedFormatError, got %T", err)
	}
}

func TestClassic_Check_EmptyFormatPasses(t *testing.T) {
	t.Parallel()
	// Engine handles default resolution; Check must not block empty.
	if err := classic.Template.Check(nil, "user", ""); err != nil {
		t.Fatalf("Check(user,empty): %v", err)
	}
}

func TestClassic_Metadata_AdvertisesFormats(t *testing.T) {
	t.Parallel()
	m := classic.Template.Metadata()
	if m == nil {
		t.Fatal("Metadata() nil")
	}
	wantFormats := map[string]bool{"svg": true, "png": true, "jpeg": true, "json": true}
	for _, f := range m.Formats {
		delete(wantFormats, f)
	}
	if len(wantFormats) > 0 {
		t.Errorf("metadata missing formats: %v", wantFormats)
	}
	if m.Name == "" {
		t.Errorf("metadata Name is empty")
	}
}

func TestClassic_Name(t *testing.T) {
	t.Parallel()
	if got := classic.Template.Name(); got != "classic" {
		t.Errorf("Name() = %q, want classic", got)
	}
}

func TestClassic_FSContainsExpectedFiles(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"metadata.yml", "partials/_.json", "style.css", "fonts.css"} {
		if _, err := classic.Template.FS().Open(name); err != nil {
			t.Errorf("FS missing %s: %v", name, err)
		}
	}
}

// TestClassic_HelpersExportedNamesMatchContract is a tiny smoke check
// that the classic package still exports its registered name through
// the constant `classic.Name`, used by the engine + cmd wiring.
func TestClassic_HelpersExportedNamesMatchContract(t *testing.T) {
	t.Parallel()
	if !strings.EqualFold(classic.Name, "classic") {
		t.Errorf("classic.Name = %q, want classic", classic.Name)
	}
}

// stubSkippableResult is a minimal type that the M4 dispatcher will
// accept (non-nil interface value) and recognize via the IsSkipped()
// duck-typed check.
type stubSkippableResult struct{ skipped bool }

func (s *stubSkippableResult) IsSkipped() bool { return s.skipped }

// TestClassic_Run_ZeroM4Plugins asserts the M4 plugin partial loop
// stays silent when no plugin_* inputs are truthy. The output must
// contain none of the wrapper markers, even though the existing M2
// base.* partials still run.
// Contract: contracts/partial-classic-m4.md §3 step 3a.
func TestClassic_Run_ZeroM4Plugins(t *testing.T) {
	t.Parallel()
	pc := &templates.PartialContext{
		Inputs: map[string]any{},
		Data:   &plugins.Data{},
	}
	out, err := classic.Template.Run(context.Background(), pc)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if strings.Contains(out, `data-plugin="`) {
		t.Fatalf("expected no data-plugin wrappers; output:\n%s", out)
	}
	if strings.Contains(out, `class="plugin-`) {
		t.Fatalf("expected no plugin- class wrappers; output:\n%s", out)
	}
}

func TestClassic_Run_BaseInputEmptySuppressesBaseSections(t *testing.T) {
	t.Parallel()
	data := plugins.NewData()
	data.User = &plugins.User{Login: "octocat", Name: "Octocat", AvatarURL: "https://example/avatar.png"}
	data.Computed.Repositories.Count = 51
	data.Computed.Repositories.Stargazers = 1500
	data.Computed.Repositories.Forks = 81
	pc := &templates.PartialContext{
		Inputs: map[string]any{"base": ""},
		Data:   data,
	}
	out, err := classic.Template.Run(context.Background(), pc)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	for _, marker := range []string{
		`data-section="header"`,
		`51 repositories`,
		`1.5k stargazers`,
		`81 forks`,
		`<footer>`,
	} {
		if strings.Contains(out, marker) {
			t.Fatalf("base=%q should suppress %q\noutput:\n%s", "", marker, out)
		}
	}
}

func TestClassic_Run_BaseInputMetadataRendersFooter(t *testing.T) {
	t.Parallel()
	data := plugins.NewData()
	data.Account = plugins.AccountUser
	data.Config.Timezone.Name = "Asia/Tokyo"
	pc := &templates.PartialContext{
		Inputs: map[string]any{"base": "metadata"},
		Data:   data,
	}
	out, err := classic.Template.Run(context.Background(), pc)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	for _, marker := range []string{
		`<footer>`,
		`These metrics include private contributions`,
		`Last updated `,
		`timezone Asia/Tokyo`,
		`mjun0812/github-metrics@`,
	} {
		if !strings.Contains(out, marker) {
			t.Fatalf("base metadata output missing %q\noutput:\n%s", marker, out)
		}
	}
	if strings.Contains(out, `data-section="header"`) {
		t.Fatalf("base=metadata should not render header\noutput:\n%s", out)
	}
}

// TestClassic_Run_PluginPartialWrapper asserts the M4 dispatcher
// (1) gates on plugin_<slug> truthy input, (2) skips when the result
// is Skipped, (3) emits the <div class="plugin-<slug>" data-plugin=
// "<slug>"> wrapper around a non-empty fragment from the registered
// partial. Uses the dynamically-registerable partials.Register entry
// point added with T006 + the slug "languages" (first entry in
// PluginPartialOrder).
// Contract: contracts/partial-classic-m4.md §3.
func TestClassic_Run_PluginPartialWrapper(t *testing.T) {
	const slug = "languages"
	const stubFragment = `<section class="languages-progress">stub</section>`

	// Register a stub partial under "plugin.languages" for this test
	// only. The registry is not goroutine-safe, so this subtest does
	// NOT call t.Parallel().
	prev, hadPrev := partials.Lookup("plugin." + slug)
	partials.Register("plugin."+slug, func(_ context.Context, _ *templates.PartialContext) (string, error) {
		return stubFragment, nil
	})
	t.Cleanup(func() {
		if hadPrev {
			partials.Register("plugin."+slug, prev)
		} else {
			// No clean unregister API yet — leaving an entry behind
			// would only matter if another test in the same binary
			// expected its absence. Reset by registering a no-op so
			// future Lookups return non-nil but emit "".
			partials.Register("plugin."+slug, func(_ context.Context, _ *templates.PartialContext) (string, error) {
				return "", nil
			})
		}
	})

	cases := []struct {
		name      string
		inputs    map[string]any
		result    any
		wantInOut bool
	}{
		{
			name:      "input_gate_off",
			inputs:    map[string]any{},
			result:    &stubSkippableResult{skipped: false},
			wantInOut: false,
		},
		{
			name:      "result_skipped",
			inputs:    map[string]any{"plugin_" + slug: true},
			result:    &stubSkippableResult{skipped: true},
			wantInOut: false,
		},
		{
			name:      "normal_wrapped",
			inputs:    map[string]any{"plugin_" + slug: true},
			result:    &stubSkippableResult{skipped: false},
			wantInOut: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			data := &plugins.Data{}
			data.SetPlugin(slug, tc.result)
			pc := &templates.PartialContext{
				Inputs: tc.inputs,
				Data:   data,
			}
			out, err := classic.Template.Run(context.Background(), pc)
			if err != nil {
				t.Fatalf("Run: %v", err)
			}
			wrapperOpen := `<div class="plugin-` + slug + `" data-plugin="` + slug + `">`
			contains := strings.Contains(out, wrapperOpen) && strings.Contains(out, stubFragment)
			if contains != tc.wantInOut {
				t.Fatalf("wrapper presence = %v want %v\noutput:\n%s", contains, tc.wantInOut, out)
			}
		})
	}
}

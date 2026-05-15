package classic_test

import (
	"errors"
	"strings"
	"testing"

	xerrors "github.com/mjun0812/github-metrics/internal/errors"
	"github.com/mjun0812/github-metrics/internal/templates/classic"
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

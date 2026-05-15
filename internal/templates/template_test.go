package templates_test

import (
	"context"
	"io/fs"
	"testing"
	"testing/fstest"

	"github.com/mjun0812/github-metrics/internal/config"
	xerrors "github.com/mjun0812/github-metrics/internal/errors"
	"github.com/mjun0812/github-metrics/internal/templates"
)

type fakeTemplate struct {
	name string
	meta *config.TemplateMetadata
	fsys fs.FS
	run  func(ctx context.Context, pc *templates.PartialContext) (string, error)
}

func (t *fakeTemplate) Name() string                       { return t.name }
func (t *fakeTemplate) Metadata() *config.TemplateMetadata { return t.meta }
func (t *fakeTemplate) FS() fs.FS                          { return t.fsys }
func (t *fakeTemplate) Check(_ map[string]any, _, _ string) error {
	return nil
}
func (t *fakeTemplate) Run(ctx context.Context, pc *templates.PartialContext) (string, error) {
	if t.run == nil {
		return "", nil
	}
	return t.run(ctx, pc)
}

func reset(t *testing.T) {
	t.Helper()
	t.Cleanup(func() { templates.Reset() })
	templates.Reset()
}

func TestRegister_DuplicatePanics(t *testing.T) {
	reset(t)

	templates.Register(&fakeTemplate{name: "classic"})
	defer func() {
		if r := recover(); r == nil {
			t.Fatalf("expected panic")
		}
	}()
	templates.Register(&fakeTemplate{name: "classic"})
}

func TestRegister_EmptyNamePanics(t *testing.T) {
	reset(t)
	defer func() {
		if r := recover(); r == nil {
			t.Fatalf("expected panic on empty name")
		}
	}()
	templates.Register(&fakeTemplate{name: ""})
}

func TestGet_PresentAndMissing(t *testing.T) {
	reset(t)
	tmpl := &fakeTemplate{name: "classic"}
	templates.Register(tmpl)

	got, ok := templates.Get("classic")
	if !ok || got.Name() != "classic" {
		t.Fatalf("Get classic: ok=%v got=%v", ok, got)
	}
	if _, ok := templates.Get("repository"); ok {
		t.Fatalf("Get repository should return false in M1")
	}
}

func TestMustGet_ReturnsNotFoundError(t *testing.T) {
	reset(t)

	_, err := templates.MustGet("classic")
	if err == nil {
		t.Fatalf("expected error for missing classic")
	}
	var nf *xerrors.NotFoundError
	if !xerrors.As(err, &nf) {
		t.Fatalf("expected *NotFoundError, got %T", err)
	}
	if nf.Resource != "template:classic" {
		t.Fatalf("Resource = %q", nf.Resource)
	}
}

func TestEach_SortedOrder(t *testing.T) {
	reset(t)
	for _, n := range []string{"repository", "classic"} {
		templates.Register(&fakeTemplate{name: n})
	}
	var seen []string
	_ = templates.Each(func(name string, _ templates.Template) error {
		seen = append(seen, name)
		return nil
	})
	if len(seen) != 2 || seen[0] != "classic" || seen[1] != "repository" {
		t.Fatalf("Each = %v, want [classic repository]", seen)
	}
}

func TestCheckFormat(t *testing.T) {
	meta := &config.TemplateMetadata{Formats: []string{"svg", "png", "jpeg", "json"}}

	for _, f := range []string{"", "svg", "png", "json"} {
		if err := templates.CheckFormat(meta, f); err != nil {
			t.Errorf("CheckFormat(%q) = %v, want nil", f, err)
		}
	}
	err := templates.CheckFormat(meta, "pdf")
	if err == nil {
		t.Fatalf("CheckFormat(pdf) should fail")
	}
	var uf *xerrors.UnsupportedFormatError
	if !xerrors.As(err, &uf) {
		t.Fatalf("expected *UnsupportedFormatError, got %T", err)
	}
	if uf.Format != "pdf" {
		t.Fatalf("UnsupportedFormatError.Format = %q", uf.Format)
	}
}

func TestCheckAccount(t *testing.T) {
	meta := &config.TemplateMetadata{Supports: []string{"user", "organization"}}

	for _, ok := range []string{"", "user", "organization"} {
		if err := templates.CheckAccount(meta, ok); err != nil {
			t.Errorf("CheckAccount(%q) = %v, want nil", ok, err)
		}
	}
	err := templates.CheckAccount(meta, "repository")
	if err == nil {
		t.Fatalf("CheckAccount(repository) should fail when not supported")
	}
	var ie *xerrors.InputError
	if !xerrors.As(err, &ie) {
		t.Fatalf("expected *InputError, got %T", err)
	}
}

func TestPartialContext_FSWiring(t *testing.T) {
	reset(t)
	fsys := fstest.MapFS{
		"image.svg": &fstest.MapFile{Data: []byte("<svg/>"), Mode: 0o644},
	}
	tmpl := &fakeTemplate{
		name: "stub",
		fsys: fsys,
		run: func(ctx context.Context, pc *templates.PartialContext) (string, error) {
			return "rendered", nil
		},
	}
	templates.Register(tmpl)

	got, ok := templates.Get("stub")
	if !ok {
		t.Fatalf("Get(stub) ok=false")
	}
	if _, err := fs.ReadFile(got.FS(), "image.svg"); err != nil {
		t.Fatalf("ReadFile via Template.FS: %v", err)
	}
	out, err := got.Run(context.Background(), &templates.PartialContext{})
	if err != nil || out != "rendered" {
		t.Fatalf("Run returned (%q, %v)", out, err)
	}
}

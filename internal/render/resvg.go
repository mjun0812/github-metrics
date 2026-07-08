package render

import (
	"bytes"
	"context"
	"fmt"
	"image/jpeg"
	"image/png"
	"log/slog"
	"os"
	"os/exec"

	xerrors "github.com/mjun0812/github-metrics/internal/errors"
)

// Resvg rasterizes SVG bytes by shelling out to the resvg CLI. It is
// the pipeline-agnostic replacement for the chromedp-backed *Browser:
// resvg has no browser dependency, so PNG/JPEG output no longer needs a
// running chromium. Because resvg does not measure layout, it assumes
// the incoming SVG already carries a finalized `height` — see Resize.
//
// resvg cannot emit JPEG, so the JPEG branch rasterizes to PNG and
// re-encodes via image/jpeg.
type Resvg struct {
	opts ResvgOpts
}

// ResvgOpts captures the construction-time configuration.
type ResvgOpts struct {
	// ExecPath, when non-empty, overrides the auto-detected resvg
	// binary. Falls back to the METRICS_RESVG_PATH env var and then to
	// a $PATH lookup when empty.
	ExecPath string
	// Logger is used for debug / warn events. nil falls back to
	// slog.Default().
	Logger *slog.Logger
}

const resvgPathEnv = "METRICS_RESVG_PATH"

// normalize fills in zero values with their documented defaults and
// resolves the resvg executable path. Resolution order: ExecPath →
// METRICS_RESVG_PATH env → exec.LookPath("resvg"). Returns an
// *InputError when no binary can be located so a missing resvg fails
// fast at construction rather than silently degrading.
func (o ResvgOpts) normalize() (ResvgOpts, error) {
	if o.Logger == nil {
		o.Logger = slog.Default()
	}
	if o.ExecPath == "" {
		o.ExecPath = os.Getenv(resvgPathEnv)
	}
	if o.ExecPath == "" {
		found, err := exec.LookPath("resvg")
		if err != nil {
			return o, xerrors.NewInputError(resvgPathEnv,
				fmt.Errorf("resvg binary not found: set %s or add resvg to PATH: %w", resvgPathEnv, err))
		}
		o.ExecPath = found
	}
	return o, nil
}

// NewResvg constructs a Resvg. The resvg binary is stat-checked up
// front so a missing or unreadable executable fails fast instead of
// producing a confusing exec error on the first Resize call.
func NewResvg(opts ResvgOpts) (*Resvg, error) {
	opts, err := opts.normalize()
	if err != nil {
		return nil, err
	}
	if _, err := os.Stat(opts.ExecPath); err != nil {
		return nil, xerrors.NewInputError(resvgPathEnv,
			fmt.Errorf("resvg binary not found at %s: %w", opts.ExecPath, err))
	}
	return &Resvg{opts: opts}, nil
}

// Resize implements Renderer. Unlike *Browser it performs no layout
// measurement: the input SVG is expected to already carry a finalized
// `height` (native SVGs written by the pipeline satisfy this). The svg
// branch is therefore a pass-through, and the returned Width/Height for
// the raster branches come from decoding the rasterized PNG.
//
// resvg subprocess failures are wrapped as *xerrors.RetryableError so
// the engine dispatch can surface them via Result.Errors while
// preserving the typed shape callers match on with errors.As.
func (r *Resvg) Resize(ctx context.Context, in string, opts ResizeOpts) (ResizeResult, error) {
	opts, err := opts.normalize()
	if err != nil {
		return ResizeResult{}, err
	}

	if opts.Convert == "svg" {
		return ResizeResult{
			Body: []byte(in),
			MIME: mimeForConvert("svg"),
		}, nil
	}

	// PNG / JPEG both rasterize to PNG first via the resvg subprocess.
	pngBytes, err := r.rasterizePNG(ctx, in)
	if err != nil {
		return ResizeResult{}, err
	}
	img, err := png.Decode(bytes.NewReader(pngBytes))
	if err != nil {
		return ResizeResult{}, xerrors.NewRetryableError(fmt.Errorf("render: decode resvg png: %w", err))
	}
	bnds := img.Bounds()

	if opts.Convert == "png" {
		return ResizeResult{
			Body:   pngBytes,
			Width:  bnds.Dx(),
			Height: bnds.Dy(),
			MIME:   mimeForConvert("png"),
		}, nil
	}

	// jpeg: resvg cannot emit JPEG, so re-encode the decoded PNG.
	var jpegBuf bytes.Buffer
	if err := jpeg.Encode(&jpegBuf, img, &jpeg.Options{Quality: 100}); err != nil {
		return ResizeResult{}, xerrors.NewRetryableError(fmt.Errorf("render: encode jpeg: %w", err))
	}
	return ResizeResult{
		Body:   jpegBuf.Bytes(),
		Width:  bnds.Dx(),
		Height: bnds.Dy(),
		MIME:   mimeForConvert("jpeg"),
	}, nil
}

// rasterizePNG runs `resvg - -c`, streaming the SVG in via stdin and
// reading the PNG back from stdout. Streaming avoids temp files; the
// SVGs the pipeline produces are self-contained (images are inlined),
// so resvg's stdin resources-dir warning does not apply.
func (r *Resvg) rasterizePNG(ctx context.Context, in string) ([]byte, error) {
	// #nosec G204 -- ExecPath comes from caller config / METRICS_RESVG_PATH,
	// both trusted in this CLI's threat model (METRICS_CHROME_PATH parity).
	cmd := exec.CommandContext(ctx, r.opts.ExecPath, "-", "-c")
	cmd.Stdin = bytes.NewReader([]byte(in))
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, xerrors.NewRetryableError(
			fmt.Errorf("render: resvg run: %w: %s", err, bytes.TrimSpace(stderr.Bytes())))
	}
	return stdout.Bytes(), nil
}

// Compile-time interface check: catch signature drift between the
// Renderer contract and *Resvg.
var _ Renderer = (*Resvg)(nil)

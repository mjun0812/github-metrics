package render

import (
	"bytes"
	"context"
	"fmt"
	"image/jpeg"
	"image/png"
	"log/slog"
	"math"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"

	xerrors "github.com/mjun0812/github-metrics/internal/errors"
)

// Resvg rasterizes SVG bytes by shelling out to the resvg CLI. It is
// the Renderer used in production: resvg has no browser dependency, so
// PNG/JPEG output needs no running browser. Because resvg does not
// measure layout, it assumes the incoming SVG already carries a
// finalized `height` — see Resize.
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

// Generic-family fonts passed to resvg. The SVG font stacks end in the
// CSS generic keywords `sans-serif` (body text) and `monospace` (code /
// language rows); every *named* family before them (-apple-system,
// Segoe UI, Helvetica, Arial, …) is a macOS/Windows font absent from
// the Linux Docker image, so resvg resolves the stack through these
// generics. Without a concrete mapping resvg cannot match the stack and
// silently *skips every <text>* (verified against #409 Phase C). We map
// the generics to the Liberation family, which the runtime image ships
// via fonts-liberation and which is metric-compatible with Arial /
// Times New Roman / Courier New.
const (
	fontSansSerif = "Liberation Sans"
	fontSerif     = "Liberation Serif"
	fontMonospace = "Liberation Mono"
)

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

// Resize implements Renderer. Unlike the removed browser renderer it
// performs no layout measurement: the input SVG is expected to already
// carry a finalized `height` (native SVGs written by the pipeline
// satisfy this). The only geometric transform is the optional
// `config.padding` expansion, applied by rewriting the root <svg>
// width/height/viewBox in Go (pure arithmetic — see applyPadding). The
// svg branch is therefore a padding-only rewrite, and the raster
// branches rasterize the padded SVG.
//
// resvg subprocess failures are wrapped as *xerrors.RetryableError so
// the engine dispatch can surface them via Result.Errors while
// preserving the typed shape callers match on with errors.As.
func (r *Resvg) Resize(ctx context.Context, in string, opts ResizeOpts) (ResizeResult, error) {
	opts, err := opts.normalize()
	if err != nil {
		return ResizeResult{}, err
	}

	padded, w, h := applyPadding(in, parsePadding(opts.Padding, r.opts.Logger))

	if opts.Convert == "svg" {
		return ResizeResult{
			Body:   []byte(padded),
			Width:  w,
			Height: h,
			MIME:   mimeForConvert("svg"),
		}, nil
	}

	// PNG / JPEG both rasterize to PNG first via the resvg subprocess.
	pngBytes, err := r.rasterizePNG(ctx, padded)
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

// rasterizePNG runs `resvg [font flags] - -c`, streaming the SVG in via
// stdin and reading the PNG back from stdout. Streaming avoids temp
// files; the SVGs the pipeline produces are self-contained (images are
// inlined), so resvg's stdin resources-dir warning does not apply. The
// generic-family flags let resvg resolve the SVG font stacks — see the
// font* constants for why they are mandatory.
func (r *Resvg) rasterizePNG(ctx context.Context, in string) ([]byte, error) {
	// #nosec G204 -- ExecPath comes from caller config / METRICS_RESVG_PATH,
	// trusted in this CLI's threat model. The remaining args are constants.
	cmd := exec.CommandContext(ctx, r.opts.ExecPath,
		"--sans-serif-family", fontSansSerif,
		"--serif-family", fontSerif,
		"--monospace-family", fontMonospace,
		"--font-family", fontSansSerif,
		"-", "-c")
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

// svgOpenTagRe captures the root <svg …> opening tag so padding
// rewrites stay scoped to it (nested <svg> partials keep their dims).
var svgOpenTagRe = regexp.MustCompile(`(?s)<svg\b[^>]*?>`)

// svgWidthRe / svgHeightRe / svgViewBoxRe capture a single attribute
// value inside the root tag. width/height are numeric; viewBox is
// four space/comma separated numbers "minX minY w h".
var (
	svgWidthRe   = regexp.MustCompile(`(\bwidth=")([0-9.]+)(")`)
	svgHeightRe  = regexp.MustCompile(`(\bheight=")([0-9.]+)(")`)
	svgViewBoxRe = regexp.MustCompile(`(\bviewBox=")([^"]*)(")`)
)

// applyPadding expands the rasterized canvas per the parsed padding
// spec. The height is already finalized in the SVG, so this is pure
// arithmetic — no browser measurement. With the default (empty / "0")
// padding it is a no-op that just reports the intrinsic width/height.
//
// A non-trivial padding rewrites the root <svg> width, height, and the
// viewBox's w/h to width*mult+abs / height*mult+abs (ceil). Growing the
// viewBox in lockstep with width/height keeps the 1:1 unit→pixel
// mapping, so the extra space renders as transparent margin on the
// right / bottom rather than scaling the content (upstream padding
// semantics). Returns the (possibly rewritten) SVG and the final
// integer dimensions; when the root dims cannot be parsed the input is
// returned unchanged with zero dims.
func applyPadding(in string, pad padding) (string, int, int) {
	tagLoc := svgOpenTagRe.FindStringIndex(in)
	if tagLoc == nil {
		return in, 0, 0
	}
	tag := in[tagLoc[0]:tagLoc[1]]

	w, okW := attrFloat(svgWidthRe, tag)
	h, okH := attrFloat(svgHeightRe, tag)
	if !okW || !okH {
		return in, 0, 0
	}

	trivial := pad.width == 1 && pad.height == 1 &&
		pad.absoluteWidth == 0 && pad.absoluteHeight == 0
	if trivial {
		return in, int(w), int(h)
	}

	newW := int(math.Max(1, math.Ceil(w*pad.width+pad.absoluteWidth)))
	newH := int(math.Max(1, math.Ceil(h*pad.height+pad.absoluteHeight)))

	newTag := svgWidthRe.ReplaceAllString(tag, "${1}"+strconv.Itoa(newW)+"$3")
	newTag = svgHeightRe.ReplaceAllString(newTag, "${1}"+strconv.Itoa(newH)+"$3")
	newTag = svgViewBoxRe.ReplaceAllStringFunc(newTag, func(m string) string {
		sub := svgViewBoxRe.FindStringSubmatch(m)
		fields := strings.Fields(strings.ReplaceAll(sub[2], ",", " "))
		if len(fields) != 4 {
			return m
		}
		return sub[1] + fields[0] + " " + fields[1] + " " +
			strconv.Itoa(newW) + " " + strconv.Itoa(newH) + sub[3]
	})

	return in[:tagLoc[0]] + newTag + in[tagLoc[1]:], newW, newH
}

// attrFloat pulls the numeric group-2 value of re out of tag.
func attrFloat(re *regexp.Regexp, tag string) (float64, bool) {
	m := re.FindStringSubmatch(tag)
	if m == nil {
		return 0, false
	}
	v, err := strconv.ParseFloat(m[2], 64)
	if err != nil {
		return 0, false
	}
	return v, true
}

// Compile-time interface check: catch signature drift between the
// Renderer contract and *Resvg.
var _ Renderer = (*Resvg)(nil)

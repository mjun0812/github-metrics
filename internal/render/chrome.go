package render

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"sync"

	"github.com/chromedp/chromedp"

	xerrors "github.com/mjun0812/github-metrics/internal/errors"
)

// Browser owns a single chromedp ExecAllocator plus the parent
// context it spawns tabs from. A Browser is safe for concurrent use;
// NewTab serializes against the internal recycle mutex.
//
// Upstream-compatible defaults:
//   - --no-sandbox, --disable-extensions, --disable-dev-shm-usage,
//     --disable-gpu, --single-process.
//   - METRICS_CHROME_PATH env var resolves the chromium executable
//     when BrowserOpts.ExecPath is empty.
//   - Recycle counter trips at BrowserOpts.RecycleEvery (defaults to
//     200, upstream `svg.resize.browser` parity).
type Browser struct {
	opts BrowserOpts

	mu           sync.Mutex
	allocCtx     context.Context
	allocCancel  context.CancelFunc
	parentCtx    context.Context
	parentCancel context.CancelFunc
	counter      int
	recycles     int
	closed       bool
}

// BrowserOpts captures the construction-time configuration. Zero
// values map to upstream-compatible defaults.
type BrowserOpts struct {
	// ExecPath, when non-empty, overrides chromedp's auto-detect path
	// for the chromium / chrome binary. Falls back to the
	// METRICS_CHROME_PATH env var when empty.
	ExecPath string
	// RecycleEvery sets how many NewTab calls trigger an allocator
	// re-init. Zero falls back to the default (200).
	RecycleEvery int
	// Headless toggles the --headless flag. Default true. Set false
	// only for local debugging via --puppeteer-disable-headless.
	Headless bool
	// ExtraFlags are appended to the chromedp ExecAllocator option
	// set after the upstream-compatible defaults are applied.
	ExtraFlags []chromedp.ExecAllocatorOption
	// Logger is used for debug / warn events. nil falls back to
	// slog.Default().
	Logger *slog.Logger
}

const (
	defaultRecycleEvery = 200
	chromePathEnv       = "METRICS_CHROME_PATH"
)

// normalize fills in zero values with their documented defaults. We do
// NOT mutate Headless on its zero value because false is a valid
// caller choice; New explicitly applies the Headless=true default by
// using a pointer-style ApplyHeadlessDefault helper.
func (o BrowserOpts) normalize(applyHeadlessDefault bool) BrowserOpts {
	if o.RecycleEvery <= 0 {
		o.RecycleEvery = defaultRecycleEvery
	}
	if o.Logger == nil {
		o.Logger = slog.Default()
	}
	if applyHeadlessDefault {
		o.Headless = true
	}
	if o.ExecPath == "" {
		o.ExecPath = os.Getenv(chromePathEnv)
	}
	return o
}

// New constructs a Browser. The returned instance owns chromedp
// resources; callers MUST call Close when done.
//
// When BrowserOpts.ExecPath is non-empty, the file is stat-checked up
// front so a missing chromium binary fails fast instead of producing a
// confusing chromedp.WaitProcess error several seconds later.
func New(opts BrowserOpts) (*Browser, error) {
	// Detect zero-value Headless before normalize forces it true.
	wantHeadlessDefault := !opts.Headless
	opts = opts.normalize(wantHeadlessDefault)

	if opts.ExecPath != "" {
		if _, err := os.Stat(opts.ExecPath); err != nil {
			return nil, xerrors.NewInputError("METRICS_CHROME_PATH",
				fmt.Errorf("chromium binary not found at %s: %w", opts.ExecPath, err))
		}
	}

	b := &Browser{opts: opts}
	b.spawnAllocator()
	return b, nil
}

// spawnAllocator (re)initializes the ExecAllocator + parent context.
// Callers MUST hold b.mu when invoking this function — either via the
// Browser construction path (single-threaded) or under the recycle
// mutex. chromedp.NewExecAllocator does not return an error, so this
// function does not either; the binary existence check happens up
// front in New.
func (b *Browser) spawnAllocator() {
	flags := []chromedp.ExecAllocatorOption{
		chromedp.NoFirstRun,
		chromedp.NoDefaultBrowserCheck,
		chromedp.Flag("no-sandbox", true),
		chromedp.Flag("disable-extensions", true),
		chromedp.Flag("disable-dev-shm-usage", true),
		chromedp.Flag("disable-gpu", true),
		// --single-process is documented as required for Docker/ARM
		// stability but breaks the CDP Network
		// domain enable step on modern desktop Chrome (137+). Callers
		// that need it MUST pass it via BrowserOpts.ExtraFlags; the
		// default set leaves it off so local dev / CI without the
		// chromedp/headless-shell image works out of the box.
	}
	if b.opts.Headless {
		flags = append(flags, chromedp.Headless)
	}
	if b.opts.ExecPath != "" {
		flags = append(flags, chromedp.ExecPath(b.opts.ExecPath))
	}
	flags = append(flags, b.opts.ExtraFlags...)

	allocCtx, allocCancel := chromedp.NewExecAllocator(context.Background(), flags...)
	parentCtx, parentCancel := chromedp.NewContext(allocCtx)

	b.allocCtx = allocCtx
	b.allocCancel = allocCancel
	b.parentCtx = parentCtx
	b.parentCancel = parentCancel
	b.counter = 0
}

// NewTab returns a child context bound to a fresh chromedp tab.
// Callers MUST call the returned cancel func (typically via defer)
// when finished with the tab. The supplied parent ctx provides the
// cancellation / deadline plumbing for the caller; chromedp tabs are
// spawned from b.parentCtx so they survive the parent ctx going out
// of scope until cancel runs.
//
// NewTab transparently recycles the underlying allocator after every
// `RecycleEvery` invocations to avoid long-lived chromium memory
// growth. Concurrent NewTab calls block during a recycle.
func (b *Browser) NewTab(_ context.Context) (context.Context, context.CancelFunc, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.closed {
		return nil, func() {}, xerrors.NewInputError("Browser",
			fmt.Errorf("render: NewTab called on a closed Browser"))
	}

	if b.counter >= b.opts.RecycleEvery {
		b.recycleLocked()
	}

	tabCtx, tabCancel := chromedp.NewContext(b.parentCtx)
	b.counter++
	return tabCtx, tabCancel, nil
}

// recycleLocked cancels the existing allocator and spawns a new one.
// Callers MUST hold b.mu.
func (b *Browser) recycleLocked() {
	b.opts.Logger.Debug("render: Browser recycling allocator", "after_uses", b.counter)
	if b.parentCancel != nil {
		b.parentCancel()
	}
	if b.allocCancel != nil {
		b.allocCancel()
	}
	b.spawnAllocator()
	b.recycles++
}

// Close cancels the allocator and waits for cleanup. Idempotent.
// Subsequent NewTab calls return an *InputError.
func (b *Browser) Close() error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		return nil
	}
	b.closed = true
	if b.parentCancel != nil {
		b.parentCancel()
	}
	if b.allocCancel != nil {
		b.allocCancel()
	}
	return nil
}

// RecycledCount returns how many allocator re-inits have occurred
// since construction. Exposed for tests so we can assert that the
// recycle-every-N counter trips at the expected moment without
// reaching into private fields.
func (b *Browser) RecycledCount() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.recycles
}

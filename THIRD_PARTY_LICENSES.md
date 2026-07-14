# Third-party licenses

This project bundles and redistributes several third-party works. Their licenses are reproduced (or referenced) below in fulfillment of each license's attribution / redistribution requirements. The MIT terms for this project itself live in [`LICENSE`](LICENSE).

---

## lowlighter/metrics — MIT

<https://github.com/lowlighter/metrics>

This project is a Go port of a subset of `lowlighter/metrics`. Adopted plugins, templates, and JSON envelopes are byte- or DOM-compatible with the corresponding parts of that upstream project.

```
MIT License

Copyright (c) lowlighter and contributors

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.
```

---

## GitHub Primer Octicons — MIT

Version: `19.25.0` — <https://github.com/primer/octicons>

The SVG icons referenced across templates come from Primer/Octicons. The pruned dataset is committed to this repository as [`assets/octicons/data.json`](assets/octicons/data.json) and embedded into the binary at build time.

```
MIT License

Copyright (c) GitHub, Inc.

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.
```

---

## Liberation Sans — SIL Open Font License 1.1

Upstream: <https://github.com/liberationfonts/liberation-fonts>

The Liberation Sans TrueType font is embedded via `//go:embed` in [`internal/render/fontmetrics/fonts/`](internal/render/fontmetrics/fonts/) and used for Go-side text-width measurement. The full OFL-1.1 license text with the Google + Red Hat copyright notices is at [`internal/render/fontmetrics/fonts/LICENSE.txt`](internal/render/fontmetrics/fonts/LICENSE.txt) and is preserved verbatim.

Redistribution requirements per OFL-1.1:

- The font is bundled with its copyright notice and full license text (satisfied by `LICENSE.txt` above).
- The font is not sold on its own.
- Modified copies of the font are not carried under the same name.

---

## Noto Sans CJK JP + Noto Color Emoji — SIL Open Font License 1.1

Upstream: <https://github.com/googlefonts/noto-cjk> · <https://github.com/googlefonts/noto-emoji>

The Docker runtime image installs Noto Sans CJK JP and Noto Color Emoji via `apt-get install fonts-noto-cjk fonts-noto-color-emoji` (see [`Dockerfile`](Dockerfile)) so resvg can rasterize CJK and emoji glyphs. These fonts are supplied by Debian's packaging and remain governed by OFL-1.1. The OFL text is not duplicated here — Debian ships it under `/usr/share/doc/fonts-noto-cjk/copyright` and `/usr/share/doc/fonts-noto-color-emoji/copyright` inside the resulting image.

---

## resvg — MIT OR Apache-2.0

Version: `0.47.0` — <https://github.com/linebender/resvg>

The `resvg` SVG rasterizer is installed at Docker image build time (`cargo install`) and invoked as a subprocess by [`internal/render/resvg.go`](internal/render/resvg.go) for PNG / JPEG output. It is not bundled into the Go binary. See the upstream repository's [`LICENSE-MIT`](https://github.com/linebender/resvg/blob/master/LICENSE-MIT) and [`LICENSE-APACHE`](https://github.com/linebender/resvg/blob/master/LICENSE-APACHE) for the full texts.

---

## Go module dependencies

Every direct and transitive Go dependency listed in [`go.mod`](go.mod) is licensed under a permissive OSI license (MIT, BSD-2/3, Apache-2.0). No GPL / AGPL / LGPL / MPL licensed code is linked into the binary. The upstream repository of each dependency ships its own `LICENSE` file; consult [`pkg.go.dev`](https://pkg.go.dev) for individual notices.

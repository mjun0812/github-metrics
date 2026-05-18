package topics

import (
	"context"
	"fmt"
	"strings"

	"github.com/mjun0812/github-metrics/internal/templates"
	"github.com/mjun0812/github-metrics/internal/templates/classic/partials"
)

func init() {
	partials.Register("plugin."+Name, Partial)
}

// pinOcticon is the upstream `<%- octicon "pin" %>` 16x16 path used in
// the topics section header. Mirrors EJS line 4.
const pinOcticon = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 16 16" width="16" height="16" aria-hidden="true"><path fill-rule="evenodd" d="M14.184 1.143a1.75 1.75 0 00-2.502-.57L.912 7.916a1.75 1.75 0 00-.53 2.32l.447.775a1.75 1.75 0 002.275.702l11.745-5.656a1.75 1.75 0 00.757-2.451l-1.422-2.464zm-1.657.669a.25.25 0 01.358.081l1.422 2.464a.25.25 0 01-.108.35l-2.016.97-1.505-2.605 1.85-1.26zM9.436 3.92l1.391 2.41-5.42 2.61-.942-1.63 4.97-3.39zM3.222 8.157l-1.466 1a.25.25 0 00-.075.33l.447.775a.25.25 0 00.325.1l1.598-.769-.83-1.436zm6.253 2.306a.75.75 0 00-.944-.252l-1.809.87a.75.75 0 00-.293.253L4.38 14.326a.75.75 0 101.238.848l1.881-2.75v2.826a.75.75 0 001.5 0v-2.826l1.881 2.75a.75.75 0 001.238-.848l-2.644-3.863z"></path></svg>`

// headerLabel mirrors upstream EJS line 5:
//
//	{starred:"Starred topics", labels:"Starred topics",
//	 icons:"Starred topics", mastered:"Mastered technologies and topics"}[mode]
func headerLabel(mode string) string {
	switch mode {
	case "mastered":
		return "Mastered technologies and topics"
	default:
		return "Starred topics"
	}
}

// renderType derives the EJS `plugins.topics.type` switch value. Upstream
// sets type = mode (starred / labels / icons). For our pragmatic
// implementation: "icons" mode emits images; everything else emits text
// labels.
func renderType(mode string) string {
	switch mode {
	case "icons":
		return "icons"
	default:
		return "labels"
	}
}

// Partial renders the classic SVG fragment for the topics plugin.
// Returns "" when the result is missing or skipped.
//
// Output structure (mirrors upstream
// org_repo/source/templates/classic/partials/topics.ejs):
//
//	<section data-section="topics">
//	  <section>
//	    <h2 class="field"><svg pin/>Starred topics</h2>
//	    <div class="row">
//	      <section>
//	        <div class="topics fill-width">
//	          [labels mode]: <div class="label" title="desc">name</div>...
//	          [icons mode]:  <img src="icon" width="24" height="24" alt="name" title="name"/>...
//	        </div>
//	      </section>
//	    </div>
//	  </section>
//	</section>
//
// Spec: specs/011-plugin-rendering-parity/PLAN_V2_MJUN0812_FOCUS.md §3 (topics)
// Settings: mjun0812 uses plugin_topics: yes, plugin_topics_limit: 15,
// plugin_topics_mode: starred (default) and a variant with mode: icons.
func Partial(_ context.Context, pc *templates.PartialContext) (string, error) {
	if pc == nil || pc.Data == nil {
		return "", nil
	}
	raw, ok := pc.Data.GetPlugin(Name)
	if !ok || raw == nil {
		return "", nil
	}
	r, ok := raw.(*Result)
	if !ok || r == nil || r.Skipped {
		return "", nil
	}

	mode := r.Mode
	if mode == "" {
		mode = "starred"
	}
	typ := renderType(mode)

	var b strings.Builder
	b.WriteString(`<section data-section="topics">`)
	b.WriteString(`<section>`)

	// Header: <h2 class="field"><svg pin/>${label}</h2>
	fmt.Fprintf(&b, `<h2 class="field">%s%s</h2>`, pinOcticon, partials.EscapeXML(headerLabel(mode)))

	b.WriteString(`<div class="row">`)
	b.WriteString(`<section>`)
	b.WriteString(`<div class="topics fill-width">`)

	// Body content per mode (matches EJS lines 18-28).
	if len(r.List) > 0 {
		switch typ {
		case "icons":
			for _, t := range r.List {
				if t.Icon == "" {
					continue
				}
				fmt.Fprintf(
					&b,
					`<img src="%s" width="24" height="24" alt="%s" title="%s"/>`,
					partials.EscapeXML(t.Icon),
					partials.EscapeXML(t.Name),
					partials.EscapeXML(t.Name),
				)
			}
		default: // "labels"
			for _, t := range r.List {
				fmt.Fprintf(
					&b,
					`<div class="label" title="%s">%s</div>`,
					partials.EscapeXML(t.Description),
					partials.EscapeXML(strings.ToLower(t.Name)),
				)
			}
		}
	}

	b.WriteString(`</div>`)
	b.WriteString(`</section>`)
	b.WriteString(`</div>`)
	b.WriteString(`</section>`)
	b.WriteString(`</section>`)
	return b.String(), nil
}

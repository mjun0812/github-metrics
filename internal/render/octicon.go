package render

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"sync"

	"github.com/mjun0812/github-metrics/assets"
)

// octiconRe matches `:octicon-<name>(-<16|24>)?:`. The name uses a
// lazy quantifier so the optional size suffix can win at the tail —
// without it, ":octicon-chevron-down-24:" would match name="chevron"
// instead of name="chevron-down". The size group is restricted to 16
// or 24 to match @primer/octicons' published sizes.
var octiconRe = regexp.MustCompile(`:octicon-([a-z0-9-]+?)(?:-(16|24))?:`)

type octiconDoc struct {
	Meta  map[string]string            `json:"_meta"`
	Icons map[string]map[string]string `json:"icons"`
}

// octiconData holds the deserialized asset. We parse it once via
// sync.Once so the (200KB JSON) decode cost is paid lazily, on the
// first ReplaceOcticons call.
var (
	octiconOnce sync.Once
	octiconData octiconDoc
	octiconErr  error
)

func loadOcticonData() (*octiconDoc, error) {
	octiconOnce.Do(func() {
		raw, err := assets.OcticonData()
		if err != nil {
			octiconErr = fmt.Errorf("render: read octicon asset: %w", err)
			return
		}
		octiconErr = json.Unmarshal(raw, &octiconData)
	})
	if octiconErr != nil {
		return nil, fmt.Errorf("render: load octicon data: %w", octiconErr)
	}
	return &octiconData, nil
}

// ReplaceOcticons swaps every `:octicon-<name>(-<size>)?:` placeholder
// in `in` for the matching SVG fragment from assets/octicons/data.json,
// injecting `class="octicon"` so downstream CSS targets it. Unknown
// (name, size) combinations pass through verbatim.
//
// The function is idempotent: ReplaceOcticons(ReplaceOcticons(in))
// equals ReplaceOcticons(in) because the rendered `<svg class="octicon">`
// fragments no longer contain the source placeholder pattern.
func ReplaceOcticons(in string) (string, error) {
	if in == "" {
		return "", nil
	}
	data, err := loadOcticonData()
	if err != nil {
		return in, err
	}

	out := octiconRe.ReplaceAllStringFunc(in, func(match string) string {
		groups := octiconRe.FindStringSubmatch(match)
		// groups[0]=match, [1]=name, [2]=size ("" or "16"/"24").
		name := groups[1]
		size := groups[2]
		if size == "" {
			size = "16"
		}
		variants, ok := data.Icons[name]
		if !ok {
			return match
		}
		fragment, ok := variants[size]
		if !ok {
			return match
		}
		return injectOcticonClass(fragment)
	})
	return out, nil
}

// injectOcticonClass adds `octicon` to the SVG root's class list. When
// the fragment already has a `class="..."` attribute, we append; when
// it does not, we splice in `class="octicon"` right after the opening
// `<svg`.
func injectOcticonClass(fragment string) string {
	const openTag = "<svg"
	idx := strings.Index(fragment, openTag)
	if idx < 0 {
		return fragment
	}
	// Locate the closing '>' of the opening <svg ...> tag.
	close := strings.Index(fragment[idx:], ">")
	if close < 0 {
		return fragment
	}
	head := fragment[:idx+close]
	tail := fragment[idx+close:]

	if strings.Contains(head, " class=\"") {
		head = strings.Replace(head, ` class="`, ` class="octicon `, 1)
		return head + tail
	}
	return head + ` class="octicon"` + tail
}

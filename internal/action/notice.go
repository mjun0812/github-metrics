package action

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mjun0812/github-metrics/internal/githubapi"
)

// CheckLatestRelease consults the upstream-style release feed of this
// repository and returns a notice message when a newer version is
// available. The result is intentionally a string (empty when no
// notice needed) so callers can do `if msg != "" { slog.Info(msg) }`.
//
// Spec FR-021. Failures (network 5xx, missing repo) return "" with
// no error so the action does not fail on a notice-channel hiccup —
// notices are best-effort.
//
// repo argument format: "owner/name" (e.g. "mjun0812/github-metrics").
func CheckLatestRelease(ctx context.Context, rest *githubapi.REST, repo, currentVersion string) string {
	if rest == nil || repo == "" || currentVersion == "" {
		return ""
	}
	body, resp, err := rest.Get(ctx, "/repos/"+repo+"/releases/latest", nil)
	if err != nil || resp == nil || resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return ""
	}
	var doc struct {
		TagName string `json:"tag_name"`
		HTMLURL string `json:"html_url"`
	}
	if jerr := json.Unmarshal(body, &doc); jerr != nil {
		return ""
	}
	latest := strings.TrimPrefix(doc.TagName, "v")
	current := strings.TrimPrefix(currentVersion, "v")
	if latest == "" || latest == current {
		return ""
	}
	return fmt.Sprintf(
		"A newer version v%s is available (you are running v%s). Release notes: %s",
		latest, current, doc.HTMLURL,
	)
}

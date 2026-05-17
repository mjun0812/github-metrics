package action

import (
	"encoding/json"
	"os"
	"strconv"
	"strings"
)

// SkipReason is the human-readable reason action.Run skipped without
// invoking engine.Compute. Returned by shouldSkip; logged by Run.
type SkipReason string

// shouldSkip reads the GitHub workflow event payload at eventPath and
// inspects the triggering commit's message for upstream-compatible
// skip markers. Returns (true, reason) when the action should
// short-circuit with exit 0.
//
// Spec FR-002. Two recognized markers (case-sensitive, substring match):
//
//   - [Skip GitHub Action]
//   - Auto-generated metrics for run #<N>
//
// The first is the explicit user-driven skip marker; the second is the
// committer's default message (FR-014 / committer.go), included so a
// metrics-action run triggered by its own prior commit does not loop
// indefinitely (e.g., on a `push` trigger).
func shouldSkip(eventPath string) (bool, SkipReason) {
	if eventPath == "" {
		return false, ""
	}
	body, err := os.ReadFile(eventPath) //nolint:gosec // eventPath is the GitHub Actions runner-supplied path
	if err != nil {
		return false, ""
	}
	var event struct {
		HeadCommit struct {
			Message string `json:"message"`
		} `json:"head_commit"`
		// `pull_request` events have the message in a different place;
		// we only support the most common `push` case for now (FR-002
		// targets schedule + push triggers).
	}
	if jerr := json.Unmarshal(body, &event); jerr != nil {
		return false, ""
	}
	msg := event.HeadCommit.Message
	if msg == "" {
		return false, ""
	}
	for _, marker := range []string{
		"[Skip GitHub Action]",
		"Auto-generated metrics for run #",
	} {
		if strings.Contains(msg, marker) {
			return true, SkipReason("commit message contains marker " + strconv.Quote(marker))
		}
	}
	return false, ""
}

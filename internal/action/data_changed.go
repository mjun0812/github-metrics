package action

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	xerrors "github.com/mjun0812/github-metrics/internal/errors"
	"github.com/mjun0812/github-metrics/internal/githubapi"
	"github.com/mjun0812/github-metrics/internal/render"
)

// HashComparator decides whether the freshly-rendered body matches
// the file already present in the target branch. When equal, the
// committer can skip the PUT /contents call so workflow runs that
// produce identical output don't pollute the commit log.
//
// Spec FR-012 / FR-013. The comparison uses M3's render.Hash to avoid
// the per-byte differences (version literals, generated_at timestamps)
// that a raw bytes.Equal would mistake for "data changed".
type HashComparator struct {
	REST      *githubapi.REST
	RepoOwner string
	RepoName  string
	Branch    string
	Filename  string
	NewBody   []byte
}

// Equal returns true when the existing remote file's render.Hash
// matches the hash of NewBody. Returns (false, nil) when the remote
// file does not exist (404) so the committer treats first-runs as
// "data changed" and commits normally.
//
// Transport errors and 5xx responses are wrapped as
// *xerrors.RetryableError so the RetryPolicy can retry them.
func (c *HashComparator) Equal(ctx context.Context) (bool, error) {
	if c.REST == nil {
		return false, fmt.Errorf("data-changed: nil REST client")
	}
	path := fmt.Sprintf("/repos/%s/%s/contents/%s", c.RepoOwner, c.RepoName, c.Filename)
	if c.Branch != "" {
		path += "?ref=" + c.Branch
	}
	body, resp, err := c.REST.Get(ctx, path, nil)
	if err != nil {
		return false, wrapRetryable(err, resp)
	}
	if resp == nil {
		return false, xerrors.NewRetryableError(fmt.Errorf("data-changed: nil response"))
	}
	switch resp.StatusCode {
	case http.StatusNotFound:
		// New file — "data changed" by definition.
		return false, nil
	case http.StatusOK:
		// fall through
	default:
		if resp.StatusCode >= 500 {
			return false, xerrors.NewRetryableError(
				fmt.Errorf("data-changed: status %d", resp.StatusCode),
			)
		}
		return false, fmt.Errorf("data-changed: unexpected status %d", resp.StatusCode)
	}

	var doc struct {
		Content  string `json:"content"`
		Encoding string `json:"encoding"`
	}
	if jerr := json.Unmarshal(body, &doc); jerr != nil {
		return false, fmt.Errorf("data-changed: decode contents: %w", jerr)
	}
	if doc.Encoding != "" && doc.Encoding != "base64" {
		return false, fmt.Errorf("data-changed: unsupported encoding %q (want base64)", doc.Encoding)
	}
	// GitHub returns base64 with newlines every 60 chars by default.
	decoded, err := base64.StdEncoding.DecodeString(stripNewlines(doc.Content))
	if err != nil {
		return false, fmt.Errorf("data-changed: base64 decode: %w", err)
	}

	existingHash, err := render.Hash(string(decoded))
	if err != nil {
		return false, fmt.Errorf("data-changed: hash existing: %w", err)
	}
	newHash, err := render.Hash(string(c.NewBody))
	if err != nil {
		return false, fmt.Errorf("data-changed: hash new: %w", err)
	}
	return existingHash == newHash, nil
}

func stripNewlines(s string) string {
	if !strings.ContainsAny(s, "\n\r") {
		return s
	}
	r := strings.NewReplacer("\n", "", "\r", "")
	return r.Replace(s)
}

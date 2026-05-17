package base

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	xerrors "github.com/mjun0812/github-metrics/internal/errors"
	"github.com/mjun0812/github-metrics/internal/githubapi"
	"github.com/mjun0812/github-metrics/internal/plugins"
)

// FetchRepo populates a *plugins.Repo for the M7 repository template
// flow. It runs once per request when the engine has selected the
// repository template and the user supplied a non-empty `repo` input.
//
// Sequence:
//
//  1. GraphQL `Repository(login, repo)` — owner / name / counts /
//     license / default branch / open-issue + open-PR totals.
//  2. REST `GET /repos/{owner}/{repo}/contributors?per_page=1` — the
//     count is extracted from the `Link` header so we don't paginate
//     the full contributor list when only the count is needed.
//  3. REST `GET /repos/{owner}/{repo}/commits?per_page=100&since=<30d>` —
//     count is the page length; "best-effort" (transient failures
//     keep the rest of the response intact + log a Warn).
//
// 404 is surfaced as a typed *xerrors.InputError on "repo" so the M6
// retry policy treats it as non-retryable (fail-fast).
func FetchRepo(ctx context.Context, login, repo string, rest *githubapi.REST, gql *githubapi.GraphQL) (*plugins.Repo, error) {
	if gql == nil {
		return nil, fmt.Errorf("base: FetchRepo: nil GraphQL client")
	}
	if login == "" || repo == "" {
		return nil, fmt.Errorf("base: FetchRepo: login and repo required (got %q/%q)", login, repo)
	}

	resp, err := gql.Repository(ctx, login, repo)
	if err != nil {
		return nil, fmt.Errorf("base: Repository(%s/%s): %w", login, repo, err)
	}
	if resp == nil || resp.Repository == nil {
		return nil, xerrors.NewInputError("repo",
			fmt.Errorf("repository %q not found", login+"/"+repo))
	}
	r := resp.Repository

	out := &plugins.Repo{
		Owner:         login,
		Name:          r.Name,
		Description:   derefString(r.Description),
		Stargazers:    r.StargazerCount,
		Forks:         r.ForkCount,
		IsArchived:    r.IsArchived,
		DefaultBranch: refName(r.DefaultBranchRef),
	}
	if r.Owner != nil {
		out.Owner = r.Owner.GetLogin()
		out.OwnerAvatar = r.Owner.GetAvatarUrl()
	}
	if r.PrimaryLanguage != nil {
		out.PrimaryLanguage = r.PrimaryLanguage.Name
		if r.PrimaryLanguage.Color != nil {
			out.PrimaryLanguageColor = *r.PrimaryLanguage.Color
		}
	}
	if r.LicenseInfo != nil {
		out.LicenseName = r.LicenseInfo.Name
	}
	if r.Issues != nil {
		out.Activity.OpenIssues = r.Issues.TotalCount
	}
	if r.PullRequests != nil {
		out.Activity.OpenPullRequests = r.PullRequests.TotalCount
	}

	// REST fallbacks — best-effort.
	if rest != nil {
		if n, lerr := fetchContributorsCount(ctx, rest, out.Owner, r.Name); lerr == nil {
			out.Contributors = n
		} else {
			slog.Warn("FetchRepo: contributors REST best-effort failure",
				"owner", out.Owner, "repo", r.Name, "err", lerr)
		}
		if n, lerr := fetchRecentCommitCount(ctx, rest, out.Owner, r.Name); lerr == nil {
			out.Activity.RecentCommits = n
		} else {
			slog.Warn("FetchRepo: commits REST best-effort failure",
				"owner", out.Owner, "repo", r.Name, "err", lerr)
		}
	}

	return out, nil
}

func refName(ref *githubapi.RepositoryRepositoryDefaultBranchRef) string {
	if ref == nil {
		return ""
	}
	return ref.Name
}

// fetchContributorsCount uses the GitHub REST trick of asking for
// `per_page=1&anon=true` and reading the total page count from the
// `Link: ...; rel="last"` header. Falls back to length of the returned
// JSON array when no Link header is present (single-page repos).
func fetchContributorsCount(ctx context.Context, rest *githubapi.REST, owner, repo string) (int, error) {
	path := fmt.Sprintf("/repos/%s/%s/contributors?per_page=1&anon=true", owner, repo)
	body, resp, err := rest.Get(ctx, path, nil)
	if err != nil {
		return 0, err
	}
	if resp == nil {
		return 0, fmt.Errorf("nil response")
	}
	if resp.StatusCode == http.StatusNotFound {
		return 0, nil // empty repo / no contributors
	}
	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("status %d", resp.StatusCode)
	}
	if link := resp.Header.Get("Link"); link != "" {
		if n := parseLinkLastPage(link); n > 0 {
			return n, nil
		}
	}
	// No Link header → single-page response; count entries.
	var rows []json.RawMessage
	if err := json.Unmarshal(body, &rows); err != nil {
		return 0, err
	}
	return len(rows), nil
}

// fetchRecentCommitCount returns the number of commits on the
// repository's default branch in the last 30 days, capped at 100 (one
// REST page). A 409 means the repo is empty.
func fetchRecentCommitCount(ctx context.Context, rest *githubapi.REST, owner, repo string) (int, error) {
	since := time.Now().UTC().AddDate(0, 0, -30).Format(time.RFC3339)
	path := fmt.Sprintf("/repos/%s/%s/commits?per_page=100&since=%s", owner, repo, since)
	body, resp, err := rest.Get(ctx, path, nil)
	if err != nil {
		return 0, err
	}
	if resp == nil {
		return 0, fmt.Errorf("nil response")
	}
	if resp.StatusCode == http.StatusConflict || resp.StatusCode == http.StatusNotFound {
		return 0, nil // empty repo
	}
	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("status %d", resp.StatusCode)
	}
	var rows []json.RawMessage
	if err := json.Unmarshal(body, &rows); err != nil {
		return 0, err
	}
	return len(rows), nil
}

// parseLinkLastPage scans a GitHub Link header for `; rel="last"` and
// extracts the `&page=N` value from that URL. Returns 0 when not found.
func parseLinkLastPage(link string) int {
	for _, part := range strings.Split(link, ",") {
		part = strings.TrimSpace(part)
		if !strings.Contains(part, `rel="last"`) {
			continue
		}
		open := strings.Index(part, "<")
		close := strings.Index(part, ">")
		if open < 0 || close < 0 || close <= open+1 {
			continue
		}
		u := part[open+1 : close]
		idx := strings.Index(u, "page=")
		if idx < 0 {
			continue
		}
		rest := u[idx+len("page="):]
		end := strings.IndexAny(rest, "&#")
		if end >= 0 {
			rest = rest[:end]
		}
		var n int
		_, _ = fmt.Sscanf(rest, "%d", &n)
		return n
	}
	return 0
}

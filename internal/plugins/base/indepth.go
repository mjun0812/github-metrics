package base

import (
	"context"
	"fmt"

	xerrors "github.com/mjun0812/github-metrics/internal/errors"
	"github.com/mjun0812/github-metrics/internal/githubapi"
	"github.com/mjun0812/github-metrics/internal/plugins"
	"github.com/mjun0812/github-metrics/internal/plugins/pluginutil"
)

// indepthInputKeys lists the plugin-input flags whose presence triggers
// base.runIndepth.
var indepthInputKeys = []string{
	"plugin_repositories_pinned",
	"plugin_isocalendar",
	"plugin_calendar",
	"plugin_habits",
}

// indepthTriggered reports whether at least one indepth-dependent
// plugin is enabled in pc.Inputs. plugin_notable_indepth additionally
// requires plugin_notable to be enabled to count.
func indepthTriggered(inputs map[string]any) bool {
	if inputs == nil {
		return false
	}
	for _, key := range indepthInputKeys {
		if pluginutil.Truthy(inputs[key]) {
			return true
		}
	}
	if pluginutil.Truthy(inputs["plugin_notable"]) && pluginutil.Truthy(inputs["plugin_notable_indepth"]) {
		return true
	}
	return false
}

// runIndepth issues the indepth GraphQL query and folds its results
// into pc.Data.Computed. On transient failure the base-standard fields
// stay populated (degraded path) and a *RetryableError is recorded on
// Data.Errors. The caller MUST already have populated Data.User /
// Data.Computed.RepositoryList via runUser + populateRepositories.
func runIndepth(ctx context.Context, pc *plugins.PluginContext, login string) error {
	reposBatch := repositoriesLimit(pc.Settings)
	if reposBatch <= 0 {
		reposBatch = 100
	}
	resp, err := pc.GraphQL.UserIndepth(ctx, login, nil, nil, reposBatch, nil)
	if err != nil {
		if isTransient(err) {
			pc.Data.AppendError(xerrors.NewRetryableError(
				fmt.Errorf("base: indepth(%q): %w", login, err),
			))
			return nil
		}
		return fmt.Errorf("base: indepth(%q): %w", login, err)
	}
	if resp == nil || resp.User == nil {
		return nil
	}
	if cc := resp.User.ContributionsCollection; cc != nil && cc.ContributionCalendar != nil {
		cal := cc.ContributionCalendar
		pc.Data.Computed.ContributionCalendar = &plugins.ContributionCalendar{
			TotalContributions: cal.TotalContributions,
			Weeks:              weeksFromIndepth(cal.Weeks),
		}
	}
	if resp.User.Repositories != nil {
		var totalCommits, totalIssues, totalPRs int
		for _, node := range resp.User.Repositories.Nodes {
			if node == nil {
				continue
			}
			totalCommits += commitsFromDefaultBranch(node.DefaultBranchRef)
			if node.Issues != nil {
				totalIssues += node.Issues.TotalCount
			}
			if node.PullRequests != nil {
				totalPRs += node.PullRequests.TotalCount
			}
		}
		pc.Data.Computed.TotalCommits = totalCommits
		pc.Data.Computed.TotalIssues = totalIssues
		pc.Data.Computed.TotalPullRequests = totalPRs
		// Mirror M1's per-aggregate fields so existing consumers see
		// the additional totals without subscribing to indepth-only
		// state. Repositories.Issues / PullRequests stayed zero in M1.
		pc.Data.Computed.Repositories.Issues = totalIssues
		pc.Data.Computed.Repositories.PullRequests = totalPRs
	}
	return nil
}

func weeksFromIndepth(weeks []*githubapi.UserIndepthUserContributionsCollectionContributionCalendarWeeksContributionCalendarWeek) []plugins.ContributionWeek {
	if len(weeks) == 0 {
		return nil
	}
	out := make([]plugins.ContributionWeek, 0, len(weeks))
	for _, w := range weeks {
		if w == nil {
			continue
		}
		days := make([]plugins.ContributionDay, 0, len(w.ContributionDays))
		for _, d := range w.ContributionDays {
			if d == nil {
				continue
			}
			days = append(days, plugins.ContributionDay{
				Date:              d.Date,
				ContributionCount: d.ContributionCount,
				Weekday:           d.Weekday,
				Color:             d.Color,
			})
		}
		out = append(out, plugins.ContributionWeek{
			FirstDay: w.FirstDay,
			Days:     days,
		})
	}
	return out
}

func commitsFromDefaultBranch(ref *githubapi.UserIndepthUserRepositoriesRepositoryConnectionNodesRepositoryDefaultBranchRef) int {
	if ref == nil || ref.Target == nil {
		return 0
	}
	// genqlient generates Target as **<GitObject interface>** (a
	// pointer to an interface variable) so type-assert via dereference.
	commit, ok := (*ref.Target).(*githubapi.UserIndepthUserRepositoriesRepositoryConnectionNodesRepositoryDefaultBranchRefTargetCommit)
	if !ok || commit == nil || commit.History == nil {
		return 0
	}
	return commit.History.TotalCount
}

package integration_test

import (

	// Side-effect imports register the M4 plugin partials with the
	// classic template's lookup table. Without these blank imports the
	// dispatcher in classic.Run would silently skip every plugin slug.
	_ "github.com/mjun0812/github-metrics/internal/plugins/achievements"
	_ "github.com/mjun0812/github-metrics/internal/plugins/activity"
	_ "github.com/mjun0812/github-metrics/internal/plugins/isocalendar"
	_ "github.com/mjun0812/github-metrics/internal/plugins/languages"
	_ "github.com/mjun0812/github-metrics/internal/plugins/repositories"
)

// p1UserOctocat mirrors userOctocat from foundation_test.go (kept
// local to avoid coupling the two suites' fixtures).
const p1UserOctocat = `{
	"data": {
		"user": {
			"databaseId": 12345,
			"id": "MDQ6VXNlcjEyMzQ1",
			"login": "octocat",
			"name": "The Octocat",
			"location": "San Francisco",
			"createdAt": "2008-01-14T04:33:35Z",
			"avatarUrl": "https://avatars.githubusercontent.com/u/12345?v=4"
		}
	}
}`

// p1UserRepositories provides 3 repositories with language byte
// breakdowns the languages plugin can aggregate. hasNextPage:false
// terminates the base paging loop after one call.
const p1UserRepositories = `{
	"data": {
		"user": {
			"repositories": {
				"totalCount": 3,
				"pageInfo": {"hasNextPage": false, "endCursor": null},
				"nodes": [
					{
						"databaseId": 1, "id": "R_a", "name": "alpha",
						"nameWithOwner": "octocat/alpha",
						"url": "https://github.com/octocat/alpha",
						"isPrivate": false, "isFork": false,
						"stargazerCount": 150, "forkCount": 20,
						"watchers": {"totalCount": 5},
						"primaryLanguage": {"name": "Go", "color": "#00ADD8"},
						"languages": {
							"totalCount": 2, "totalSize": 8000,
							"edges": [
								{"size": 6000, "node": {"name": "Go", "color": "#00ADD8"}},
								{"size": 2000, "node": {"name": "JavaScript", "color": "#f1e05a"}}
							]
						}
					},
					{
						"databaseId": 2, "id": "R_b", "name": "beta",
						"nameWithOwner": "octocat/beta",
						"url": "https://github.com/octocat/beta",
						"isPrivate": false, "isFork": false,
						"stargazerCount": 60, "forkCount": 5,
						"watchers": {"totalCount": 3},
						"primaryLanguage": {"name": "TypeScript", "color": "#3178c6"},
						"languages": {
							"totalCount": 1, "totalSize": 4500,
							"edges": [
								{"size": 4500, "node": {"name": "TypeScript", "color": "#3178c6"}}
							]
						}
					},
					{
						"databaseId": 3, "id": "R_c", "name": "gamma",
						"nameWithOwner": "octocat/gamma",
						"url": "https://github.com/octocat/gamma",
						"isPrivate": false, "isFork": false,
						"stargazerCount": 30, "forkCount": 2,
						"watchers": {"totalCount": 2},
						"primaryLanguage": {"name": "Go", "color": "#00ADD8"},
						"languages": {
							"totalCount": 1, "totalSize": 4000,
							"edges": [
								{"size": 4000, "node": {"name": "Go", "color": "#00ADD8"}}
							]
						}
					}
				]
			}
		}
	}
}`

// p1UserIsocalendar answers the isocalendar plugin's windowed
// contributionsCollection(from,to) query (#467). The fixture serves the
// same single week for every 4-week chunk the plugin requests (7 chunks
// for the default half-year window), which is fine for the DOM-marker
// assertions below.
const p1UserIsocalendar = `{
	"data": {
		"user": {
			"contributionsCollection": {
				"contributionCalendar": {
					"weeks": [
						{
							"firstDay": "2026-W18",
							"contributionDays": [
								{"date": "2026-05-04", "contributionCount": 1, "weekday": 0, "color": "#9be9a8"},
								{"date": "2026-05-05", "contributionCount": 2, "weekday": 1, "color": "#40c463"},
								{"date": "2026-05-06", "contributionCount": 0, "weekday": 2, "color": "#ebedf0"},
								{"date": "2026-05-07", "contributionCount": 3, "weekday": 3, "color": "#30a14e"},
								{"date": "2026-05-08", "contributionCount": 4, "weekday": 4, "color": "#216e39"},
								{"date": "2026-05-09", "contributionCount": 2, "weekday": 5, "color": "#40c463"},
								{"date": "2026-05-10", "contributionCount": 1, "weekday": 6, "color": "#9be9a8"}
							]
						}
					]
				}
			}
		}
	}
}`

// p1Inputs flips on all five P1 plugins via their truthy gate.
func p1Inputs() map[string]any {
	return map[string]any{
		"plugin_languages":    true,
		"plugin_activity":     true,
		"plugin_achievements": true,
		"plugin_repositories": true,
		"plugin_isocalendar":  true,
	}
}

package main

// Side-effect imports register the 21 adopted plugins + the classic
// template into their respective global registries. Without these
// imports, engine.Compute returns "template not found" / "base plugin
// missing". Keep this list in sync with internal/plugins/* + spec
// docs/design/15-selection-answer.md.

import (
	_ "github.com/mjun0812/github-metrics/internal/plugins/achievements"
	_ "github.com/mjun0812/github-metrics/internal/plugins/activity"
	_ "github.com/mjun0812/github-metrics/internal/plugins/base"
	_ "github.com/mjun0812/github-metrics/internal/plugins/calendar"
	_ "github.com/mjun0812/github-metrics/internal/plugins/contributors"
	_ "github.com/mjun0812/github-metrics/internal/plugins/core"
	_ "github.com/mjun0812/github-metrics/internal/plugins/habits"
	_ "github.com/mjun0812/github-metrics/internal/plugins/isocalendar"
	_ "github.com/mjun0812/github-metrics/internal/plugins/languages"
	_ "github.com/mjun0812/github-metrics/internal/plugins/notable"
	_ "github.com/mjun0812/github-metrics/internal/plugins/people"
	_ "github.com/mjun0812/github-metrics/internal/plugins/projects"
	_ "github.com/mjun0812/github-metrics/internal/plugins/reactions"
	_ "github.com/mjun0812/github-metrics/internal/plugins/repositories"
	_ "github.com/mjun0812/github-metrics/internal/plugins/sponsors"
	_ "github.com/mjun0812/github-metrics/internal/plugins/sponsorships"
	_ "github.com/mjun0812/github-metrics/internal/plugins/stargazers"
	_ "github.com/mjun0812/github-metrics/internal/plugins/starlists"
	_ "github.com/mjun0812/github-metrics/internal/plugins/stars"
	_ "github.com/mjun0812/github-metrics/internal/plugins/topics"
	_ "github.com/mjun0812/github-metrics/internal/plugins/traffic"

	_ "github.com/mjun0812/github-metrics/internal/templates/classic"
)

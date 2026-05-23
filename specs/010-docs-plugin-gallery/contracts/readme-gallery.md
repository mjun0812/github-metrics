# Contract: README hero + plugins-gallery section

**Date**: 2026-05-19 | **Plan**: [../plan.md](../plan.md) | **Related**: [../data-model.md](../data-model.md) E-003, E-004

Codifies the two README.md sections this feature inserts /
manages: the **hero block** near the top and the **plugins
gallery** in the middle. Both live between HTML-comment AUTOGEN
markers so the `gen-plugin-docs` tool can refresh them
idempotently.

## 1. Hero block (FR-001)

**Location**: between the project description (current line ~10)
and the "Highlights" section.

**Markers**:

```html
<!-- AUTOGEN_START: hero -->
... 2 SVG images side by side or stacked ...
<!-- AUTOGEN_END: hero -->
```

**Content** (markdown):

```markdown
<!-- AUTOGEN_START: hero -->
### Example output

`classic` template (user profile metrics):

![classic template sample](docs/examples/hero-classic.svg)

`repository` template (single-repo focus):

![repository template sample](docs/examples/hero-repository.svg)
<!-- AUTOGEN_END: hero -->
```

**Generator behavior**:

- The tool unconditionally writes this block (no human-authored
  zone inside it).
- The image references are fixed paths — the tool does not check
  whether the SVG file exists; that's the maintainer's
  responsibility via `make docs-samples`.

## 2. Plugins-gallery section (FR-002)

**Location**: between the existing "Plugins" section (currently
listing 19 plugins as a markdown table by tier) and the next
section ("Output formats").

**Markers**:

```html
<!-- AUTOGEN_START: plugins-gallery -->
... 3-column markdown table ...
<!-- AUTOGEN_END: plugins-gallery -->
```

**Content** (markdown — 7 rows of 3 columns, last row has 1 empty
cell):

```markdown
<!-- AUTOGEN_START: plugins-gallery -->
| | | |
|:---:|:---:|:---:|
| [![achievements](docs/examples/plugin-achievements.svg)](docs/plugins/achievements.md) | [![activity](docs/examples/plugin-activity.svg)](docs/plugins/activity.md) | [![calendar](docs/examples/plugin-calendar.svg)](docs/plugins/calendar.md) |
| [`achievements`](docs/plugins/achievements.md) | [`activity`](docs/plugins/activity.md) | [`calendar`](docs/plugins/calendar.md) |
| [![contributors](docs/examples/plugin-contributors.svg)](docs/plugins/contributors.md) | [![habits](docs/examples/plugin-habits.svg)](docs/plugins/habits.md) | [![isocalendar](docs/examples/plugin-isocalendar.svg)](docs/plugins/isocalendar.md) |
| [`contributors`](docs/plugins/contributors.md) | [`habits`](docs/plugins/habits.md) | [`isocalendar`](docs/plugins/isocalendar.md) |
| [![languages](docs/examples/plugin-languages.svg)](docs/plugins/languages.md) | [![notable](docs/examples/plugin-notable.svg)](docs/plugins/notable.md) | [![people](docs/examples/plugin-people.svg)](docs/plugins/people.md) |
| [`languages`](docs/plugins/languages.md) | [`notable`](docs/plugins/notable.md) | [`people`](docs/plugins/people.md) |
| [![projects](docs/examples/plugin-projects.svg)](docs/plugins/projects.md) | [![reactions](docs/examples/plugin-reactions.svg)](docs/plugins/reactions.md) | [![repositories](docs/examples/plugin-repositories.svg)](docs/plugins/repositories.md) |
| [`projects`](docs/plugins/projects.md) | [`reactions`](docs/plugins/reactions.md) | [`repositories`](docs/plugins/repositories.md) |
| [![sponsors](docs/examples/plugin-sponsors.svg)](docs/plugins/sponsors.md) | [![sponsorships](docs/examples/plugin-sponsorships.svg)](docs/plugins/sponsorships.md) | [![stargazers](docs/examples/plugin-stargazers.svg)](docs/plugins/stargazers.md) |
| [`sponsors`](docs/plugins/sponsors.md) | [`sponsorships`](docs/plugins/sponsorships.md) | [`stargazers`](docs/plugins/stargazers.md) |
| [![starlists](docs/examples/plugin-starlists.svg)](docs/plugins/starlists.md) | [![stars](docs/examples/plugin-stars.svg)](docs/plugins/stars.md) | [![topics](docs/examples/plugin-topics.svg)](docs/plugins/topics.md) |
| [`starlists`](docs/plugins/starlists.md) | [`stars`](docs/plugins/stars.md) | [`topics`](docs/plugins/topics.md) |
| [![traffic](docs/examples/plugin-traffic.svg)](docs/plugins/traffic.md) | | |
| [`traffic`](docs/plugins/traffic.md) | | |
<!-- AUTOGEN_END: plugins-gallery -->
```

**Ordering**: alphabetical by slug. The generator sorts the plugin
list before emitting so re-runs are byte-stable.

**Generator behavior**:

- Reads the 19 adopted plugin slugs from a hardcoded list (mirror
  of `tests/compliance/compliance_test.go::adoptedM4Plugins`, less
  `base` / `core`). The hardcoded list is asserted equal to the
  compliance test's list by the tool's unit test, preventing
  drift.
- Emits the table with thumbnail + caption rows in the order
  shown.
- Inserts an empty trailing cell to keep the table rectangular
  (19 % 3 == 1).

## 3. Idempotency across regenerations

A `make docs` re-run (which invokes `gen-plugin-docs`) re-emits
both blocks. The HTML-comment markers ensure that:

- Content **inside** the markers is fully overwritten.
- Content **outside** the markers (the existing README prose) is
  byte-preserved.

The tool's unit test runs `gen-plugin-docs` twice on a fixed
input and asserts the second run produces zero diff.

## 4. First-time generation

If README.md does NOT yet contain the AUTOGEN markers (first run
of this feature), the tool **inserts them** at well-known
anchors:

- Hero block: immediately after the project description (one
  paragraph after the badges block).
- Plugins gallery: at the existing "Plugins" section heading,
  replacing the current tier-table list with the gallery.

The exact insertion points are matched by regex against the
current README structure (e.g., after the first `## Highlights`
heading for the hero, replacing the table under the first
`## Plugins` heading for the gallery). The tool aborts with a
clear error if the anchors are missing — the maintainer must add
the markers manually in that case.

## 5. Inside the gallery markers, what about images that haven't been generated yet?

The gallery references `docs/examples/plugin-<slug>.svg` for
every adopted plugin. If `make docs-samples` has not run, those
files don't exist, and github.com renders broken-image icons in
the gallery.

**Resolution**: the recommended maintainer flow per `quickstart.md`
is:

1. Run `make docs-samples` first (generates SVGs).
2. Run `make docs` (refreshes the markdown — gallery refs are now
   valid).
3. Commit both `docs/examples/*.svg` and `README.md` /
   `docs/plugins/*.md` together.

The umbrella `make docs-examples` runs both steps in order.

The new compliance test (`TestCompliance_DocsPluginPagesMatchAdoptedSet`)
asserts the doc-page set; a separate optional lint can be added
later to assert sample-image existence, but that's out of scope
for v1 of this feature (the gallery is the human-visible signal).

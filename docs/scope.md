# Adopted Scope and Decisions

This project is a Go port of only a **subset** of the [`lowlighter/metrics`](https://github.com/lowlighter/metrics) (Node.js / EJS). This document records the finalized decisions on "what was adopted and what was not adopted." CLAUDE.md references this document as the source of truth for adoption decisions.

Before starting work on a new feature, always check this document to confirm whether the target is within the adopted scope. The presence of unadopted features is detected in CI by `tests/compliance/compliance_test.go` (see [§4](#4-compliance-gate)).

---

## 1. Runtime forms

- Only two forms are adopted: **GitHub Action** and **CLI**.
- **The Web instance is not adopted** ([§3.1](#31-web-instance-formerly-m5)).

A single binary (`cmd/metrics-cli`) serves as both the Action and the CLI.

## 2. Adopted plugins

Shared data fetching is consolidated in the lazy / memoized Provider in `internal/dataprovider`, and each plugin reads user / repository data from there.

### 2.1 Foundation plugins (always present, not part of the gallery)

| slug   | role                                                                                                             |
| ------ | ---------------------------------------------------------------------------------------------------------------- |
| `core` | Injects configuration (`config_*`) and runs the parallel execution runner for each plugin                        |
| `base` | Summary panel for activity+community / repositories. An opt-in plugin that reads only from `dataprovider` (#625) |

> The former `base` plugin (which also handled shared data fetching) was dismantled in #605; data fetching was split out to `dataprovider`, the identity card to the `header` plugin (#602), and the summary panel to the current `base`, reintroduced in #625.

### 2.2 User-visible plugins (19)

The 19 plugins corresponding to the README gallery and `docs/plugins/*.md`:

`header`, `languages` (with `recent` / `indepth` submodes), `activity`, `achievements`, `repositories`, `isocalendar`, `calendar`, `habits`, `stars`, `people`, `notable`, `contributors`, `reactions`, `sponsors`, `sponsorships`, `stargazers`, `traffic`, `topics`, `starlists`

`header` is an opt-in plugin that renders the identity card (avatar / name / stats). It can be individually included / excluded when embedding the SVG (#602).

For each plugin's inputs, outputs, and required scopes, see the auto-generated pages under [`docs/plugins/`](plugins/). For the list of data fetching methods (REST / GraphQL / HTML scraping / local computation), see [`github-api-data-sources.md`](github-api-data-sources.md).

> The `projects` plugin was adopted once but was removed in #670 as unused.

## 3. Features not adopted (implementation prohibited)

### 3.1 Web instance (formerly M5)

The full set of features that expose metrics over HTTP, including an HTTP server / OAuth / Insights HTML / rate limiter / control endpoint.

**Reason for non-adoption**: judged to have high operational cost and to be unnecessary for README badge usage (the Action and CLI satisfy the purpose).

Adding `terminal` / `markdown` etc. to `internal/templates/` is also prohibited, as it would conflict with this decision (gated by `TestCompliance_M7_TemplateInvariant`).

### 3.2 Social / external API plugins (formerly M8)

The group of plugins that require the user to prepare tokens / keys for external services. None of these are adopted:

`wakatime`, `pagespeed`, `posts`, `rss`, `stackoverflow`, `leetcode`, `anilist`, `music`, `steam`, `tweets`,
`crypto`, `nightscout`, `stock`, `chess`, `splatoon`, `fortune`, `poopmap`, `screenshot`, `16personalities`

**Reason for non-adoption**: increases external service dependencies, which is not worth the maintenance cost, and strays from the project's original purpose of handling GitHub data.

### 3.3 Other non-adopted items

| Feature                                                                        | Reason for non-adoption                                                       |
| ------------------------------------------------------------------------------ | ----------------------------------------------------------------------------- |
| `lines` / `gists` / `followup` / `discussions` / `skyline` / `support` plugins | Low necessity, heavy API rate consumption, or deprecated `lowlighter/metrics` |
| `code` plugin                                                                  | Risk of private code leakage                                                  |
| Markdown / Markdown-PDF output                                                 | Low demand, and PDF generation would reintroduce a browser dependency         |
| Community plugin / dynamic template fetching                                   | Safety / maintenance cost                                                     |
| SVGO-equivalent SVG optimization                                               | CSS purge + XML formatting is sufficient                                      |

The only adopted output formats are **SVG / PNG / JPEG / JSON**. See [`rendering.md`](rendering.md) for details.

## 4. Compliance gate

The following automated tests enforce the adopted scope in CI. A failure signals that an unadopted feature has crept in and must be addressed immediately:

- `TestCompliance_M4_AdoptedPlugins`: the set of directories under `internal/plugins/` exactly matches the list of adopted plugins.
- `TestNoUnadoptedPluginReference`: no unadopted plugin slug is present in production code (excluding comments).
- `TestCompliance_M7_TemplateInvariant`: `internal/templates/` contains only `classic` / `repository`.
- `TestCompliance_DocsPluginPagesMatchAdoptedSet`: `docs/plugins/*.md` matches the set of 19 user-visible plugins.
- `TestNoRemovedSentinelComments`: `// removed:` style removal-history comments must not be written.

## 5. Completion history

| Milestone | Content                                                                                                                                                              | Release             |
| --------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ------------------- |
| M1-M10    | Go port MVP from `lowlighter/metrics` (foundation → classic / JSON → rendering → 19 plugins → Action/CLI → repository template → test infrastructure → distribution) | v1.0.0 (2026-05-18) |
| #409      | Full migration to browser-free rendering with native SVG + resvg, eliminating chromedp / Chromium                                                                    | v4.0.0 (2026-07-09) |

The original order of the porting phases was `M1 → M2 → M3 → M4 → M6 → M7 → M9 → M10` (M5 / M8 are missing numbers because they were not adopted). The decision log for #409 is recorded in issue #409 and sub-issues #682-#695.

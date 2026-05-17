# Feature Specification: M6 — GitHub Action / CLI

**Feature Branch**: `005-m6-action-cli`

**Created**: 2026-05-17

**Status**: Draft

**Input**: User description: "M6: GitHub Action / CLI — INPUT_* パーサ + presets 統合 + token validation + rate quota check + render + retry + committer (commit / pull-request / data-changed) + CLI 専用フラグ。docs/design/16-tasks-mvp.md Phase M6 (T-105-T-117) を参照。"

## Clarifications

### Session 2026-05-17

- Q: Docker image registry / tag scheme は? → A: GitHub Container Registry (ghcr.io) + semver タグ (`vX.Y.Z`, `latest`, `:sha-<short>`)。upstream / Action エコシステムの標準形
- Q: CLI バイナリの配布手段は? → A: GitHub Releases に pre-built binary (linux/macos × amd64/arm64) を添付 + `go install github.com/mjun0812/github-metrics/cmd/metrics-action@latest` をサポート。Docker は always available
- Q: Retry 対象とする error class は? → A: `*xerrors.RetryableError` でラップされた error のみリトライ (= 5xx + timeout + 一部の rate-limit)。それ以外 (4xx, auth, scope, token, config) は即 fail (exit 1)
- Q: 未対応 `output_action` 値 (gist / markdown-*) の振る舞いは? → A: Fail fast (exit 1) + 明示的 migration message。silent fallback は事故源なので config-error として扱う
- Q: 操作者向けログ / バナーの言語は? → A: 英語固定。upstream banner / Action エコシステムの grep 互換性最優先。日本語環境のユーザにも汎用的 (operator-facing artifact は本プロジェクトの「会話日本語」ポリシーの対象外)

## User Scenarios & Testing *(mandatory)*

### User Story 1 — README に metrics SVG を貼りたい個人開発者 (Priority: P1) 🎯 MVP

**Concrete user**: 自分の GitHub プロフィール README に metrics SVG を毎日自動更新で貼りたい開発者。`lowlighter/metrics@latest` を `mjun0812/github-metrics@latest` (本リポジトリ) に差し替え、既存の `.github/workflows/metrics.yml` をそのまま使う。日次 schedule で workflow が走り、生成された `github-metrics.svg` がリポジトリにコミットされ、README の `<img src="github-metrics.svg">` が更新される。

**Why this priority**: M1-M4 で構築した `engine.Compute` を「実際に GitHub 上で動かす」唯一の MVP パスがこの Action フロー。これが動かない限り本プロジェクトは「ユーザにとっての価値がゼロ」 — README にメトリクスを乗せたいユーザにとっての出口がない。upstream `lowlighter/metrics` の drop-in 代替を提供することが M6 の存在理由そのもの。

**Independent Test**: GitHub Actions ランナー相当の docker 環境で `mjun0812/github-metrics` action を `with: { user: octocat, token: <PAT>, template: classic }` で実行。`/renders/github-metrics.svg` が生成され、`output_action=commit` でリポジトリにコミットされる。`output_action=none` (dryrun) なら標準出力 or workspace に SVG が残るだけでコミットは走らない。

**Acceptance Scenarios**:

1. **Given** action.yml で `user: octocat`, `token: <valid-PAT>`, `template: classic`, `output_action: commit` を指定、**When** action が起動する、**Then** `engine.Compute` が呼ばれ、生成された SVG が `/renders/github-metrics.svg` に書き出され、`PUT /repos/.../contents/github-metrics.svg` でリポジトリの `main` (デフォルト branch) に commit される
2. **Given** action.yml で `dryrun: yes`, `output_action: commit` を指定、**When** action が起動する、**Then** SVG はファイル出力されるが commit は走らず、exit 0 で終了する
3. **Given** トリガコミットのメッセージが `[Skip GitHub Action]` を含む、**When** action が起動する、**Then** 即座に skip ログを出して exit 0 (engine.Compute は呼ばない)
4. **Given** `mjun0812/github-metrics@latest` を `uses` で参照した workflow、**When** GitHub Actions が `docker run ghcr.io/mjun0812/github-metrics:latest` 相当を起動する、**Then** コンテナ内の `metrics-action` binary が `INPUT_USER` / `INPUT_TOKEN` / `INPUT_TEMPLATE` 環境変数を読んで動作する
5. **Given** action が `INPUTS={"user":"octocat","template":"classic"}` JSON 形式で入力を受け取る、**When** バイナリが入力を解釈する、**Then** `INPUTS` の値が優先され、`INPUT_<UPPER>` 環境変数はフォールバックとして使われる

---

### User Story 2 — git history を最小限に保ちたい運用者 (Priority: P1)

**Concrete user**: 毎日 cron 走行する metrics action で、SVG が前回と byte 単位で変わっていない場合に commit を作りたくないユーザ。「データが変わったときだけ commit」の挙動 (`output_condition=data-changed`) を期待する。

**Why this priority**: ホスティングしている公開リポジトリで毎日空コミットが積まれると、変更履歴のノイズになり commit log や README diff ビューが汚れる。`data-changed` はリポジトリ運用者にとって P1 級の必須機能で、M3 で landing 済みの `render.Hash` を Action 経路から呼べる形で wire するだけ。

**Independent Test**: モックされた GitHub backend で 2 回連続 action を走らせる。1 回目は新規 commit、2 回目は同一の Compute 結果が返るがリポジトリに `output_condition=data-changed` が設定されているため commit はスキップされる。

**Acceptance Scenarios**:

1. **Given** リポジトリに `github-metrics.svg` が無い、**When** `output_condition=data-changed` 付きで action が走る、**Then** 通常通り Compute → commit される (条件「データが変わった」が初回は trivially true)
2. **Given** `github-metrics.svg` が既に存在し、今回の Compute 結果が前回と byte 一致 (Hash 一致)、**When** action が走る、**Then** 既存 SVG を `GET /repos/.../contents/...` で取得 → `render.Hash` で比較 → 一致と判定 → `committer.Commit=false` で commit スキップ → exit 0
3. **Given** 今回の Compute 結果が前回と異なる、**When** action が走る、**Then** 新規 SVG で上書き commit が走る
4. **Given** `output_condition=always` (デフォルト)、**When** action が走る、**Then** Hash 比較はスキップされ常に commit される (regression-safe)

---

### User Story 3 — レビューフローを通したい組織運用者 (Priority: P2)

**Concrete user**: 個人 README ではなく組織管理下のリポジトリで、`metrics.svg` の更新も他の変更と同じく PR レビューを通したい運用者。`output_action=pull-request` で metrics を専用 branch (`metrics-run-<runId>`) にコミットし、main へ PR を作る。承認後 merge (auto-merge 含む) する運用。

**Why this priority**: P1 (commit 直 push) で多くのユーザは満足。組織や OSS で「protected main」運用しているリポジトリ向けの追加 surface なので P2。実装は P1 の committer 構造体に PR 作成パスを足すだけ。

**Independent Test**: モック GitHub backend で `output_action=pull-request` を指定 → action 完走時に `metrics-run-<runId>` branch が作成され、`POST /repos/.../pulls` で PR が立つ。`output_action=pull-request-merge` なら `PUT /repos/.../pulls/{n}/merge` まで自動で走る。

**Acceptance Scenarios**:

1. **Given** `output_action=pull-request` 設定、**When** action が走る、**Then** `metrics-run-<runId>` branch 作成 → SVG commit → `POST /repos/.../pulls` で PR 作成 → PR URL がログに出力される
2. **Given** `output_action=pull-request-merge`、**When** PR 作成成功 → `mergeable=true`、**Then** `PUT /repos/.../pulls/{n}/merge` が呼ばれて auto-merge される
3. **Given** `output_action=pull-request-squash`、**When** 同上、**Then** `PUT /repos/.../pulls/{n}/merge?merge_method=squash` で squash merge される
4. **Given** PR 作成が失敗 (例: 同名 branch が既に open PR を持つ)、**When** action が走る、**Then** warning ログを出して action は exit 0 (失敗で workflow を止めない、best-effort)

---

### User Story 4 — ローカルでデバッグしたい開発者 (Priority: P2)

**Concrete user**: action.yml を編集中で「この入力を変えると何が出るか」を docker run せず素早く確認したい開発者。`metrics-action` binary を `metrics-action --user octocat --template classic --plugin plugin_languages=true --output svg --dryrun` のように CLI として直接叩く。

**Why this priority**: P1/P2 (GitHub Actions 経由) で本番運用は成立する。CLI モードは開発者の生産性 (反復速度) 向上が目的で、P2 級の付加機能。CI で `action.yml` を変えるたびに workflow を走らせるより、ローカル CLI で 1 秒で結果を見られる方が良い。

**Independent Test**: シェル上で `metrics-action --user octocat --template classic --output svg --dryrun` を実行 → 標準出力に SVG が流れる、または `--filename out.svg` でファイル出力される。`--config <path>.yaml` で YAML config を読み込めば action.yml 相当の入力を完全に再現できる。

**Acceptance Scenarios**:

1. **Given** ローカルで `metrics-action --user octocat --template classic --output svg --dryrun` を実行、**When** binary が起動する、**Then** mocked or real Compute (token があれば real) を実行し、SVG を `/renders/github-metrics.svg` に書く (または `--filename -` で stdout)
2. **Given** `--config metrics-inputs.yaml` で YAML config を指定、**When** binary が起動する、**Then** YAML が `INPUTS` JSON 相当に変換されてパイプラインに流される (action.yml 形式と完全互換)
3. **Given** `--plugin plugin_languages=true --plugin plugin_languages_limit=5`、**When** binary が起動する、**Then** 各 `--plugin` フラグが `plugin_<key>` 入力として engine に渡る
4. **Given** `--token-env GITHUB_PAT`、**When** binary が起動する、**Then** `GITHUB_PAT` 環境変数の値を token として使う (シェル history に token が残らない)

---

### Edge Cases

- **invalid token (`github_pat_*`)**: GitHub の fine-grained PAT は metrics 用途で必要な scope を満たさない場合があり、設計上は拒否してエラー終了
- **token に必要 scope が無い**: 起動時 `HEAD /` の `X-OAuth-Scopes` で確認、不足分があれば warning ログ + 関連 plugin は skipped で SVG を返す
- **quota 不足 (`quota_required_rest=1000` だが remaining=500)**: 起動時 `/rate_limit` で確認、不足なら exit 0 (skipped log を出して workflow は失敗扱いにしない)
- **Compute が transient 5xx で失敗**: `retries=3, retries_delay=300` で自動リトライ。3 回失敗で最後の error を持って exit 1
- **commit 先 branch が存在しない**: 起動時に `PUT /git/refs` で head branch を作成 (デフォルト branch から fork)
- **`output_condition=data-changed` で前回ファイルが取得できない (削除されている等)**: 「変わった」と判定して通常 commit
- **`docker run` で `INPUTS` も `INPUT_USER` も両方未設定**: 起動時に required input 不足 error で exit 1
- **`use_mocked_data=true`**: GitHub API を一切叩かず M2 mock fixture から SVG を返す (CI / ローカルデモ用)
- **新リリース通知 (`notice_releases=true`)**: 起動時に `GET /repos/<org>/<repo>/releases/latest` を叩いて自分より新しい version があれば 1 行 info ログ
- **`--dryrun` + `output_action=pull-request`**: dryrun は output_action よりも優先、PR は作られない
- **Action コンテキスト外での起動 (CLI モード)**: `GITHUB_REPOSITORY` 等が未設定の場合、commit / PR モードを選んでいたら error で exit 1
- **`output_action` に未サポート値 (`gist`, `markdown commit` 等) を指定**: 即 fail (exit 1)、サポート値一覧を含む明示的 migration message を出す。silent fallback は禁止 (FR-015b)

## Requirements *(mandatory)*

### Functional Requirements

**FR-001** (US1): System MUST provide a `metrics-action` binary entrypoint that reads inputs from `INPUTS` JSON env var (preferred) and `INPUT_<UPPER>` env vars (fallback), translates them into `engine.Compute` Request fields, runs Compute, writes the output to `/renders/<filename>` (or the configured location), and optionally performs the configured `output_action`.

**FR-002** (US1): System MUST honor the `[Skip GitHub Action]` and `Auto-generated metrics for run #N` commit-message-based skip conditions by detecting them in `GITHUB_EVENT_PATH` and exiting 0 without invoking Compute.

**FR-003** (US1): System MUST emit a startup banner **in English** (matching upstream Node format) including version, configured template, configured plugins (sorted, with deprecation flags), and runtime context (action vs CLI), so existing operator dashboards / grep patterns continue to work. All operator-facing artifacts (banner, info / warning / error logs, FR-021 release-notice, FR-015b migration message) MUST also be English. (The project's Japanese-language policy applies to spec / conversation / commit messages, not runtime artifacts.)

**FR-004** (US1): System MUST validate the supplied token: reject `github_pat_*` fine-grained tokens with a clear error message; warn (not error) on missing scopes detected via `HEAD /` X-OAuth-Scopes; verify `/rate_limit` shows enough quota for the requested plugins (per-plugin `quota_required_rest|graphql|search` annotation), exit 0 with a skipped log when quota is insufficient.

**FR-005** (US1): System MUST package and distribute as a Docker image published to **GitHub Container Registry** at `ghcr.io/mjun0812/github-metrics` with the following tag set: (a) `vX.Y.Z` for each release (Semantic Versioning), (b) `latest` pinned to the newest stable release, (c) `sha-<short>` per merged commit for reproducibility. The `action.yml` references this image so existing `lowlighter/metrics@latest` workflow files swap by `uses:` line only.

**FR-006** (US1): System MUST support `presets` (`config_presets`) by loading the referenced preset YAML and merging its `q` map into the inputs before Compute, matching upstream's preset semantics so that presets vendored in user repositories continue to work.

**FR-007** (US1): System MUST retry `engine.Compute` transparently per the `retries` / `retries_delay` inputs (defaults: 3 retries, 300 ms delay) and only exit with the final error after exhaustion. **Retry-eligible errors** are limited to those wrapped in `*xerrors.RetryableError` (the M4-established marker covering 5xx, network timeout, and the documented transient rate-limit retry surface). Non-retryable error classes — 4xx (login not found, repo not visible), token validation (`github_pat_*` reject, missing scope), config errors, and any error NOT wrapped as RetryableError — MUST fail fast (exit 1) with no retry attempts.

**FR-008** (US1): System MUST resolve the `_filename` wildcard `*` (e.g., `github-metrics.*`) into the appropriate extension based on `config.output` (`.svg` / `.png` / `.jpeg` / `.json` / `.markdown`).

**FR-009** (US1): System MUST honor `dryrun=yes` by skipping all `output_action` side effects (no commit, no PR, no GitHub state mutation) while still writing the output file to disk for inspection.

**FR-010** (US1): System MUST write Action outputs (`metrics_url`, `metrics_sha`) by appending `key=value` to `$GITHUB_OUTPUT`, matching `@actions/core` v2 contract so downstream workflow steps can `${{ steps.metrics.outputs.metrics_url }}`.

**FR-011** (US1): System MUST honor `use_mocked_data=true` by routing through the M2 mock fixture path, so contributor environments without GITHUB tokens can run the action locally for demos / CI checks.

**FR-012** (US2): System MUST implement `output_condition=data-changed`: when the destination file already exists in the target branch, fetch its current content via `GET /repos/.../contents/<filename>`, hash it via `render.Hash`, compare to the freshly rendered SVG hash, and skip the commit when bytes are equal.

**FR-013** (US2): System MUST default `output_condition` to `always` so the data-changed gate is opt-in (no surprise behavior change vs upstream Node default).

**FR-014** (US3): System MUST implement `output_action=commit` (default): create the head branch via `PUT /git/refs` if missing, then `PUT /repos/.../contents/<filename>` to commit the new file.

**FR-015** (US3): System MUST implement `output_action=pull-request`, `pull-request-merge`, `pull-request-squash`, `pull-request-rebase`: create the run-scoped branch `metrics-run-<runId>`, commit, `POST /repos/.../pulls`, then for `*-merge` / `*-squash` / `*-rebase` variants follow up with `PUT /repos/.../pulls/{n}/merge?merge_method=<method>` once the PR reports `mergeable=true`.

**FR-015b** (US3): System MUST validate `output_action` against the supported value set (`none`, `commit`, `pull-request`, `pull-request-merge`, `pull-request-squash`, `pull-request-rebase`) **before** invoking Compute. Unsupported values (notably `gist`, `markdown commit`, `markdown pull-request` from upstream's broader set) MUST cause an **immediate fail-fast exit 1** with an error message that (a) names the unsupported value, (b) lists the supported set, (c) points to a migration note. Silent fallback to `commit` or `none` MUST NOT happen — silent migration would mask "I expected my SVG to be committed to a gist" mistakes.

**FR-016** (US3): System MUST treat all committer failures as warning-level (logged, but action exits 0) — running the action MUST never block the user's main workflow when the only failure is a metrics commit / PR issue.

**FR-017** (US4): System MUST provide a CLI entrypoint that accepts standard flags (`--config`, `--user`, `--template`, `--token`, `--token-env`, `--plugin key=value` repeatable, `--output`, `--filename`, `--dryrun`) and routes them through the same input-parsing pipeline as the Action mode, so CLI invocations produce byte-identical output to the equivalent action.yml run.

**FR-018** (US4): System MUST accept `--token-env <NAME>` to read the token from `os.Getenv(NAME)` instead of `--token <value>`, so shell history / process list does not leak the PAT.

**FR-019** (US4): System MUST accept `--config <path>.yaml` and load a YAML file whose schema mirrors the action.yml inputs (`user: octocat`, `template: classic`, `plugins: { languages: true, ... }`), so users can keep their action.yml inputs and a local `metrics-inputs.yaml` in sync without manual translation.

**FR-020** (US4): System MUST support `--dryrun` and `--filename -` (stdout) so the CLI can be piped (`metrics-action --user octocat --output svg --dryrun --filename - | xmllint --pretty`).

**FR-020b** (US4): System MUST publish pre-built `metrics-action` binaries as GitHub Releases assets covering at minimum **linux/amd64**, **linux/arm64**, **darwin/amd64**, **darwin/arm64**. The same release tag carries both the binaries and the matching Docker image (FR-005), so a user can pin the same `vX.Y.Z` for either path. The Go module path (`github.com/mjun0812/github-metrics/cmd/metrics-action`) MUST also work for `go install ...@latest` / `@vX.Y.Z` so contributors who already have a Go toolchain can install in one command.

**FR-021** (cross-cutting): System MUST emit a notice log when `notice_releases=true` and `GET /repos/<org>/<repo>/releases/latest` returns a newer version than the running binary, so users see the upgrade hint in their workflow logs.

**FR-022** (cross-cutting): System MUST mask token values in all log output (including error wrapping). Tokens are matched by both the explicit `token` input slot and the `type: token` metadata field on any plugin input.

**FR-023** (scope discipline, 原則 III): System MUST NOT introduce any new plugin, template, or feature beyond what M1-M4 + this M6 spec ships. The 21 採用 plugin set from M4 remains frozen.

### Key Entities

- **Action invocation context**: per-run state. Captures the resolved inputs (after JSON / env var / preset merging), the resolved token + verified scopes / quota, the GitHub event payload (skip-message detection), the target output filename, and the chosen output_action.
- **Committer**: per-run helper holding the resolved branch / message / author / sign-off configuration, plus the in-progress commit / PR state. Implements the commit / pull-request output paths.
- **Hash comparator**: thin wrapper over `render.Hash` used by `output_condition=data-changed`. Reads current file from GitHub Contents API, hashes both sides, returns bool.
- **CLI flag set**: per-process state populated from `argparse`-like flag parsing. Convertible to the same input map the Action path uses, so the downstream pipeline is shared.
- **Preset bundle**: optional, loaded from disk per `config_presets`. Merged into the input map.
- **Retry policy**: per-run wrapper over `engine.Compute` invocation, parameterized by `retries` / `retries_delay`.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001** (US1): A clean checkout MUST be able to build the `metrics-action` binary, ship it inside a Docker image whose entrypoint is the binary, and run that image via `docker run -e INPUT_USER=octocat -e INPUT_TOKEN=<PAT> -e INPUT_TEMPLATE=classic ghcr.io/mjun0812/github-metrics:<tag>` to produce a valid SVG at `/renders/github-metrics.svg` — end-to-end within 30 seconds against mocked deps (under 60 s against real GitHub).

- **SC-002** (US1): An existing workflow file that previously used `uses: lowlighter/metrics@latest` MUST work after changing the `uses:` line to `uses: mjun0812/github-metrics@latest`, with no other YAML changes required, for the M4 採用 21 plugin set.

- **SC-003** (US1): The Action's startup banner MUST include the version string, sorted plugin list, runtime mode (`action` vs `cli`), and the resolved template name — verified by integration-test snapshot diff against the upstream Node banner format (line-by-line semantic comparison; trailing whitespace / exact emoji are out of scope).

- **SC-004** (US1): The skip-message detection (`[Skip GitHub Action]` / `Auto-generated metrics for run #N`) MUST trigger correct skipping in 2/2 deterministic test cases, with action exiting code 0 and no engine invocation.

- **SC-005** (US2): `output_condition=data-changed` end-to-end test MUST verify: (a) first run commits when no existing file, (b) second run with identical Compute result skips commit, (c) third run with different Compute result resumes committing. Mocked GitHub backend.

- **SC-006** (US3): `output_action=pull-request-merge` end-to-end test MUST verify: branch creation → SVG commit → PR creation → PR returns `mergeable=true` → auto-merge succeeds, all against a mocked GitHub backend. PR cleanup is the merge-target's responsibility (not in scope).

- **SC-007** (US3): On `output_action` failure (PR conflict, branch protection blocking merge, etc.) action MUST exit 0 with a warning log; the failure MUST NOT mark the GitHub workflow run as failed (verified by 4 deterministic failure-injection test cases). **Separately**, when `output_action` is set to an unsupported value (`gist`, `markdown commit`, `markdown pull-request`, or any non-listed string), the action MUST exit 1 with a migration message — verified by 1 test case per unsupported value covering at minimum `gist` and `markdown commit`.

- **SC-008** (US4): The CLI binary MUST run on macOS / Linux without docker (no chromium-required test cases skip cleanly when chromium is absent). `metrics-action --user octocat --template classic --output svg --dryrun --filename -` MUST emit a valid SVG to stdout within 30 seconds (mocked deps).

- **SC-009** (US4): CLI input parsing MUST be byte-identical to the equivalent action.yml input — verified by 5 paired test cases where a `.yaml` config (CLI) and an `action.yml`-style env-var fixture (Action) produce the same `engine.Request`.

- **SC-010** (cross-cutting): All 7 retry/quota/token-validation paths MUST be covered by deterministic test cases: `token rejected (github_pat_*)`, `token missing`, `quota insufficient → skipped`, `scope warning`, `Compute RetryableError → retry success`, `Compute RetryableError → retry exhausted (exit 1)`, **`Compute non-Retryable error → fail fast (no retry, exit 1)`** — the last case verifies that 4xx / auth / config errors bypass the retry loop.

- **SC-011** (cross-cutting): Token masking MUST be verified across 4 log paths (engine error wrap, retry log, banner, slog field) — no test case may produce log output containing the raw `<PAT>` literal supplied by the test.

- **SC-012** (constitution 原則 III): The 19 unadopted upstream plugins MUST NOT appear in any new M6 source file, asserted by extending `tests/compliance/compliance_test.go::TestNoUnadoptedPluginReference` to cover `internal/action/` and `cmd/metrics-action/`.

- **SC-013** (M4 freeze): The compliance test `TestCompliance_M4_AdoptedPlugins` MUST continue to assert exactly 19 plugin directories (+ base / core). M6 MUST NOT touch the plugin set.

## Assumptions

- **Docker image**: the action ships as a Docker image hosted on **GitHub Container Registry (`ghcr.io/mjun0812/github-metrics`)** with semver + `latest` + per-commit `sha-<short>` tags (matching upstream's distribution model); no `runs.using: composite` shell-script wrappers needed.
- **CLI binary distribution**: pre-built binaries (linux/amd64, linux/arm64, darwin/amd64, darwin/arm64) are attached to each GitHub Release matching the Docker image tag; `go install github.com/mjun0812/github-metrics/cmd/metrics-action@latest` (and `@vX.Y.Z`) is supported as the toolchain-native fallback. Homebrew tap / aur / chocolatey / scoop are out of scope for MVP.
- **Single binary for Action + CLI**: the same `metrics-action` binary handles both modes; the run mode is inferred from environment context (presence of `GITHUB_ACTIONS=true` env var → action mode; otherwise CLI mode).
- **`action.yml` parity**: this spec ships an `action.yml` whose input definitions cover the 21 M4 plugin set + core inputs; we do not aim for line-by-line copy of upstream's 1600-line action.yml. Inputs that exist only for unadopted plugins are omitted by design (constitution 原則 III).
- **Preset YAML schema**: matches upstream's preset format (a single `q:` map of input overrides). No new preset features.
- **Markdown / Gist / Insights output actions**: out of scope. The spec covers `none`, `commit`, `pull-request`, `pull-request-merge`, `pull-request-squash`, `pull-request-rebase`. `gist`, `markdown commit`, and `markdown pull-request` are deferred (markdown variant lands with the markdown template, which is itself non-MVP).
- **`use_prebuilt_image`**: deferred — the action always docker-build / docker-pull the published image; there is no fallback "build locally during the run" path in MVP.
- **`clean_workflows`, `setup_community_templates`**: deferred (Node-version-specific cleanup helpers; not needed for Go-binary distribution).
- **CLI flag style**: standard POSIX-long-style flags (`--user`, `--template`, `--plugin`) implemented with Go's `flag` package or a small wrapper. No subcommands needed for MVP (single-shot run).
- **Config file format**: YAML only. JSON / TOML out of scope.
- **Token validation strictness**: `github_pat_*` is hard-rejected, classic PATs are accepted as long as `HEAD /` returns X-OAuth-Scopes (Bot token / app token flows are out of scope; they would arrive with a separate spec).
- **Banner format**: human-readable text via slog (Default level=info), not JSON. CI log readability prioritized. **Language: English fixed** (matches upstream Node banner for grep / dashboard compatibility).
- **Logging defaults**: `slog` default handler, level=info, machine-readable + human-readable middle ground. All operator-facing log lines (info / warning / error, FR-021 release-notice, FR-015b migration message) are English (the Japanese-language project policy applies to spec / conversation / commit messages, not runtime artifacts).
- **No web UI / HTTP server**: explicitly excluded per `docs/design/15-selection-answer.md` §1 — M6 ships Action + CLI only.
- **Exit codes**: 0 = success or intentional skip; 1 = config error / token error / unrecoverable Compute error. No finer-grained exit codes in MVP.
- **GitHub Enterprise**: standard `github_api_rest` / `github_api_graphql` override inputs are honored, but no GHE-specific test cases ship in MVP.

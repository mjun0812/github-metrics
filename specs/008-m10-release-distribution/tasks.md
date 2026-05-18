---

description: "M10 — release / Docker distribution task list"
---

# Tasks: M10 — release / Docker distribution

**Input**: Design documents from `/specs/008-m10-release-distribution/`

**Prerequisites**: plan.md (required), spec.md (required for user stories), research.md, data-model.md, contracts/

**Tests**: Test tasks ARE included for the new test files mandated by
plan.md (the docker-smoke integration test + the compliance
invariant guard). The release pipeline itself is verified via dry-run
mode (a single task — T012) rather than via a unit-test suite,
matching M6's release.yml verification pattern.

**Organization**: Tasks are grouped by user story to enable independent
implementation and testing of each story.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2, US3)
- Include exact file paths in descriptions

## Path Conventions

- **Single project**: `cmd/`, `internal/`, `deploy/`, `.github/workflows/`,
  `scripts/`, `tests/`, `docs/` at repository root (per plan.md
  "Project Structure" section).

---

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Project ergonomics + Makefile targets used by every story's verification step.

- [ ] T001 Add `docker-smoke` and `release-dry-run` Makefile targets to `Makefile`. `docker-smoke` runs `go test -tags=docker_smoke ./tests/integration/...`; `release-dry-run` wraps `gh workflow run release.yml -f dry_run=true && gh run watch` for ergonomics (per quickstart.md §6 "Local dev shortcuts").

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Constitution III invariant guard + release-verification helper script skeleton. These are the only cross-story prerequisites — they unblock both US1 (release publish) and US2 (consumer verification).

**⚠️ CRITICAL**: No user story work can begin until this phase is complete.

- [ ] T002 [P] Add `TestCompliance_M10_PluginTemplateInvariant` to `tests/compliance/compliance_test.go`: re-assert the M1-M9 adopted-feature set is unchanged after M10 (FR-010). The test must enumerate the 21 adopted plugin directory names + 2 adopted template names and fail if either set drifts.
- [ ] T003 [P] Create `scripts/release-verify.sh` skeleton with argument parsing (accept `<tag>` positional), usage message, `set -euo pipefail`, and 4 empty sections marked `# 1. docker manifest`, `# 2. sha256sum`, `# 3. cosign verify`, `# 4. action.yml grep` (per quickstart.md §4). Body content lands in US1 (T011).

**Checkpoint**: Foundation ready — US1 / US2 / US3 implementation can now begin in parallel.

---

## Phase 3: User Story 1 — Cut tagged release with multi-arch Docker + signed binaries (Priority: P1) 🎯 MVP

**Goal**: A semver tag push (or manual `workflow_dispatch` real-run) publishes a multi-arch GHCR image, 4 cross-compiled CLI binaries, a `SHA256SUMS` file, and cosign signatures for image + each binary, end-to-end within 25 minutes.

**Independent Test**: Push `v0.99.0-rc1` to a fork branch; the release workflow completes; `docker manifest inspect ghcr.io/<fork>/github-metrics:v0.99.0-rc1` lists both `linux/amd64` and `linux/arm64` entries; downloaded binaries pass `sha256sum -c SHA256SUMS`; `cosign verify` against the image returns success. Spec SC-001..SC-004 + SC-007.

### Implementation for User Story 1

- [ ] T004 [US1] Update `deploy/Dockerfile`: add non-root `metrics` user (uid 10001, gid 10001, `--no-create-home`, `/sbin/nologin` shell), `USER metrics` directive after binary COPY; reorder runtime-stage RUN steps so `apt-get clean` + `rm -rf /var/lib/apt/lists/*` runs in the same layer as install; ensure multi-arch builds cleanly (no arch-specific RUN). Reference `contracts/dockerfile.md` §1-§3.
- [ ] T005 [P] [US1] Create `tests/integration/dockerfile_test.go` with build tag `//go:build docker_smoke`: builds the image via `docker build -f deploy/Dockerfile -t metrics-action:m10-smoke .`, runs `docker run --rm metrics-action:m10-smoke metrics-action --help` and asserts exit 0 + stdout contains `Usage:` or `metrics-action`; asserts image size ≤ 600 MB per `contracts/dockerfile.md` §5. Skip the test if `docker` binary is not available on the test host.
- [ ] T006 [US1] Extend `.github/workflows/release.yml` `release-docker` job: add `docker/setup-qemu-action@v3` + `docker/setup-buildx-action@v3` steps; change `docker/build-push-action@v5` invocation to include `platforms: linux/amd64,linux/arm64`; add a size-budget assertion shell step that runs `docker image inspect` and fails when size > 600 MB; install cosign via `sigstore/cosign-installer@v3`; add `cosign sign --yes <image-ref>` step gated on `${{ inputs.dry_run != 'true' }}`. Reference `contracts/release-workflow.md` §2.1.
- [ ] T007 [US1] Extend `.github/workflows/release.yml` `release-binary` job: after the matrix `go build` step, add a SHA256SUMS generation step (`cd dist && sha256sum metrics-action_* > SHA256SUMS`); install cosign; for each binary + the SHA256SUMS file, run `cosign sign-blob --yes --output-signature ${f}.sig --output-certificate ${f}.cert --bundle ${f}.cosign.bundle ${f}` gated on `${{ inputs.dry_run != 'true' }}`; replace the dry-run path's `actions/upload-artifact@v4` invocation so it includes the new SHA256SUMS + .sig + .cert + .cosign.bundle files. Reference `contracts/release-workflow.md` §2.2.
- [ ] T008 [US1] Add the `id-token: write` permission to the workflow-level `permissions:` block in `.github/workflows/release.yml` (sits next to existing `contents: write` and `packages: write`). Reference `contracts/release-workflow.md` §3.
- [ ] T009 [P] [US1] Update `internal/tools/gen-action-yml/main.go` (and any template/data files it consumes — likely under `internal/tools/gen-action-yml/`) so the emitted `runs.image:` line is a parameterized string defaulting to the project version (e.g., reads `VERSION` env var or builds from the `cmd/metrics-action` ldflags pattern), producing `docker://ghcr.io/mjun0812/github-metrics:<version>`. Pre-release path stays `Dockerfile` for local builds. Reference `contracts/action-yml.md` §1-§3.
- [ ] T010 [US1] Run `make gen-action-yml` (or the underlying `go run ./internal/tools/gen-action-yml`) with `VERSION=v1.0.0` (placeholder for the eventual release tag); verify `action.yml` `runs.image:` line now reads `'docker://ghcr.io/mjun0812/github-metrics:v1.0.0'`; verify lefthook's `action-yml-drift` pre-commit hook passes (re-running the generator yields no diff). Reference `contracts/action-yml.md` §3.
- [ ] T011 [P] [US1] Fill in the body of `scripts/release-verify.sh` (skeleton from T003): section 1 calls `docker manifest inspect ghcr.io/mjun0812/github-metrics:${TAG}` and asserts both `linux/amd64` and `linux/arm64` digest entries (SC-001); section 2 calls `sha256sum -c SHA256SUMS` against locally-downloaded binaries (SC-003); section 3 calls `cosign verify ghcr.io/.../...:${TAG} --certificate-identity-regexp 'https://github.com/mjun0812/github-metrics/.github/workflows/release.yml@refs/tags/v.*' --certificate-oidc-issuer https://token.actions.githubusercontent.com` (SC-004); section 4 greps `action.yml` for `image: 'docker://ghcr.io/mjun0812/github-metrics:${TAG}'` and exits non-zero on mismatch (FR-007). Per quickstart.md §4 + contracts/action-yml.md §6.
- [ ] T012 [US1] Trigger the release pipeline in dry-run mode via `gh workflow run release.yml -f dry_run=true` from the `008-m10-release-distribution` branch and verify: (a) `release-docker` job uploads the multi-arch buildx logs + size-budget log line ≤ 600 MB; (b) `release-binary` job uploads 4 binaries + `SHA256SUMS` as workflow artifacts with 7-day retention; (c) no GHCR push, no GH Release entry, no Rekor entry; (d) end-to-end runtime ≤ 25 min (SC-007). Capture the workflow run URL in the spec checklist for traceability. Per quickstart.md §1.

**Checkpoint**: At this point, US1 is fully exercised end-to-end via the dry-run path; the only remaining gap to publishing v1.0.0 is the actual tag push (which happens at release-cut time, outside the per-PR task list).

---

## Phase 4: User Story 2 — Action consumer pins to released tag and runs (Priority: P2)

**Goal**: An end-user workflow referencing `uses: mjun0812/github-metrics@v1.0.0` runs to completion on both x86 (`ubuntu-latest`) and arm64 (`ubuntu-24.04-arm`) GitHub-hosted runners, producing DOM-equivalent output to the local-build (`uses: ./`) baseline.

**Independent Test**: After v1.0.0 is published, manually trigger the new `sample-action-smoke.yml` workflow (defined in T013) on both runners; assert both jobs complete green within 90s per SC-006.

### Implementation for User Story 2

- [ ] T013 [US2] Create `.github/workflows/sample-action-smoke.yml`: `on: workflow_dispatch`; one job `smoke-x86` on `ubuntu-latest` and one job `smoke-arm` on `ubuntu-24.04-arm`, both invoking `uses: mjun0812/github-metrics@v1.0.0` with `with: { user: octocat, token: ${{ secrets.METRICS_TOKEN }}, template: classic, output: svg, dryrun: 'yes' }`. The token secret name `METRICS_TOKEN` follows the README convention. Document in the YAML header comments that this workflow is intended for post-release smoke verification per spec SC-006 + quickstart.md §5. The workflow is checked in but not auto-triggered — maintainers run it manually after each release.

**Checkpoint**: US2 is wired and ready to be exercised once the v1.0.0 tag is published (which is outside the M10 implementation PR — happens at release-cut time).

---

## Phase 5: User Story 3 — Upstream user migrates with a documented guide (Priority: P3)

**Goal**: A developer currently running `lowlighter/metrics@v3` can read `docs/migration-to-go.md` in under 10 minutes, identify the unported plugins/templates, understand input-compatibility semantics, and execute the migration + rollback procedure.

**Independent Test**: SC-005 reader check — a first-time reader answers 3 questions correctly within 10 minutes: (a) which plugins are unavailable, (b) are existing inputs recognized, (c) how to roll back. The migration-guide contract §4 spells out the acceptance protocol.

### Implementation for User Story 3

- [ ] T014 [US3] Create `docs/migration-to-go.md` per `contracts/migration-guide.md`: 6 sections in Japanese (概要 / 採用機能一覧 / 未対応機能一覧 / 入力互換性 / 移行手順 / ロールバック), ≤ 250 lines total. Enumerate all 21 adopted plugins + 2 adopted templates + 19 M8 unported plugins explicitly with rows referencing `docs/design/15-selection-answer.md`. Include the worked `with:` example from `contracts/migration-guide.md` §2.4 and the one-line rollback diff from §2.6. No promotional language, no upstream-benchmark claims.
- [ ] T015 [US3] Execute the SC-005 reader-acceptance check from `contracts/migration-guide.md` §4: ask a teammate (or the project maintainer if solo) unfamiliar with the M10 doc to read `docs/migration-to-go.md` and answer the 3 acceptance questions within 10 minutes; record the outcome in the M10 PR description. If any question is mis-answered, iterate on the guide and re-run.

**Checkpoint**: All 3 user stories are now independently verifiable. The release pipeline can be triggered (US1), the consumer smoke is wired (US2), and the migration story has a reviewed guide (US3).

---

## Phase 6: Polish & Cross-Cutting Concerns

**Purpose**: README + meta-doc cross-link refresh, final regression gate, post-release verification.

- [ ] T016 [P] Update `README.md`: bump the `@v0.6.0` Action-usage examples to `@v1.0.0` (in both the GitHub Action section and the Docker run example); add a one-line link to `docs/migration-to-go.md` near the project description for upstream users.
- [ ] T017 [P] Update `CLAUDE.md` "Adopted phase order" parenthetical from `(M1-M4 + M6 + M7 + M9 完了済、現在は M10 spec/plan 段階)` to `(M1-M4 + M6 + M7 + M9 + M10 完了済)` once M10 is merged (this can be amended in the merge commit or a follow-up).
- [ ] T018 Run the full regression locally — `make test` + `make lint` + `make hooks-run` — and verify all jobs pass. Re-run `tests/compliance/...` explicitly to confirm `TestCompliance_M10_PluginTemplateInvariant` (added in T002) is green. Per spec SC-008.
- [ ] T019 After the v1.0.0 tag is pushed and the release pipeline completes, run `scripts/release-verify.sh v1.0.0` (built in T011) and capture the full output in the M10 release notes / GitHub Release body. Re-trigger `sample-action-smoke.yml` (from T013) and confirm both x86 + arm64 jobs complete within 90s per SC-006.

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: T001 has no dependencies — can start immediately.
- **Foundational (Phase 2)**: T002 + T003 depend on Setup completion. T002 + T003 themselves are independent and can run in parallel. Blocks all user stories.
- **User Stories (Phase 3-5)**: All depend on Foundational completion.
  - US1 + US2 + US3 are file-disjoint and can run in parallel if staffed.
  - US2's T013 references the v1.0.0 tag but the workflow file is checked in regardless of whether the tag exists yet.
- **Polish (Phase 6)**: T016 + T017 are file-independent and parallel; T018 sequential after both US1-US3 implementation complete; T019 is post-publish and runs after the release tag.

### User Story Dependencies

- **US1 (P1)**: Independent of US2/US3. Can ship as MVP — produces the release pipeline that makes v1.0.0 publishable.
- **US2 (P2)**: Depends on US1 for the actual published image (the workflow file alone is independent; the smoke run requires v1.0.0 to exist).
- **US3 (P3)**: Independent of US1/US2 — pure documentation. Can be drafted in parallel.

### Within US1

- T004 (Dockerfile) before T005 (docker-smoke test) — test exercises the modified image.
- T005 in parallel with T006 (different files).
- T006 → T007 → T008 — sequential because all three modify `.github/workflows/release.yml`.
- T009 (gen-action-yml) before T010 (regenerate action.yml) — sequential.
- T009 / T011 in parallel with the release.yml chain (different files).
- T012 (dry-run) depends on T004..T011 all being merged or at least pushed to a runnable branch.

### Within US3

- T014 before T015 — reader-check requires the guide to exist.

### Parallel Opportunities

- T002 ∥ T003 (Foundational)
- T004 ∥ T005 ∥ T009 ∥ T011 (US1 early stages — disjoint files)
- T006 → T007 → T008 (must be sequential — same file)
- US1 ∥ US3 (different files, different domains)
- T016 ∥ T017 (Polish — different files)

---

## Parallel Example: US1 early stages

```bash
# Launch the file-disjoint US1 tasks together once T002/T003 complete:
Task: "Update deploy/Dockerfile per contracts/dockerfile.md"        # T004
Task: "Create tests/integration/dockerfile_test.go (docker_smoke)"  # T005
Task: "Update internal/tools/gen-action-yml for version arg"        # T009
Task: "Fill scripts/release-verify.sh body"                          # T011
```

(T006/T007/T008 then run sequentially because all three modify
`.github/workflows/release.yml`.)

---

## Implementation Strategy

### MVP First (US1 Only)

1. Complete Phase 1: Setup (T001).
2. Complete Phase 2: Foundational (T002, T003).
3. Complete Phase 3: US1 (T004..T012) — at this point the dry-run pipeline is end-to-end green, the production-grade Dockerfile is in place, action.yml references the eventual v1.0.0 image, and the release-verify script is ready to run.
4. **STOP and VALIDATE**: T012 dry-run output is the MVP acceptance gate.
5. Optionally cut v1.0.0 right here — the M10 contract is satisfied for the release-pipeline story.

### Incremental Delivery

1. Setup + Foundational → MVP scaffolding ready.
2. Add US1 → dry-run green → cut v0.99.0-rc1 as a verification release on a fork.
3. Add US2 → wire the sample smoke; defer execution until a real release exists.
4. Add US3 → migration guide draft + reader review.
5. Polish → README + CLAUDE.md updates → final regression → publish v1.0.0 → T019 post-publish verification.

### Parallel Team Strategy

If staffed by more than one developer:

1. Team completes T001-T003 together.
2. Once Foundational is done:
   - Developer A: US1 (T004..T012) — the core deliverable.
   - Developer B: US3 (T014..T015) — migration guide (doc-only, independent).
3. After US1 lands on `008-m10-release-distribution`, anyone can add US2 (T013).
4. Polish (T016..T019) is a single-developer wrap-up after everything merges.

---

## Notes

- **Tests requested**: only the compliance invariant (T002) and the docker-smoke (T005) get dedicated test files. Release-workflow correctness is verified via the dry-run path (T012) rather than a unit-test suite — this matches M6's release.yml validation pattern and reflects the fact that the release pipeline IS the code under test.
- **[P] = different files, no dependencies**: applied conservatively. Tasks touching the same workflow YAML or the same Dockerfile are sequential even if the edits are notionally independent.
- **Constitution III guard**: T002's `TestCompliance_M10_PluginTemplateInvariant` is the load-bearing assertion that M10 does not silently introduce M5/M8 features. CI runs this on every push.
- **Verification artifacts**: T012's dry-run URL + T015's reader check outcome + T019's release-verify output go into the M10 PR description for audit trail.
- **Tag-push happens outside the PR**: T019 explicitly runs *after* the release tag is pushed, which is a maintainer action separate from merging the M10 PR. This matches the M6/M7 pattern.
- **Windows binary deliberately out of scope** for v1.0 per spec assumption — the matrix in T007 stays at 4 variants (linux/darwin × amd64/arm64).

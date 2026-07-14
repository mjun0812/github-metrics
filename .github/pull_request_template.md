<!--
Thanks for the PR!

Before requesting review, please:
- Read `CONTRIBUTING.md` (development workflow, commit style, test categories, scope discipline).
- Confirm the change fits the adopted MVP (`docs/scope.md`). Non-adopted plugins / templates / output formats are out of scope.
- Run `make test`, `golangci-lint run ./internal/... ./tests/...`, and (if you touched rendering) `make test-resvg`.
-->

## Summary

<!-- One-paragraph description of what changes and why. -->

## Related

<!-- Issue / discussion links. -->

- Closes #

## Type of change

- [ ] Bug fix (non-breaking)
- [ ] Feature (non-breaking)
- [ ] Breaking change
- [ ] Docs / tooling only

## Test plan

<!-- How you verified the change end-to-end. -->

- [ ]
- [ ]

## Checklist

- [ ] Change is in scope per `docs/scope.md`.
- [ ] Tests updated / added (goldens regenerated intentionally — no drive-by timestamp diffs).
- [ ] `make test` passes.
- [ ] `golangci-lint run ./internal/... ./tests/...` passes.
- [ ] Docs updated when relevant (`docs/`, `README.md`, `README_ja.md`, generated plugin pages via `make docs`).

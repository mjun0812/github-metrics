// Package testutil hosts the shared test infrastructure landed in M9.
//
// The package consolidates the mock GraphQL / REST RoundTrippers and
// golden-file helpers that M1-M7 grew ad-hoc across 8+ test files.
// Per [docs/design/10-testing-deployment.md §2 mocks-の設計] +
// [docs/design/16-tasks-mvp.md Phase M9 (T-118..T-125)], the canonical
// surface lives here so future tests reuse one well-tested helper set
// instead of copying scattered scaffolding.
//
// Sub-packages:
//
//   - mocks/  — RESTMux (http.RoundTripper) + GraphQLMux (Doer) +
//     PluginContext builder. Fixture files dispatched from
//     tests/fixtures/github/{rest,graphql}/<name>.json.
//   - golden/ — Compare / CompareSVG / CompareJSON family unified
//     around the existing -update flag (declared in
//     tests/integration/output_json_test.go).
//
// Scope guarantee: testutil is an adopted M9 surface per the project
// constitution principle III. No production code path imports this
// package — it is consumed exclusively by *_test.go files.
package testutil

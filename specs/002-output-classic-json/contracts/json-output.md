# Contract: JSON output

**Date**: 2026-05-15 | **Plan**: [../plan.md](../plan.md)

本書は `engine.Marshal(*plugins.Data)` が返す JSON の構造契約を定義する。constitution 原則 II「JSON 出力は上流と完全互換」を満たすため、キー集合と型 shape はここに記載した規則と一致しなければならない。

## 1. トップレベル shape

```json
{
  "user": { ... },
  "account": "user" | "organization" | "repository",
  "config": {
    "timezone": { "name": "Asia/Tokyo", "error": null }
  },
  "computed": {
    "commits": 1234,
    "repositories": {
      "count": 250,
      "stargazers": 150,
      "forks": 13,
      "releases": 0,
      "watchers": 5,
      "issues": 0,
      "pullRequests": 0,
      "languages": { "Go": 12345, "JavaScript": 8200 }
    }
  },
  "plugins": {
    "<plugin-name>": <plugin-payload> | "[Circular]"
  },
  "errors": [
    { "error": "engine: template \"foo\": ..." }
  ]
}
```

- `user`: `*plugins.User` の field を camelCase で出力。`null` 許容 (base plugin が失敗した場合)。
- `account`: `plugins.AccountKind` の string 値。
- `config`: M1 で確立した `ComputedConfig` の shape。`timezone.error` は string (`err.Error()`) または `null`。
- `computed`: `plugins.Computed` 構造体の field を camelCase で出力。
- `plugins`: 各 plugin の自由形式 payload。M1 では空 map / map[string]error の場合あり (Die=false 経路)。
- `errors`: `data.Errors` の各 `error` を `{"error": "<msg>"}` object に正規化。空の場合は `[]` を返す ([] でも null でも上流互換だが、`[]` で揃える)。

## 2. キー命名規約

- Go 構造体の exported field は **lowerCamelCase** で出力。`User.Login` → `"login"`、`Repositories.PullRequests` → `"pullRequests"`。
- 例外: `data.Plugins` の **キー** は plugin slug (= `Plugin.Name()`) をそのまま使う。dot を含むキー (`languages.recent` 等) も許容する。
- `error` キーは小文字単数形。

## 3. 値の正規化 ([data-model.md §E-002](../data-model.md))

| Go 型 | JSON 表現 |
|---|---|
| `nil`, `*T == nil` | `null` |
| `string` | `"..."` |
| `int / int64 / float64` | 数値 |
| `bool` | `true` / `false` |
| `time.Time` | RFC 3339 string (`"2025-01-01T00:30:00Z"`) |
| `map[string]any` | object (キー lexicographic ソート) |
| `map[K]V` where K != string | `[{"key": K, "value": V}]` 配列 (Key 安定ソート) |
| `map[T]struct{}` (set 慣習) | ソート済み `[T]` 配列 |
| `[]T` | array (各要素を再帰 normalize) |
| `error` | `{"error": "<err.Error()>"}` |
| `config.Token` | `"(provided)"` か `""` (FR-012) |
| 循環参照 | `"[Circular]"` string |

## 4. 上流互換性 (SC-001)

`tests/fixtures/upstream/octocat.json` (上流 `lowlighter/metrics` の `output_action=metrics.json` 出力) のキー集合と本実装出力のキー集合差分が **0 件**。

- 比較対象: トップレベル + `computed.*` + `user.*` + 各 `plugins[*]` の **キー名のみ**。値の型は M4 plugin が揃うまで shape 差分を許容 (例: 採用していない plugin 由来のキーは未出現で OK)。
- 比較は `tests/compatibility/json_test.go` に実装。

## 5. 循環参照 (SC-FR-002)

- `data.Plugins["sample"]` が自己参照 (struct.Self = &struct) を持つケース、互いに参照する 2 plugin payload (A → B → A) のケース、共に `"[Circular]"` 文字列に置換され Marshal は完走する。
- cycleDetector は `reflect.Value.Pointer()` を訪問済みキーにする。`Pointer()` が unaddressable な場合は再入扱いとし、即 `"[Circular]"` を返す保守的設計。

## 6. テスト契約

- **Golden**: `tests/golden/json/<case>.json`。`assert.JSONEq` で比較 (whitespace 非依存)。
- **Update**: `go test ./... -update` で再生成 (M1 で確立した規約)。
- **Upstream diff**: `tests/compatibility/json_test.go::TestUpstreamKeysetCompatibility` を必須テストとして CI で実行 (`compliance` job に統合検討、本 spec の T-029 で決める)。
- **Cycle**: `tests/integration/json_cycle_test.go` で自己参照 plugin payload を Marshal し、`"[Circular]"` を含むことを assert。

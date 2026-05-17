# Contract: output_action validation

**Date**: 2026-05-17 | **Plan**: [../plan.md](../plan.md) | **Related**: [../data-model.md](../data-model.md) E-008

本書は `output_action` 入力値の validation 規則を定める。spec clarification Q4 で確定した「未対応値は fail-fast + 明示的 migration message」を実装する。

## 1. サポート値一覧 (6 値)

```text
none
commit
pull-request
pull-request-merge
pull-request-squash
pull-request-rebase
```

これら以外の値はすべて **fail-fast (exit 1)** で拒否。silent fallback は禁止 (FR-015b)。

## 2. 未対応値の migration message テンプレート

| 入力値 | Exit code | Message (English fixed, 1 line) |
|--------|-----------|----------------------------------|
| `gist` | 1 | `output_action='gist' is not supported in this distribution. Use 'commit' to commit the rendered file directly, or open an issue if Gist support is critical to your workflow.` |
| `markdown commit` | 1 | `output_action='markdown commit' is not supported (markdown template is out of MVP scope). Use 'commit' with config_output=svg/png/jpeg/json. See specs/004-m4-github-plugins/spec.md for the adopted template list.` |
| `markdown pull-request` | 1 | `output_action='markdown pull-request' is not supported (markdown template is out of MVP scope). Use 'pull-request' with config_output=svg/png/jpeg/json.` |
| `markdown gist` | 1 | `output_action='markdown gist' is not supported (both markdown template and Gist are out of MVP scope). Use 'commit' or 'pull-request' with config_output=svg/png/jpeg/json.` |
| `gist pull-request` | 1 | `output_action='gist pull-request' is not supported (Gist is out of MVP scope). Use 'pull-request' to open a PR with the rendered file.` |
| 任意の未知文字列 | 1 | `output_action='<value>' is not a recognized value. Supported: none / commit / pull-request / pull-request-merge / pull-request-squash / pull-request-rebase. See https://github.com/mjun0812/github-metrics/blob/main/action.yml for full input docs.` |

## 3. 実装 (E-008 `OutputActionRegistry`)

```go
package action

type OutputActionRegistry struct {
    Supported            []string
    UnsupportedMigration map[string]string
}

func DefaultRegistry() *OutputActionRegistry {
    return &OutputActionRegistry{
        Supported: []string{
            "none",
            "commit",
            "pull-request",
            "pull-request-merge",
            "pull-request-squash",
            "pull-request-rebase",
        },
        UnsupportedMigration: map[string]string{
            "gist":                  "...", // 上記表参照
            "markdown commit":       "...",
            "markdown pull-request": "...",
            "markdown gist":         "...",
            "gist pull-request":     "...",
        },
    }
}

// Validate returns nil iff value is in Supported.
// Otherwise it returns a *ConfigError with the migration message.
func (r *OutputActionRegistry) Validate(value string) error {
    for _, s := range r.Supported {
        if value == s {
            return nil
        }
    }
    if msg, ok := r.UnsupportedMigration[value]; ok {
        return &ConfigError{
            Key:   "output_action",
            Value: value,
            Msg:   msg,
        }
    }
    return &ConfigError{
        Key:   "output_action",
        Value: value,
        Msg:   fmt.Sprintf("output_action=%q is not a recognized value. Supported: %s. ...",
            value, strings.Join(r.Supported, " / ")),
    }
}

// ConfigError is reported with exit code 1 (non-retryable, see retry-policy.md).
type ConfigError struct {
    Key   string
    Value string
    Msg   string
}

func (e *ConfigError) Error() string { return e.Msg }
```

## 4. 起動順序

`internal/action/action.go::Run` の中で **Compute 呼び出し前** に validation:

```text
1. parseInputs (INPUTS JSON / INPUT_<UPPER> / preset merge)
2. DefaultRegistry().Validate(inputs["output_action"]) → エラーなら即 exit 1
3. tokenValidator.Validate(...)
4. engine.Compute(...)
5. Committer.Run(...)
```

これにより、ユーザが `output_action=gist` で workflow を起動した場合、**engine.Compute が呼ばれる前に** 即座に fail し、無駄な API 消費 + GitHub Actions の minute を浪費しない。

## 5. テスト戦略 (SC-007 後半: 1 test case per unsupported value)

`internal/action/output_action_test.go`:

```go
func TestRegistry_Validate(t *testing.T) {
    r := DefaultRegistry()

    // Supported values
    for _, s := range r.Supported {
        if err := r.Validate(s); err != nil {
            t.Errorf("Validate(%q) = %v, want nil", s, err)
        }
    }

    // Unsupported with migration message
    cases := []string{"gist", "markdown commit", "markdown pull-request", "markdown gist", "gist pull-request"}
    for _, value := range cases {
        err := r.Validate(value)
        if err == nil {
            t.Errorf("Validate(%q) = nil, want error", value)
            continue
        }
        var ce *ConfigError
        if !errors.As(err, &ce) {
            t.Errorf("Validate(%q) error type = %T, want *ConfigError", value, err)
        }
        if !strings.Contains(ce.Msg, "not supported") && !strings.Contains(ce.Msg, "Supported") {
            t.Errorf("Validate(%q) message = %q, want it to mention 'not supported' / 'Supported'", value, ce.Msg)
        }
    }

    // Generic unknown value
    err := r.Validate("foo-bar-baz")
    if err == nil {
        t.Fatal("expected error for unknown value")
    }
    if !strings.Contains(err.Error(), "Supported:") {
        t.Errorf("generic error message missing list of supported values: %q", err.Error())
    }
}
```

End-to-end test (`tests/integration/action_test.go::TestAction_Unsupported_OutputAction_FailFast`):

- mocked engine + `INPUT_OUTPUT_ACTION=gist` を env で渡す
- main 関数の終了 status が 1 であることを assert
- stderr に migration message が出ていることを assert
- engine.Compute が **一度も呼ばれていない** ことを mock の call counter で assert (FR-015b の "fail-fast before Compute" を担保)

# Contract: CLI flags

**Date**: 2026-05-17 | **Plan**: [../plan.md](../plan.md) | **Related**: [../data-model.md](../data-model.md) E-004

本書は `metrics-action` バイナリを CLI として起動した時のフラグ仕様を定める。Action モードと CLI モードは同じバイナリで、起動時の env (`GITHUB_ACTIONS=true`) と flag の有無で分岐する。CLI flag は action.yml input と 1:1 対応するため、action.yml で動く inputs は CLI でも動く (FR-019)。

## 1. フラグ一覧

| Flag | Type | Default | Action yml equivalent | Description |
|------|------|---------|----------------------|-------------|
| `--config <path>.yaml` | string | '' | (なし) | YAML config を読み込み、action.yml 互換の inputs に展開 |
| `--user <login>` | string | '' | `with.user` | GitHub user / org login |
| `--template <name>` | string | `classic` | `with.template` | template (現状 `classic` のみ) |
| `--token <PAT>` | string | '' | `with.token` | GitHub Personal Access Token (history に残るので非推奨、`--token-env` 推奨) |
| `--token-env <NAME>` | string | '' | (CLI 限定) | `os.Getenv(NAME)` で token を取得 |
| `--plugin <key>=<value>` | repeatable | (none) | `with.plugin_<key>: <value>` | plugin 入力。複数指定可。`--plugin plugin_languages=true --plugin plugin_languages_limit=5` |
| `--output <fmt>` | string | `svg` | `with.config_output` | svg / png / jpeg / json |
| `--filename <path>` | string | `github-metrics.*` | `with.filename` | 出力先パス。`-` で stdout |
| `--dryrun` | bool | false | `with.dryrun: yes` | ファイル出力のみ、commit / PR 系の output_action を skip |
| `--preset <path>.yaml` | string | '' | `with.config_presets` | preset YAML パス |

## 2. 使用例

```bash
# 最小: octocat の classic SVG を stdout に出す (mocked deps)
metrics-action --user octocat --token-env GITHUB_TOKEN --output svg --filename - --dryrun

# YAML config で action.yml と等価な動作 (本番 token + plugin 設定)
metrics-action --config metrics-inputs.yaml --token-env GITHUB_TOKEN

# plugin を CLI flag で個別指定 (preset 不使用)
metrics-action --user octocat --token-env GITHUB_TOKEN \
  --plugin plugin_languages=true \
  --plugin plugin_languages_limit=5 \
  --plugin plugin_activity=true \
  --output png --filename out.png

# pipe して他ツールに渡す
metrics-action --user octocat --token-env GITHUB_TOKEN --output svg \
  --filename - --dryrun | xmllint --pretty 1 -
```

## 3. 入力優先度 (CLI mode)

複数の入力源が衝突した場合の優先度 (highest first):

1. CLI flag (`--user`, `--plugin key=val` 等)
2. `--config <path>.yaml` の YAML 内容
3. `--preset <path>.yaml` の preset の `q:` map
4. environment variable (`INPUT_<UPPER>`)
5. metadata.yml の default

これは action.yml mode の優先度 (INPUTS JSON > INPUT_<UPPER> > preset > metadata default) と semantic 一致 (FR-019 の "byte-identical to the equivalent action.yml input" を担保)。

## 4. token 入力の取り扱い

| Source | Behavior |
|--------|----------|
| `--token <PAT>` | そのまま使う。**shell history に残る点を warning ログで通知**。本番 token には非推奨 |
| `--token-env <NAME>` | `os.Getenv(NAME)` を読む。`NAME` 自体が空 / env 未定義なら error |
| 両方指定 | `--token-env` を優先 + warning ログ ("--token ignored when --token-env is set") |
| どちらも未指定 | dryrun + `use_mocked_data=true` なら OK、それ以外は exit 1 (token required) |

token validation (`github_pat_*` 拒否 / scope warning / quota check) は CLI / Action 共通 (`internal/action/token.go::Validator`)。

## 5. `--filename` の semantics

| Value | Action |
|-------|--------|
| `-` | stdout に書く (binary format = png/jpeg は文字化け警告ログ、ユーザ責任) |
| `*.svg` のような wildcard | `*` 部分を `config.output` で決定 (`svg` / `png` / `jpeg` / `json`) |
| 通常パス | そのまま書く (mkdir -p で親 dir 作成) |

## 6. `--config <path>.yaml` schema

```yaml
# metrics-inputs.yaml — action.yml の inputs に対応する key: value
user: octocat
template: classic
output: svg
filename: github-metrics.*

plugins:
  languages: true
  languages_limit: 5
  activity: true
  achievements: true
  # ... 21 plugin の有効化 + 各 plugin の option (plugin_languages_limit など)
  # → action.yml では plugin_languages, plugin_languages_limit と prefix 付きだが
  # YAML config では plugins: 下にネスト + plugin_ prefix を省略可

config:
  presets: my-preset.yaml
  timezone: Asia/Tokyo
  padding: '5%'

committer:
  branch: ''         # 空ならデフォルト
  message: 'Auto-generated metrics for run #${run}'
  author: 'metrics[bot]'
  email: 'metrics@noreply.localhost'

output_action: commit
output_condition: data-changed
dryrun: false
retries: 3
retries_delay: 300
```

YAML loader (`internal/action/cli.go::LoadYAMLConfig`) はこれを action.yml 互換の flat `inputs: map[string]any` に変換する:

```yaml
{
  "user": "octocat",
  "template": "classic",
  "config_output": "svg",
  "filename": "github-metrics.*",
  "plugin_languages": true,
  "plugin_languages_limit": 5,
  "plugin_activity": true,
  "plugin_achievements": true,
  "config_presets": "my-preset.yaml",
  "config_timezone": "Asia/Tokyo",
  "config_padding": "5%",
  "committer_branch": "",
  "committer_message": "Auto-generated metrics for run #${run}",
  "committer_author": "metrics[bot]",
  "committer_email": "metrics@noreply.localhost",
  "output_action": "commit",
  "output_condition": "data-changed",
  "dryrun": false,
  "retries": 3,
  "retries_delay": 300,
}
```

## 7. error handling

| Condition | Exit code | Message |
|-----------|-----------|---------|
| `--user` 未指定 + `--config` も未指定 | 1 | `--user is required (or pass via --config)` |
| `--token` / `--token-env` 両方未指定 + `--dryrun` 未指定 + `use_mocked_data!=true` | 1 | `token required: pass via --token, --token-env, or use --dryrun with use_mocked_data: yes` |
| `--config` ファイル不在 | 1 | `config file not found: <path>` |
| `--preset` ファイル不在 | 1 | `preset file not found: <path>` |
| `--output` が svg/png/jpeg/json 以外 | 1 | `--output must be one of: svg, png, jpeg, json (got: <value>)` |
| 不明 flag | 2 (Go `flag` default) | flag package's default usage message |

## 8. テスト戦略 (SC-008 / SC-009)

- `internal/action/cli_test.go`:
  - `TestParseFlags_*` で 12+ flag combination × expected `CLIFlags` struct を table test
  - `TestLoadYAMLConfig_*` で 5 paired YAML / expected `Invocation.Inputs` map を table test
- `tests/integration/cli_test.go`:
  - `TestCLI_OctocatSVG_Stdout` で SC-008 (`metrics-action --user octocat --template classic --output svg --dryrun --filename -` が valid SVG を出す)
  - `TestCLI_ConfigYAML_Equivalence` で SC-009 (5 paired CLI vs action.yml で `engine.Request` が byte 一致)

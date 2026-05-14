# Contract: CLI / GitHub Action 入力

**Date**: 2026-05-15 | **Plan**: [../plan.md](../plan.md)

本書は `cmd/metrics-action` および `cmd/metrics-cli` が外部から受け取る入力の契約を定義する。M1 段階では **入力解釈までを satisfaction**、出力 (commit / pull-request / data-changed) は M6 (T-105..117) で扱う。

## 1. GitHub Action からの入力 (`cmd/metrics-action`)

### 1.1 環境変数

| 変数名 | 型 | 必須 | 由来 | 説明 |
|---|---|---|---|---|
| `INPUTS` | JSON 文字列 | optional | `action.yml` の `with:` 一括 | 上流互換。優先度最高。 |
| `INPUT_<UPPER>` | string | optional (各 input ごと) | `action.yml` の input | `INPUTS` 未指定時のフォールバック。上流と完全同名。 |
| `GITHUB_REPOSITORY` | string | optional | runner | Docker 環境では Action context を補完する目的で使う。 |
| `MOCKED_TOKEN` | string | optional | テスト時のみ | 値が "true" の場合、mock RoundTripper を強制注入。 |

### 1.2 入力解釈の順序

1. `INPUTS` が存在すれば JSON parse → `inputs` map のベース。
2. なければ `INPUT_<UPPER>` を各 input ごとに `os.Getenv` で取得。
3. `config_presets` 指定があれば `LoadPresets` で `@name` / URL / ローカルパス YAML をマージ (M1 範囲では preset ローダは optional 採用 T-006、本 spec の §6.1 追加で採用)。
4. `metadata.plugins.core.Inputs.ForAction(env, preset)` で `config_*` 入力を抽出、`q` map に展開。
5. `metadata.yml` の `type` に従って `NormalizeInput(def, raw)` で型変換。

### 1.3 上流互換性 (NON-NEGOTIABLE)

- input キー名は `action.yml` 上のものと **完全一致**。リネーム / エイリアス追加は MUST NOT。
- 既定値は `action.yml` の `default:` を踏襲。MUST NOT 変更。
- 未知 input は warn ログを残しスキップ (前方互換)。
- `INPUTS` JSON / `INPUT_<UPPER>` 環境の **両方** から読めること。

## 2. CLI からの入力 (`cmd/metrics-cli`)

### 2.1 M1 段階のフラグ (空 main + `--help` 動作)

M1 段階では FR-002 を満たすため `--help` が exit 0 を返す **空 main** を `bin/metrics-cli` として用意する。本格的なフラグは T-117 (M6) で実装する。

```text
metrics-cli — Generate GitHub user/org/repository metrics.

Usage:
  metrics-cli [flags]

Flags:
  -h, --help    Show help message and exit.
```

### 2.2 M6 で追加されるフラグ (将来契約 — 本 spec の範囲外だが互換維持の制約として記録)

| フラグ | 型 | 上流対応 | M6 で実装 |
|---|---|---|---|
| `--config <path>` | path | — | T-117 |
| `--user <login>` | string | `INPUT_USER` | T-117 |
| `--template <name>` | string | `INPUT_TEMPLATE` | T-117 |
| `--token <value>` | string | `INPUT_TOKEN` | T-117 |
| `--plugin key=val` | repeated KV | `INPUT_PLUGIN_*` | T-117 |
| `--output <format>` | string | `INPUT_OUTPUT_CONVERT` | T-117 |
| `--filename <pattern>` | string | `INPUT_FILENAME` | T-117 |
| `--dryrun` | bool | `INPUT_DRY_RUN` | T-117 |
| `--token-env KEY=ENV` | KV | — | T-117 |

CLI の YAML 入力は内部的に `INPUTS` 相当の構造に整形してから既存パイプライン (`cmd/metrics-action` の処理路) に流す。M1 段階ではこの整形ロジックは未実装。

## 3. `settings.json` 入力 (Web インスタンス互換だが本 MVP では Web 未実装)

Web インスタンスは採用しない (原則 III)。ただし上流互換のため `settings.json` ローダは存在する。

### 3.1 検索順

1. リポジトリルートの `settings.json` を `os.ReadFile`。
2. 存在しなければ `Settings{Port: 3000}` を返し、エラーにしない。
3. `Sandbox=true` (`internal/config` 内で sandbox モード判定) ではファイル読み込みをスキップし、defaults を強制設定する。

### 3.2 強制設定 (Sandbox)

`Sandbox=true` で以下を必ず上書きする:

- `Optimize = true`
- `Cached = 0`
- `PluginsDefault = true`
- `Extras.Default = true`
- `Mocked = true`

### 3.3 `//` キー処理

トップレベルおよび任意のネストで、key が `//` で始まるエントリは無視する (R-005)。

## 4. 動的プレースホルダ

`metadata.yml` の input value および preset value で以下のプレースホルダが使える。

| プレースホルダ | 解決値 | 解決元 |
|---|---|---|
| `.user.login` | `data.User.Login` | base plugin 後 |
| `.user.name` | `data.User.Name` | base plugin 後 |
| `.repository.name` | `data.Repository.Name` | repository account 時 |
| `.repository.full_name` | `data.Repository.FullName` | 同上 |

未解決のプレースホルダは warn ログを残し、元の文字列をそのまま残す (Edge Case)。

## 5. テスト契約

- `tests/fixtures/inputs/*.yml` 配下のケース YAML はそれぞれ独立した `INPUTS` 相当の入力を保持。
- `tests/golden/inputs/*.json` 配下に期待マップを置き、`Inputs.ForAction` の出力と golden 比較 (FR-030)。
- `MOCKED_TOKEN=true` 環境下で `cmd/metrics-action` を起動した場合、`internal/githubapi` の test helper が mock RoundTripper を注入し、実 GitHub URL への通信は MUST NOT 発生する (FR-017 / SC-004)。

# 設定・入力

入力は upstream `action.yml` と互換のフラットなキー空間で表現される。パースは `internal/action.ParseInputs` (env) と `internal/action.CLIFlags` (フラグ) が担う。

## 目次

- [1. 入力の解決順](#1-入力の解決順)
- [2. 入力の命名体系](#2-入力の命名体系)
- [3. CLI フラグ](#3-cli-フラグ)
- [4. `--config` YAML](#4---config-yaml)
- [5. 型の正規化](#5-型の正規化)
- [6. 動的プレースホルダ](#6-動的プレースホルダ)
- [7. metadata.yml と action.yml](#7-metadatayml-と-actionyml)

---

## 1. 入力の解決順

単一バイナリが Action と CLI を兼ねる (#646) ため、両者は同じ入力パイプラインを通る。優先順位は高い順に:

1. **CLI フラグ** (`--user` / `--plugin key=value` など)
2. **`--config <path>.yaml`** で読み込んだ値
3. **`INPUT_<UPPER>` 環境変数 / `INPUTS` JSON** (Action ランナーが `action.yml` から生成)
4. **`metadata.yml` の `default`**

`INPUT_<KEY>` は入力名を大文字化し `.` を `_` に置換したもの (例: `config_timezone` → `INPUT_CONFIG_TIMEZONE`)。`--no-env` を渡すと env レイヤをスキップし、CLI フラグのみで解決する。

> upstream にあった `config_presets` (`--preset`) と一括 `--plugins` フラグは現在の実装には存在しない。プラグインは `plugin_<slug>` で個別に有効化する。

## 2. 入力の命名体系

| 接頭辞                   | 意味                                 | 例                                                                                                                                             |
| ------------------------ | ------------------------------------ | ---------------------------------------------------------------------------------------------------------------------------------------------- |
| `plugin_<slug>`          | プラグインの有効化                   | `plugin_languages`, `plugin_achievements`                                                                                                      |
| `plugin_<slug>_<option>` | プラグインのオプション               | `plugin_achievements_threshold`, `plugin_activity_limit`, `plugin_calendar_limit`                                                              |
| `chrome_<section>`       | カード枠のセクションゲート (boolean) | `chrome_header`, `chrome_activity`, `chrome_community`, `chrome_repositories`, `chrome_metadata`, `chrome_introduction`                        |
| `config_<name>`          | 全体設定                             | `config_display`, `config_animations`, `config_timezone`, `config_base64`, `config_order`, `config_octicon`, `config_output`, `config_padding` |

> `chrome_*` の「chrome」は **カードの枠 (UI 用語)** で、ブラウザとは無関係。upstream の `base=<csv>` / `plugin_base_<section>` 形式は v3.0 (#649 / #652) で廃止され、受理されない。

その他の主要な core 入力 (`assets/plugins/core/metadata.yml`): `token` (GitHub PAT), `user`, `repo`, `template`, `filename`, `output_dir`, `combined`, `output_action` (commit / pull-request), `output_condition` (`data-changed` 等), `committer_*`, `repositories_skip_private`, `optimize`, `debug_flags`, `github_api_rest` / `github_api_graphql` (GitHub Enterprise 用エンドポイント)。

## 3. CLI フラグ

`internal/action/cli.go` (`ParseFlags`) が受け付けるフラグ:

| フラグ                          | 対応する入力                | 備考                                          |
| ------------------------------- | --------------------------- | --------------------------------------------- |
| `--user <login>`                | `user`                      |                                               |
| `--template <name>`             | `template`                  | 既定 `classic`                                |
| `--repo <name>`                 | `repo`                      | `--template=repository` で必須                |
| `--plugin key=value`            | 任意の入力                  | 繰り返し可。最も強い上書き                    |
| `--output svg\|png\|jpeg\|json` | `config_output`             | 既定 `svg`                                    |
| `--filename <path>`             | `filename`                  | `-` で stdout。指定すると `--combined` を含意 |
| `--output-dir <dir>`            | `output_dir`                | プラグイン別 SVG 出力 (既定モード)            |
| `--combined`                    | `combined`                  | 単一 SVG モード                               |
| `--skip-private-repo`           | `repositories_skip_private` | 全プラグインでプライベートリポジトリを除外    |
| `--config <path>.yaml`          | ([§4](#4---config-yaml))    |                                               |
| `--dryrun`                      | `dryrun`                    | commit / PR の副作用を抑止                    |
| `--no-env`                      | —                           | env レイヤを無視                              |

ユーザーが明示的に渡したフラグ (`fs.Visit`) のみが env 値を上書きする。例えば `INPUT_TOKEN=secret metrics-cli --dryrun` では `INPUT_TOKEN` はそのまま残る。

## 4. `--config` YAML

`--config` は `action.yml` 相当の入力を YAML ファイルで渡す手段 (`LoadYAMLConfig`)。階層化された `plugins:` / `config:` / `committer:` マップはフラットなキー空間へ展開される:

```yaml
output: svg # → config_output
config:
  timezone: Asia/Tokyo # → config_timezone
plugins:
  languages: yes # → plugin_languages
  languages_indepth: yes # → plugin_languages_indepth
committer:
  message: "..." # → committer_message
```

トップレベルのキーはそのまま渡される。

## 5. 型の正規化

`config.NormalizeInput(def, raw)` が `metadata.yml` の `type` に従って値を変換する (`internal/config/inputs.go`):

| `type`    | 規則                                                                                                                                                       |
| --------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `boolean` | `yes` / `true` / `on` / `1` → true。`no` / `false` / `off` / `0` → false。空文字は default にフォールバック。それ以外は false (trim + lower-case して判定) |
| `number`  | `strconv.ParseFloat`。`min` / `max` で clamp                                                                                                               |
| `string`  | trim                                                                                                                                                       |
| `array`   | `format` (`comma-separated` / `space-separated` / `newline-separated`) に従って split し、trim + 重複除去                                                  |
| `token`   | 秘匿文字列 (`Token` 型)                                                                                                                                    |
| `json`    | `json.Unmarshal`。空は `null`                                                                                                                              |

## 6. 動的プレースホルダ

`default: .user.login` のように `.` で始まる値は、base データ取得後に実データで置換される (`dataResolver`)。対応するプレースホルダ:

- `.user.login`
- `.user.name`
- `.repository.name`
- `.repository.full_name`

## 7. metadata.yml と action.yml

- 各プラグイン / テンプレートの入力定義は `assets/plugins/<slug>/metadata.yml` にあり、`//go:embed` でバンドルされる。`config.MetadataLoader` が起動時に 1 回ロードする。
- `action.yml` はこれらの `metadata.yml` から自動生成される (`internal/tools/gen-action-yml`)。lefthook の `action-yml-drift` フックがコミットごとに再生成し、ずれがあれば失敗する。再生成手順は [`CONTRIBUTING.md`](../CONTRIBUTING.md) を参照。

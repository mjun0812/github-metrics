# 採用スコープと決定事項

本プロジェクトは upstream [`lowlighter/metrics`](https://github.com/lowlighter/metrics) (Node.js / EJS) の **subset** のみを Go に移植したものである。本書は「何を採用し、何を採用しなかったか」の確定した決定事項を記録する。CLAUDE.md はこの文書を採用判断の source of truth として参照する。

新機能に着手する前に、対象が採用スコープに含まれるかを必ず本書で確認すること。採用外機能の混入は `tests/compliance/compliance_test.go` が CI で検出する ([§4](#4-compliance-ゲート) 参照)。

---

## 1. 起動形態

- **GitHub Action** と **CLI** の 2 形態のみ採用。
- **Web インスタンスは不採用** ([§3.1](#31-web-インスタンス-旧-m5))。

単一バイナリ (`cmd/metrics-cli`) が Action と CLI の両方を兼ねる。

## 2. 採用プラグイン

共有データ取得は `internal/dataprovider` の lazy / memoized Provider に集約されており、各プラグインはそこからユーザー / リポジトリのデータを読む。

### 2.1 基盤プラグイン (常設・ギャラリー対象外)

| slug   | 役割                                                                                                     |
| ------ | -------------------------------------------------------------------------------------------------------- |
| `core` | 設定 (`config_*`) の注入と、各プラグインの並列実行ランナー                                               |
| `base` | activity+community / repositories のサマリーパネル。`dataprovider` からのみ読む opt-in プラグイン (#625) |

> 旧 `base` プラグイン (共有データ取得を兼ねていたもの) は #605 で解体され、データ取得は `dataprovider` に、識別カードは `header` プラグイン (#602) に、サマリーパネルは #625 で再導入された現行 `base` に分離された。

### 2.2 ユーザー可視プラグイン (19)

README ギャラリーと `docs/plugins/*.md` に対応する 19 プラグイン:

`header`, `languages` (`recent` / `indepth` サブモード付き), `activity`, `achievements`, `repositories`, `isocalendar`, `calendar`, `habits`, `stars`, `people`, `notable`, `contributors`, `reactions`, `sponsors`, `sponsorships`, `stargazers`, `traffic`, `topics`, `starlists`

`header` は識別情報カード (アバター / 名前 / 統計) を描画する opt-in プラグイン。SVG 埋め込み時に個別に含める / 除外できる (#602)。

各プラグインの入出力・必要スコープは [`docs/plugins/`](plugins/) の自動生成ページを参照。データ取得方法 (REST / GraphQL / HTML スクレイピング / ローカル計算) の一覧は [`github-api-data-sources.md`](github-api-data-sources.md) を参照。

> `projects` プラグインは一度採用されたが、未使用のため #670 で廃止した。

## 3. 採用しなかった機能 (実装禁止)

### 3.1 Web インスタンス (旧 M5)

HTTP サーバ / OAuth / Insights HTML / rate limiter / control endpoint など、HTTP でメトリクスを公開する機能一式。

**不採用の理由**: 運用コストが高く、README バッジ運用には不要と判断した (Action と CLI で目的を満たせる)。

`internal/templates/` に `terminal` / `markdown` などを追加することも、この決定に抵触するため禁止 (`TestCompliance_M7_TemplateInvariant` が gating)。

### 3.2 ソーシャル / 外部 API プラグイン (旧 M8)

ユーザーが外部サービスのトークン / 鍵を用意する必要があるプラグイン群。全数不採用:

`wakatime`, `pagespeed`, `posts`, `rss`, `stackoverflow`, `leetcode`, `anilist`, `music`, `steam`, `tweets`,
`crypto`, `nightscout`, `stock`, `chess`, `splatoon`, `fortune`, `poopmap`, `screenshot`, `16personalities`

**不採用の理由**: 外部サービス依存が増えメンテナンスコストに見合わない。GitHub データを扱う本来の目的から外れる。

### 3.3 その他の不採用

| 機能                                                                              | 不採用の理由                                                        |
| --------------------------------------------------------------------------------- | ------------------------------------------------------------------- |
| `lines` / `gists` / `followup` / `discussions` / `skyline` / `support` プラグイン | 必要性が低い、API レート消費が大きい、または upstream で deprecated |
| `code` プラグイン                                                                 | プライベートコード漏洩リスク                                        |
| Markdown / Markdown-PDF 出力                                                      | 需要が低く、PDF 化はブラウザ依存を再導入する                        |
| community プラグイン / テンプレートの動的取得                                     | 安全性・メンテナンスコスト                                          |
| SVGO 相当の SVG 最適化                                                            | CSS purge + XML 整形で十分                                          |

採用する出力形式は **SVG / PNG / JPEG / JSON** のみ。詳細は [`rendering.md`](rendering.md) を参照。

## 4. compliance ゲート

以下の自動テストが採用スコープを CI で強制する。落ちた場合は採用外機能が混入した合図であり、即対処する:

- `TestCompliance_M4_AdoptedPlugins`: `internal/plugins/` のディレクトリ集合が採用プラグイン一覧と厳密一致すること。
- `TestNoUnadoptedPluginReference`: 不採用プラグイン slug が production code (コメントを除く) に混入していないこと。
- `TestCompliance_M7_TemplateInvariant`: `internal/templates/` が `classic` / `repository` のみであること。
- `TestCompliance_DocsPluginPagesMatchAdoptedSet`: `docs/plugins/*.md` がユーザー可視 19 プラグインと一致すること。
- `TestNoRemovedSentinelComments`: `// removed:` 系の削除履歴コメントを書かないこと。

## 5. 完了履歴

| マイルストン | 内容                                                                                                                                          | リリース            |
| ------------ | --------------------------------------------------------------------------------------------------------------------------------------------- | ------------------- |
| M1–M10       | upstream からの Go 移植 MVP (基盤 → classic / JSON → レンダリング → 19 プラグイン → Action/CLI → repository テンプレート → テスト基盤 → 配布) | v1.0.0 (2026-05-18) |
| #409         | chromedp / Chromium を排除し、native SVG + resvg のブラウザレスなレンダリングへ全面移行                                                       | v4.0.0 (2026-07-09) |

移植フェーズの当初順序は `M1 → M2 → M3 → M4 → M6 → M7 → M9 → M10` (M5 / M8 は不採用のため欠番)。#409 の意思決定ログは issue #409 と sub issue #682–#695 に記録されている。

# specs — Go 移植向け仕様書

[lowlighter/metrics](https://github.com/lowlighter/metrics) (Node.js) を **Go** へ全面的に書き換えるための詳細仕様書群です。各ドキュメントは独立して読めるよう自己完結に書いてありますが、`00-overview.md` から順に読むのが最短経路です。

## 読む順

1. **[00-overview.md](./00-overview.md)** — プロジェクト全体概要・移植スコープ・非機能要件
2. **[01-architecture.md](./01-architecture.md)** — Go パッケージ構成・ランタイム依存・データフロー
3. **[02-action.md](./02-action.md)** — GitHub Action / CLI バイナリ仕様
4. **[03-web-server.md](./03-web-server.md)** — Web サーバ API・rate-limit・OAuth
5. **[04-rendering.md](./04-rendering.md)** — SVG/PDF/PNG/JSON/Markdown 生成パイプライン
6. **[05-plugins.md](./05-plugins.md)** — プラグインインタフェースと registry
7. **[06-plugins-detail.md](./06-plugins-detail.md)** — 各プラグインの入出力一覧
8. **[07-templates.md](./07-templates.md)** — テンプレート構造・partials
9. **[08-external-services.md](./08-external-services.md)** — GitHub/Twitter/Spotify/WakaTime 等外部 API
10. **[09-configuration.md](./09-configuration.md)** — settings.json / metadata.yml / preset 解釈
11. **[10-testing-deployment.md](./10-testing-deployment.md)** — テスト戦略・Docker・CI/CD
12. **[11-go-migration.md](./11-go-migration.md)** — Node → Go ライブラリ対応表・移植難所・フェーズ計画
13. **[12-tasks.md](./12-tasks.md)** — issue 化可能なタスク分解 (T-001〜T-132)
14. **[13-appendix.md](./13-appendix.md)** — base GraphQL クエリ全文・取得アルゴリズム・各種規則の付録
15. **[14-feature-selection.md](./14-feature-selection.md)** — 機能選定ガイド (カタログ・依存早見・選定ワークシート)
16. **[15-selection-answer.md](./15-selection-answer.md)** — 選定回答書テンプレート (記入式)
17. **[16-tasks-mvp.md](./16-tasks-mvp.md)** — MVP タスクリスト (選定済 77 + 追加 2 = 79 タスクを実装順に整理)

## このドキュメントの目的

- 既存 Node.js 実装の **挙動を 1:1 で文書化** し、Go で再実装する際の単一の情報源とする。
- 入力互換 (`action.yml`, `settings.json`) と JSON 出力互換を維持しつつ、内部実装の置換を可能にする。
- 移植中・移植後にレビュー時の比較基準となる。

## 既存実装との対応

| Node 版ファイル | 仕様書 |
|---------------|-------|
| `source/app/metrics/index.mjs` | [01](./01-architecture.md), [04](./04-rendering.md) |
| `source/app/metrics/setup.mjs` | [05](./05-plugins.md), [07](./07-templates.md), [09](./09-configuration.md) |
| `source/app/metrics/metadata.mjs` | [09](./09-configuration.md) |
| `source/app/metrics/presets.mjs` | [09](./09-configuration.md) §5 |
| `source/app/metrics/utils.mjs` | [04](./04-rendering.md), [08](./08-external-services.md) |
| `source/app/action/index.mjs`, `run.sh`, `action.yml` | [02](./02-action.md) |
| `source/app/web/instance.mjs` | [03](./03-web-server.md) |
| `source/plugins/base/*` | [05](./05-plugins.md) §5, [06](./06-plugins-detail.md) §1.1 |
| `source/plugins/core/*` | [05](./05-plugins.md) §6, [06](./06-plugins-detail.md) §1.2 |
| `source/plugins/<name>/*` | [06](./06-plugins-detail.md) §2〜§4 |
| `source/templates/*` | [07](./07-templates.md) |
| `tests/*` | [10](./10-testing-deployment.md) |
| `Dockerfile` | [10](./10-testing-deployment.md) §5 |

## 未確定項目

主な未解決事項は [11-go-migration.md §6.2](./11-go-migration.md#62-未解決事項-要追議論) を参照。
M1 開始前に決定したい項目を中心に列挙しています。

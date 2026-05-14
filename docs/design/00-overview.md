# 00. プロジェクト全体概要

## 目次

- [1. プロジェクトの目的](#1-プロジェクトの目的)
- [2. 既存実装(Node.js)の全体像](#2-既存実装nodejsの全体像)
- [3. Go移植の目的とスコープ](#3-go移植の目的とスコープ)
- [4. 非機能要件](#4-非機能要件)
- [5. 用語集](#5-用語集)
- [6. ドキュメント構成](#6-ドキュメント構成)

---

## 1. プロジェクトの目的

`metrics` は GitHub ユーザー / 組織 / リポジトリの統計情報を集約し、SVG / PNG / JPEG / JSON / Markdown / PDF として出力するインフォグラフィック生成ツールである。

主なユースケース:

1. **GitHub Action として** ユーザーの README に貼り付ける動的バッジ画像を生成する。
2. **Web インスタンスとして** Web サーバを立ち上げて URL 経由で SVG を返す。
3. **Insights モード** で対話型ダッシュボードを描画する。

40+ のプラグインによって表示要素 (言語比率、アクティビティ、Achievements、WakaTime、PageSpeed、トピック、リポジトリ一覧 など) を組み合わせられる。

## 2. 既存実装(Node.js)の全体像

- 言語: Node.js (ES Modules, `.mjs`)
- 主要依存:
  - HTTP: `express`, `compression`, `express-rate-limit`
  - GitHub API: `@octokit/graphql`, `@octokit/rest`
  - HTTP クライアント: `axios`
  - SVG レンダリング: 自作 + `ejs` (テンプレート), `svgo` (最適化), `puppeteer` (ブラウザレンダリング・スクリーンショット)
  - 画像処理: `sharp`, `gifencoder`
  - 言語解析: `linguist-js`
  - その他: `marked`, `prismjs`, `sanitize-html`, `js-yaml`, `simple-git`, `rss-parser`, `open-graph-scraper`
- 公式 Docker イメージは `ghcr.io/lowlighter/metrics` で配布される。

### 主要ディレクトリ

```
source/
├── app/
│   ├── action/        … GitHub Action entry (index.mjs, run.sh, action.yml)
│   ├── metrics/       … Core engine (index, setup, metadata, presets, utils)
│   └── web/           … Web instance (instance.mjs, statics/)
├── plugins/           … 40+ plugins (base, core, languages, activity, …)
└── templates/         … classic / repository / terminal / markdown / community
```

詳しくは [01-architecture.md](./01-architecture.md) を参照。

## 3. Go移植の目的とスコープ

### 3.1 移植の目的

| 項目 | 目的 |
|------|------|
| 起動速度 | Node.js + npm の依存解決を排し、シングルバイナリで瞬時に起動する |
| 配布性 | クロスコンパイルで多 OS / アーキテクチャに対応する |
| メンテナンス性 | 型システムによる早期の不整合検出、コード量の削減 |
| メモリ効率 | puppeteer 同居時のメモリ消費を抑える |

### 3.2 移植対象 (In Scope)

- GitHub Action entry (action.yml に対応する CLI)
- Web インスタンスの全エンドポイント
- すべてのコアプラグインと組み込みプラグイン (`base`, `core`, 主要 plugin 約 40 個)
- すべての公式テンプレート (classic, repository, terminal, markdown)
- SVG / PNG / JPEG / JSON / Markdown / PDF の各出力
- Insights モード
- 設定ファイル (`settings.json`) と `metadata.yml` ベースの自動入力解釈
- Docker イメージ配布

### 3.3 移植非対象 / 慎重に扱う項目 (Out of Scope / Deferred)

| 項目 | 取扱 |
|------|------|
| community plugins | 初版では取り込まず、外部プラグインの仕組みを残す |
| community templates | git clone による動的取得は v2 で対応 |
| ユーザー任意の `extras.css` / `extras.js` | 安全な範囲のみ |
| puppeteer の事前インストール | chromedp に置換 (Chrome は外部依存) |
| 旧 Node 固有のレガシー機能 | API ドキュメントから明示的に除外 |

詳細は [11-go-migration.md](./11-go-migration.md) を参照。

### 3.4 互換性

- 既存ユーザーの `action.yml` の input 名・既定値、`settings.json` のスキーマは **完全に踏襲** する。
- 生成される SVG / Markdown の構造はテンプレート単位で再現する (バイト同一は目指さない)。
- Insights JSON のスキーマは互換性を維持する。

## 4. 非機能要件

| 区分 | 要件 |
|------|------|
| 性能 | 1 ユーザーあたり SVG 生成 < 30 秒 (puppeteer 抜きで < 5 秒) |
| 並行性 | デフォルトで `GOMAXPROCS` を尊重し、リクエスト並列度は設定可能 |
| 可用性 | Web インスタンスは Docker 上で動作し、SIGTERM で graceful shutdown する |
| ログ | `slog` 構造化ログ、レベル `debug/info/warn/error` |
| 監視 | API レートリミット残量を `/.requests` で返却 |
| 言語 | ソース・コメントは英語、本仕様書は日本語 |
| ライセンス | MIT (既存と同じ) |

## 5. 用語集

| 用語 | 意味 |
|------|------|
| Plugin | 個別の統計情報を取得・整形するモジュール (例: languages, wakatime) |
| Template | プラグインの出力を統合してレンダリングする視覚定義 (例: classic) |
| Partial | テンプレート内の個別セクション。`partials/<name>.ejs` で記述される |
| Base content | プラグインのうち、ユーザー情報・カレンダーなど共通データを準備する特殊プラグイン (`base`) |
| Core | 共通入力(`config.*`, `output_*`) を定義する特殊プラグイン (`core`) |
| Embed mode | 通常の SVG/Markdown 等の埋め込み画像出力モード |
| Insights mode | 対話型 HTML ダッシュボードを生成するモード |

## 6. ドキュメント構成

| ファイル | 内容 |
|---------|------|
| [00-overview.md](./00-overview.md) | 本書: プロジェクト全体概要 |
| [01-architecture.md](./01-architecture.md) | Go パッケージ構成、データフロー、依存関係 |
| [02-action.md](./02-action.md) | GitHub Action / CLI 仕様 |
| [03-web-server.md](./03-web-server.md) | HTTP API・rate limit・キャッシュ・OAuth |
| [04-rendering.md](./04-rendering.md) | SVG/PDF/PNG/JSON/Markdown 生成パイプライン |
| [05-plugins.md](./05-plugins.md) | プラグインインタフェース・登録機構 |
| [06-plugins-detail.md](./06-plugins-detail.md) | 各プラグインの入出力一覧 |
| [07-templates.md](./07-templates.md) | テンプレート構造・partials・EJS 相当の置換 |
| [08-external-services.md](./08-external-services.md) | GitHub / Twitter / Spotify / WakaTime 等外部 API |
| [09-configuration.md](./09-configuration.md) | settings.json / metadata.yml / 入力解釈 |
| [10-testing-deployment.md](./10-testing-deployment.md) | テスト戦略・Docker・CI/CD |
| [11-go-migration.md](./11-go-migration.md) | Node→Go の置換表、移植上の難所と対策 |

# lowlighter/metrics から github-metrics (Go 移植) への移行ガイド

**対象バージョン**: `mjun0812/github-metrics` v1.0.0
**最終更新**: 2026-05-18

## 1. 概要

本プロジェクトは [`lowlighter/metrics`](https://github.com/lowlighter/metrics)
を Go に移植したものです。upstream の全機能ではなく、
[`docs/design/15-selection-answer.md`](./design/15-selection-answer.md)
で採用判断した **21 plugin + 2 template + 4 出力形式** の subset
のみを対象としています。

**このガイドの対象読者**: 現在 upstream `lowlighter/metrics` を
使用しており、Go 移植版 (採用 subset) への移行を検討している方。

**移行コストの大前提** (constitution 原則 I / II):

- **入力互換性**: 採用機能の `with:` input 名・既定値・型は
  upstream と完全互換 (`uses:` 行だけを差し替えれば動作)。
- **出力互換性**: JSON は byte 互換、SVG は **DOM 構造単位**で
  upstream と同等 (バージョン文字列・生成時刻の差分は許容)。

## 2. 採用機能一覧

### 2.1 Plugin (21)

| 種別     | 名前              | upstream slug    | 注記                                |
| -------- | ----------------- | ---------------- | ----------------------------------- |
| Core     | base              | base             | account-kind dispatcher (内部)      |
| Core     | core              | core             | settings + parallel runner (内部)   |
| MVP (P1) | languages         | languages        | `recent` / `indepth` サブモード対応 |
| MVP (P1) | activity          | activity         |                                     |
| MVP (P1) | achievements      | achievements     |                                     |
| MVP (P1) | repositories      | repositories     |                                     |
| MVP (P1) | isocalendar       | isocalendar      |                                     |
| P2       | calendar          | calendar         |                                     |
| P2       | habits            | habits           |                                     |
| P2       | stars             | stars            |                                     |
| P2       | people            | people           | followers / following               |
| P2       | notable           | notable          |                                     |
| P2       | contributors      | contributors     |                                     |
| P2       | reactions         | reactions        |                                     |
| P2       | projects          | projects         |                                     |
| P2       | sponsors          | sponsors         |                                     |
| P2       | sponsorships      | sponsorships     |                                     |
| P2       | stargazers        | stargazers       |                                     |
| P2       | traffic           | traffic          |                                     |
| P3       | topics            | topics           | chromedp scrape                     |
| P3       | starlists         | starlists        | chromedp scrape                     |

### 2.2 Template (2)

| 種別     | 名前       | 注記                                              |
| -------- | ---------- | ------------------------------------------------- |
| Template | classic    | M2 adopted, 21 plugin partials filtered to subset |
| Template | repository | M7 adopted, `--user <owner> --repo <name>` 中心   |

### 2.3 出力形式 (4)

| 形式 | CLI flag        | wired in | 備考                          |
| ---- | --------------- | -------- | ----------------------------- |
| SVG  | `--output svg`  | M1       | デフォルト                    |
| JSON | `--output json` | M2       | upstream byte 互換            |
| PNG  | `--output png`  | M3       | chromedp `CaptureScreenshot`  |
| JPEG | `--output jpeg` | M3       | chromedp `CaptureScreenshot`  |

詳細根拠は
[`docs/design/15-selection-answer.md`](./design/15-selection-answer.md)
§2-§4 を参照。

## 3. 未対応機能一覧

下記は本 Go 移植では **実装されていません**。upstream で動作している
機能がここに含まれる場合、移行は推奨しません。

### 3.1 Runtime / Mode

| 種別 | 名前              | 不採用理由                       | 参照                                       |
| ---- | ----------------- | -------------------------------- | ------------------------------------------ |
| Mode | Web インスタンス (M5) | 運用コスト過大                 | [15-selection-answer.md §1](./design/15-selection-answer.md) |
| Mode | OAuth / Insights HTML | M5 web 一体・別途認証基盤必要 | 同上                                       |

### 3.2 Template (community 系)

| 名前      | 不採用理由                       |
| --------- | -------------------------------- |
| terminal  | 互換性価値 low、用途特殊          |
| markdown  | Markdown / PDF 出力と一体        |
| community | community 拡張機構未採用          |

### 3.3 GitHub data 系 plugin (6)

| slug         | 不採用理由                        |
| ------------ | --------------------------------- |
| lines        | M、REST stats 多用 — 後続候補     |
| gists        | S、優先度低                       |
| followup     | S、優先度低                       |
| discussions  | S、優先度低                       |
| skyline      | L、重い・3D city/skyline          |
| support      | S、upstream 終了済 (deprecated)   |

### 3.4 community 拡張 plugin (3)

| slug         | 不採用理由               |
| ------------ | ------------------------ |
| code         | community 拡張機構経由   |
| introduction | community 拡張機構経由   |
| licenses     | community 拡張機構経由   |

### 3.5 ソーシャル / 外部 API 系 plugin (19, M8 全数不採用)

| slug            | サイズ感 | 不採用理由 |
| --------------- | -------- | ---------- |
| anilist         | S        | 外部 API   |
| chess           | -        | community  |
| chessdotcom     | -        | community  |
| crypto          | -        | community  |
| fortune         | -        | community  |
| leetcode        | S        | 外部 API   |
| music           | L        | OAuth 複雑 |
| nightscout      | -        | community  |
| pagespeed       | S        | 外部 API   |
| poopmap         | -        | community  |
| posts           | S        | 外部 API   |
| rss             | S        | 任意 URL   |
| screenshot      | -        | community  |
| splatoon        | -        | community  |
| stackoverflow   | S        | 外部 API   |
| steam           | M        | 外部 API   |
| stock           | -        | community  |
| tweets          | -        | deprecated |
| wakatime        | M        | 外部 API   |
| 16personalities | -        | community  |

(注: 上記には `chessdotcom` も含めて 19 slug を網羅。実際の `chess`
パッケージ名差異は upstream の plugin ディレクトリに準拠)

### 3.6 出力形式

| 名前     | 不採用理由                                          |
| -------- | --------------------------------------------------- |
| pdf      | upstream は Puppeteer; chromedp 経由は複雑度割に合わず |
| markdown | community template と一体                           |

詳細根拠は
[`docs/design/15-selection-answer.md`](./design/15-selection-answer.md)
§3-§4 を参照。

## 4. 入力互換性

constitution 原則 I より:

- **採用 input** (上表 2.x の plugin / template / output 関連入力)
  は upstream と **完全互換**。key 名・既定値・型は同一。
- **未対応 input** (上表 3.x に該当する plugin の `plugin_<slug>`
  ゲート、および付帯入力) は **silently no-op**。ワークフロー
  ファイルは変更不要のままで動作し、未対応 plugin の出力だけが
  生成されません。
- 不正な input 名や型の場合でも **ハードエラーにしない** (constitution
  原則 I MUST NOT)。upstream と同じく未知 key は素通し。

### 4.1 動作例

下記の workflow は **そのまま** Go 移植版で動作します:

```yaml
- uses: mjun0812/github-metrics@v1.0.0
  with:
    user: octocat
    plugin_languages: yes              # 採用 → 出力に反映
    plugin_languages_limit: '5'         # 採用 → 反映
    plugin_anilist: yes                 # 未対応 → silently no-op
    plugin_anilist_medias: anime        # 未対応 → silently no-op
```

実行結果: SVG に languages パネルが描画され、anilist パネルは
含まれません。警告もエラーも発生しません。

## 5. 移行手順

### Step 1: `uses:` 行を差し替え

```diff
- uses: lowlighter/metrics@v3.34
+ uses: mjun0812/github-metrics@v1.0.0
```

### Step 2: (任意) 未対応 input を削除

未対応の `plugin_*` ゲートを workflow から削除すると可読性が
向上します。**必須ではありません** (§4 の silently no-op が
保証されているため)。

### Step 3: 再実行

`workflow_dispatch` でワークフローをトリガーするか、次回の
scheduled run を待ちます。`output_action: commit` 等の出力ア
クションも upstream と同じ semantics で動作します。

### Step 4: 出力検証

- **JSON 出力**: constitution 原則 II より byte 互換。
  `diff old.json new.json` が空であることを確認できます。
- **SVG 出力**: DOM 構造単位での同等性。バージョン文字列
  (`github-metrics@vX.Y.Z`) や生成時刻 (`Last updated ...`)
  は差分が出ます。構造を確認するには `xmllint --format` で
  整形してから diff してください。

## 6. ロールバック

`uses:` 行を 1 行戻すだけで完全ロールバックできます (採用 input
は drop-in 互換のため、設定ファイル側の調整は不要です):

```diff
- uses: mjun0812/github-metrics@v1.0.0
+ uses: lowlighter/metrics@v3.34
```

commit / push で完了。upstream の挙動が完全に復元されます。

---

**Source of truth**:

- 採用判断: [`docs/design/15-selection-answer.md`](./design/15-selection-answer.md)
- Input 互換性: [`.specify/memory/constitution.md`](../.specify/memory/constitution.md) 原則 I
- 出力互換性: [`.specify/memory/constitution.md`](../.specify/memory/constitution.md) 原則 II
- 採用 phase 順序: [`CLAUDE.md`](../CLAUDE.md) Adopted phase order

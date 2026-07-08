# lowlighter/metrics から github-metrics (Go 移植) への移行ガイド

**対象バージョン**: `mjun0812/github-metrics` v1.0.0
**最終更新**: 2026-05-24

## 1. 概要

本プロジェクトは [`lowlighter/metrics`](https://github.com/lowlighter/metrics)
を Go に移植したものです。upstream の全機能を網羅するのではなく、
よく使われる **21 plugin + 2 template + 4 出力形式** に絞った subset
を提供しています。

**このガイドの対象読者**: 現在 upstream `lowlighter/metrics` を
使用しており、Go 移植版への移行を検討している方。

**移行コストの大前提**:

- **入力互換性**: サポート対象の `with:` input 名・既定値・型は
  upstream と完全互換 (`uses:` 行だけを差し替えれば動作します)。
- **出力互換性**: JSON は upstream とバイト互換、SVG は **DOM 構造単位** で
  upstream と同等です (バージョン文字列・生成時刻の差分は許容)。

## 2. サポート対象機能

### 2.1 Plugin (21)

すべての plugin は GitHub の API トークンと通常の HTTP 取得だけで
動作します。`topics` / `starlists` は upstream では Headless Chromium で
ページをスクレイプしますが、本移植では goquery による HTML パースに
置き換えているためブラウザは不要です。

| 名前              | upstream slug    | 注記                                |
| ----------------- | ---------------- | ----------------------------------- |
| base              | base             | プロファイル基本情報 (内部使用)     |
| core              | core             | 設定注入 + 並列実行 (内部使用)      |
| languages         | languages        | `recent` / `indepth` サブモード対応 |
| activity          | activity         |                                     |
| achievements      | achievements     |                                     |
| repositories      | repositories     | Featured / Pinned / Starred / Random |
| isocalendar       | isocalendar      | 3D 等尺カレンダー                   |
| calendar          | calendar         | 多年カレンダー                      |
| habits            | habits           | 曜日 / 時間帯傾向                   |
| stars             | stars            | 最近スターしたリポジトリ            |
| people            | people           | フォロワー / フォロイング           |
| notable           | notable          |                                     |
| contributors      | contributors     | repository テンプレート向け         |
| reactions         | reactions        | リアクション集計                    |
| projects          | projects         | GitHub Projects (`read:project` 必要) |
| sponsors          | sponsors         | (`read:user` / `read:org` 必要)     |
| sponsorships      | sponsorships     | (`read:user` / `read:org` 必要)     |
| stargazers        | stargazers       | 累積 star チャート                  |
| traffic           | traffic          | 閲覧数 (`repo` 必要)                |
| topics            | topics           | HTML スクレイプ (goquery)           |
| starlists         | starlists        | HTML スクレイプ (goquery)           |

### 2.2 Template (2)

| 名前       | 注記                                              |
| ---------- | ------------------------------------------------- |
| classic    | ユーザー / 組織向けのデフォルトテンプレート       |
| repository | リポジトリ単体メトリクス (`--user <owner> --repo <name>`) |

### 2.3 出力形式 (4)

| 形式 | CLI flag        | 備考                          |
| ---- | --------------- | ----------------------------- |
| SVG  | `--output svg`  | デフォルト                    |
| JSON | `--output json` | upstream とバイト互換         |
| PNG  | `--output png`  | resvg でネイティブ SVG をラスタライズ |
| JPEG | `--output jpeg` | resvg PNG を Go で JPEG 再エンコード |

> **レンダリングにブラウザは不要です。**
> upstream は最終 SVG の高さ計測と PNG/JPEG 出力に Headless Chromium
> (puppeteer) を使いますが、本移植では高さを Go 側のフォントメトリクスで
> 計算し、PNG/JPEG は [resvg](https://github.com/linebender/resvg) で
> ラスタライズします。この結果 chromium を Docker image から外し、
> distro のパッケージ更新でブラウザが起動しなくなる障害クラスを
> 構造的に排除しました (経緯は
> [#409](https://github.com/mjun0812/github-metrics/issues/409))。

## 3. 未対応機能一覧

下記は本 Go 移植では **実装されていません**。upstream で動作している
機能がここに含まれる場合、移行は推奨しません。

### 3.1 Runtime / Mode

| 種別 | 名前                  | 不採用理由                                |
| ---- | --------------------- | ----------------------------------------- |
| Mode | Web インスタンス      | 運用コスト過大 (Action / CLI のみサポート) |
| Mode | OAuth / Insights HTML | Web インスタンス前提 (上記とともに不採用) |

### 3.2 Template (community 系)

| 名前      | 不採用理由                       |
| --------- | -------------------------------- |
| terminal  | 互換性価値 low、用途特殊          |
| markdown  | Markdown / PDF 出力と一体        |
| community | community 拡張機構未採用          |

### 3.3 GitHub data 系 plugin (6)

| slug         | 不採用理由                        |
| ------------ | --------------------------------- |
| lines        | REST stats を多用するため負荷大   |
| gists        | 優先度低                          |
| followup     | 優先度低                          |
| discussions  | 優先度低                          |
| skyline      | 3D city / skyline — 重く優先度低  |
| support      | upstream 終了済 (deprecated)      |

### 3.4 community 拡張 plugin (3)

| slug         | 不採用理由               |
| ------------ | ------------------------ |
| code         | community 拡張機構経由   |
| introduction | community 拡張機構経由   |
| licenses     | community 拡張機構経由   |

### 3.5 ソーシャル / 外部 API 系 plugin (19)

| slug            | サイズ感 | 不採用理由 |
| --------------- | -------- | ---------- |
| anilist         | S        | 外部 API   |
| chess           | -        | community  |
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

### 3.6 出力形式

| 名前     | 不採用理由                                          |
| -------- | --------------------------------------------------- |
| pdf      | upstream は Puppeteer でブラウザ描画 — ブラウザ非依存の本移植では対象外 |
| markdown | community template と一体のため                    |

## 4. 入力互換性

- **サポート対象 input** (上表 §2 の plugin / template / output 関連入力)
  は upstream と **完全互換** です。key 名・既定値・型は同一。
- **未対応 input** (§3 に該当する plugin の `plugin_<slug>` ゲートおよび
  付帯入力) は **silently no-op** です。ワークフローファイルは変更不要
  のままで動作し、未対応 plugin の出力だけが生成されません。
- 不正な input 名や型でも **ハードエラーにしません**。upstream と同じく
  未知 key は素通しします。

### 4.1 動作例

下記の workflow は **そのまま** Go 移植版で動作します:

```yaml
- uses: mjun0812/github-metrics@v1
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
+ uses: mjun0812/github-metrics@v1
```

`@v1` は最新の v1.x.y リリースに自動追従する floating tag です
(patch / minor 更新が出ても workflow の変更不要)。バイト単位で
ピン留めしたい場合は `@v1.0.0` 等の exact `vX.Y.Z` 形式を使えます。

### Step 2: (任意) 未対応 input を削除

未対応の `plugin_*` ゲートを workflow から削除すると可読性が
向上します。**必須ではありません** (§4 の silently no-op が
保証されているため)。

### Step 3: 再実行

`workflow_dispatch` でワークフローをトリガーするか、次回の
scheduled run を待ちます。`output_action: commit` 等の出力ア
クションも upstream と同じ semantics で動作します。

### Step 4: 出力検証

- **JSON 出力**: upstream とバイト互換のため、
  `diff old.json new.json` が空であることを確認できます。
- **SVG 出力**: DOM 構造単位での同等性。バージョン文字列
  (`github-metrics@vX.Y.Z`) や生成時刻 (`Last updated ...`)
  は差分が出ます。構造を確認するには `xmllint --format` で
  整形してから diff してください。

## 6. ロールバック

`uses:` 行を 1 行戻すだけで完全ロールバックできます (サポート対象 input
は drop-in 互換のため、設定ファイル側の調整は不要です):

```diff
- uses: mjun0812/github-metrics@v1
+ uses: lowlighter/metrics@v3.34
```

commit / push で完了。upstream の挙動が完全に復元されます。

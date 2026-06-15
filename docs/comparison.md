# upstream ↔ Go実装 出力比較 (plugin parity)

各 plugin について、**左 = upstream `lowlighter/metrics` の公式出力 (lowlighter データ)**、**中 = upstream を `mjun0812` データで実行した参照出力**、**右 = 本リポジトリ Go 実装の出力 (`mjun0812` データ)** を 3 列で並べて、レイアウト・DOM 構造のパリティを目視確認するためのページです。

- 左 (upstream / lowlighter): [`docs/original_examples/`](original_examples/) — `lowlighter/metrics` の `examples` ブランチ (commit `1dac69e`)。lowlighter 本人のプロフィールデータをレンダリングした正規サンプル。出所詳細は [original_examples/README.md](original_examples/README.md)。
- 中 (upstream / mjun0812): [`docs/reference_examples/`](reference_examples/) — upstream `lowlighter/metrics@v3.34` を **`mjun0812` 本人のデータ**で実行した参照出力。右の Go 実装と**同一データ**のため、データを揃えた状態での見た目・DOM 差分 (apples-to-apples) を確認できます。生成方法・欠番の理由は [reference_examples/README.md](reference_examples/README.md)。
- 右 (Go 実装 / mjun0812): [`docs/examples/`](examples/) — 本リポジトリのサンプル出力。`make docs-samples` (= `scripts/gen-doc-samples.sh`) で `mjun0812` のデータからレンダリング。

## 凡例 / 注意

- ⚠ **左列だけはデータソースが異なるため数値・項目は一致しません**（lowlighter vs mjun0812）。中列と右列は同一データ (`mjun0812`) なので、項目レベルで一致するはずです。確認対象は「枠・配色・要素の配置・DOM 構造」が upstream と同等か（= parity）です。
- 🖼️ **すべてのサンプルはサイズに関わらず `<img>` で埋め込んでいます**（数 MB のファイルも含む）。GitHub 上での表示は重くなる場合がありますが、リンク化せず全カードを並べる方針です。
- 一部の SVG は `<foreignObject>` (HTML 埋め込み) を含み、`<img>` 経由だとブラウザ / GitHub 上で HTML 部分が描画されないことがあります。崩れて見える場合はファイルを直接開いて確認してください。
- 表示幅は `width="420"` で統一しています（原寸は各ファイル参照）。
- **空カードについて**: 中列 (reference) と右列 (Go 実装) はともに `mjun0812` のデータを使うため、本人に該当データが無い plugin（topics / starlists / sponsors 等）は枠だけの空表示になります。これは未実装ではなく「サンプルユーザーにデータが無い」ためです。
- **中列の欠番 (— 該当なし)**: 中列の参照カードは upstream v3.34 のエラー・API 廃止・既知バグで生成できなかったものがあります（achievements / activity / habits / projects / languages.recent 等）。各セクションに理由を明記しています。詳細は [reference_examples/README.md](reference_examples/README.md) の「正しく描画できなかったカード」を参照。
- **repository mode**: M7 で実装した `--template repository` のサンプルは「base chrome 単体」と「単体 toggle で base chrome と byte 差を生む plugin」のみを `plugin-<slug>-repo.svg` / `.png` として生成しています。対象リポジトリは `mjun0812/flash-attention-prebuild-wheels`（owner 本人なので traffic plugin も実データを取得）。詳細・除外理由 (byte-identical / user-mode-only / partial 未登録) は本ページ末尾の [「repository mode サンプル一覧」](#repository-mode-サンプル一覧) を参照。

## variant（サブモード）の Go 実装対応状況

upstream に存在する各 plugin のサブモードについて、Go 実装の対応状況です。

| upstream variant                 | Go 側                  | 備考                                                                                                                                           |
| -------------------------------- | ---------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------- |
| `languages.recent`               | ✅ 比較可              | `plugin-languages-recent.svg`                                                                                                                  |
| `languages.indepth`              | ✅ 比較可              | `plugin-languages-indepth.svg`                                                                                                                 |
| `languages.details`              | ✅ 比較可              | `plugin-languages-details.svg`（本対応で追加）                                                                                                 |
| `achievements.compact`           | ✅ 比較可              | `plugin-achievements-compact.svg`（本対応で追加）                                                                                              |
| `isocalendar.fullyear`           | ✅ 比較可              | `plugin-isocalendar-fullyear.svg`（本対応で追加）                                                                                              |
| `calendar.full`                  | ◐ 実装済み・生成待ち   | `plugin_calendar_limit=0` で全期間を取得。サンプルは次回 `make docs-samples` で生成                                                            |
| `repositories.pinned`            | ◐ partial 未配線       | `plugin_repositories_pinned=yes` を受け付けるが partial が variant を反映せず、無印と同じ Featured set (stars 降順) を描画する。follow-up 予定 |
| `topics.icons`                   | ○ データ無し           | `plugin_topics_mode=icons` 対応。サンプルユーザーに topics 無しで空                                                                            |
| `starlists.languages`            | ○ データ無し           | `plugin_starlists_languages` 対応。サンプルユーザーにデータ無しで空                                                                            |
| `sponsors.full`                  | ○ データ無し           | `plugin_sponsors_sections` 対応。サンプルユーザーにデータ無しで空                                                                              |
| `habits.facts` / `habits.charts` | ✅ 比較可              | `plugin_habits_facts` / `plugin_habits_charts` で個別トグル。<br>`plugin-habits-facts.svg` / `plugin-habits-charts.svg`                        |
| `notable.indepth`                | ✅ 比較可              | `plugin_notable_indepth` 対応。`@owner/repo` 粒度チップ + 統計ゲージ。<br>`plugin-notable-indepth.svg`                                         |
| `contributors.contributions`     | ✅ 比較可（repo mode） | `plugin_contributors_contributions` 対応。<br>`plugin-contributors-repo-contributions.svg` で adds/dels 列を表示                               |
| `stargazers.graph`               | ✅ 比較可              | `plugin_stargazers_charts_type=graph` 対応。<br>`plugin-stargazers-graph.svg`                                                                  |
| `stargazers.worldmap`            | ✗ 未対応               | backlog（Google Maps API、R-012 Skipped path）                                                                                                 |
| `stargazers.chartist`            | ◐ graph と同一         | `charts_type=chartist` は deprecated alias。`graph` とバイト同一出力                                                                           |
| `people.repository`              | ✅ 比較可（repo mode） | contributors / stargazers / watchers 対応。<br>`plugin-people-repo-types.svg` で 3 種同時表示                                                  |

✅ = 左右比較可 / ◐ = Go は対応するがサンプル差分なし・または repository 専用 / deprecated alias でサンプル省略 / ○ = Go は対応するがサンプルユーザーにデータ無し（空） / ✗ = Go 実装が未対応（要実装・backlog）

---

## テンプレート / base

upstream の参考表示と Go 実装側の対応サンプルを並べています。`classic 総合` は upstream の `metrics.classic.svg` (= 実質 base ヘッダのみ) に揃え、Go 側も plugin トグルなしの classic/base 出力を表示します ([#463](https://github.com/mjun0812/github-metrics/issues/463): 以前は主要 12 plugin を合成した縦長 overview だったため、upstream の見た目と一致せず ~3200px に肥大化していた)。各採用 plugin の個別パネルは下のプラグイン別サンプルで確認できます。`repository 総合` は `--template repository --repo mjun0812/flash-attention-prebuild-wheels` の **base 出力** で、upstream `metrics.repository.svg` と apples-to-apples になるよう plugin トグルなしの base chrome のみを描画します (issue #464)。repository `base.header` partial がリポジトリ名 + `Created` / `Deployed` / disk-usage / 貢献カレンダー / `Environments` を出します。

| 種別               | upstream (lowlighter)                                            | upstream (mjun0812)                                               | Go 実装 (mjun0812)                                      |
| ------------------ | ---------------------------------------------------------------- | ----------------------------------------------------------------- | ------------------------------------------------------- |
| base (plugin なし) | <img src="original_examples/metrics.base.svg" width="420">       | <img src="reference_examples/metrics.base.svg" width="420">       | <img src="examples/plugin-base.svg" width="420">        |
| classic 総合       | <img src="original_examples/metrics.classic.svg" width="420">    | <img src="reference_examples/metrics.classic.svg" width="420">    | <img src="examples/metrics-classic.svg" width="420">    |
| repository 総合    | <img src="original_examples/metrics.repository.svg" width="420"> | <img src="reference_examples/metrics.repository.svg" width="420"> | <img src="examples/metrics-repository.svg" width="420"> |

> ✅ Go サンプル `metrics-classic.svg` / `.png` は `scripts/gen-doc-samples.sh` / `scripts/samples.json` で `base=header, repositories` を指定して生成します。upstream `metrics.classic.svg` が実質 header + repositories であるため、本サンプルもその 2 セクションのみの出力となります。採用 plugin の個別パネルは下のプラグイン別サンプルを参照してください。
> ✅ Go サンプル `metrics-repository.svg` / `.png` は同スクリプトの repository mode セクションで生成。`mjun0812/flash-attention-prebuild-wheels` を対象に、plugin トグルなしの **base repository 出力** を描画します (issue #464)。upstream `metrics.repository.svg` と同じく base chrome (repository 名 + `Created` / `Deployed` / disk-usage / 貢献カレンダー / `Environments`) のみで、個別 plugin partial は repository template でも `plugin_<slug>` トグルで gate されます。各 plugin partial 単体のサンプルは `plugin-base-repo` / `plugin-people-repo` / `plugin-contributors-repo-contributions` / `plugin-people-repo-types` 側でカバーします。

### base partial parity (`base.header` / `base.repositories`)

`base.header.ejs` / `base.repositories.ejs` で upstream が描画する各フィールドの本リポジトリ実装状況です。両 partial は全 60+ サンプル SVG が embed する基盤のため、ここのカバレッジが上がると全サンプルの見た目が一斉に改善します。詳細は [#429](https://github.com/mjun0812/github-metrics/issues/429) (Phase 1〜3 完了 — 採用範囲 14 / 14 フィールド達成)。

| partial             | フィールド                           | 状態         | データソース                                                           |
| ------------------- | ------------------------------------ | ------------ | ---------------------------------------------------------------------- |
| `base.header`       | Avatar + display name                | ✅ 実装済み  | `user.{avatarUrl,name,login}`                                          |
| `base.header`       | Joined GitHub `<age>`                | ✅ Phase 1   | `user.createdAt`                                                       |
| `base.header`       | Followed by N users                  | ✅ Phase 1   | `user.followers.totalCount`                                            |
| `base.header`       | Following N users                    | ✅ Phase 1   | `user.following.totalCount`                                            |
| `base.header`       | Contributed to N repositories        | ✅ Phase 2   | `user.repositoriesContributedTo.totalCount`                            |
| `base.header`       | Contribution calendar 11×7 mini grid | ✅ Phase 3   | `user.contributionsCollection.contributionCalendar.weeks` (末尾 11 週) |
| `base.repositories` | N repositories                       | ✅ 実装済み  | `Computed.Repositories.Count`                                          |
| `base.repositories` | N stargazers                         | ✅ 実装済み  | `Computed.Repositories.Stargazers`                                     |
| `base.repositories` | N forks                              | ✅ 実装済み  | `Computed.Repositories.Forks`                                          |
| `base.repositories` | Watching N repositories              | ✅ Phase 1   | `user.watching.totalCount`                                             |
| `base.repositories` | N sponsors                           | ✅ Phase 1   | `user.sponsorshipsAsMaintainer.totalCount`                             |
| `base.repositories` | N releases                           | ✅ Phase 2   | `repository.releases.totalCount` の合算                                |
| `base.repositories` | N packages                           | ✅ Phase 2   | `repository.packages.totalCount` の合算                                |
| `base.repositories` | `<disk-usage>` used                  | ✅ Phase 2   | `repository.diskUsage` (KB) の合算                                     |
| `base.repositories` | License preference (top 3)           | ✅ Phase 2   | `repository.licenseInfo` の集計 + 上位 N                               |
| n/a (out of scope)  | `+N added` / `-N removed`            | ✗ 永久対象外 | M8 不採用の `lines` plugin (`docs/design/15-selection-answer.md` §1)   |

> Phase 1 / Phase 2 の octicon は中立な `<svg class="octicon"></svg>` placeholder です。upstream と同等のアイコン path 形状は別 issue で扱います (Phase 3 は contribution mini grid のみが対象)。
> Phase 2 の License preference は `licenseInfo` を返した repo のみを母数として集計するため、ライセンス未設定 repo が混在する場合でも 100% に正規化されます (上位 3 件表示)。
> Phase 3 の contribution mini grid は upstream `base.header.ejs` が描画する単列ストリップを GitHub プロフィール準拠の 11 列 × 7 行に拡張したものです。各セルは GraphQL が返した hex 色をそのまま `fill` に埋め込み、加えて `calendar-graph-day-<level>` クラスを付与するためテーマ CSS (`--color-calendar-graph-day-Ln-bg`) もそのまま適用されます。データは `runUser` が常時取得するため、`calendar` / `isocalendar` 等の indepth-tier plugin を有効化しなくても base 単体で表示されます。

#### Layout / 縦長の根本対応 (`fix/base-layout-vertical-shrink`)

`#429` の Phase 1〜3 を merge した後も `plugin-base.svg` の実 HTML 高さが upstream の 5〜6 倍 (約 1824 px vs 325 px) になっていた件への根本対応です。原因と修正:

- **原因**: `<svg class="octicon"></svg>` placeholder が `width` / `height` / `viewBox` のいずれも持たないため、Chrome がデフォルトの SVG ビューポート (300 × 150 px) を採用していました。CSS で `.field { display: flex; align-items: center }` を当てているため、各 `<div class="field">` 行が **150 px** 高さに膨張し、これが upstream の `width="16" height="16"` 付き octicon (17 px 行高) と比較して 9 倍弱の縦長を生んでいました。Phase 1 前 (#427 時点) で既に upstream の 2.3 倍 (743 px) だったのもこの一点で説明できます。
- **なぜ CSS では効かないか**: SVG の intrinsic size は CSS プロパティではなく **要素属性**（`width` / `height` / `viewBox`）で決まります。SVG の仕様上、CSS の `width`/`height` は SVG 要素が外部コンテナ内に配置された後の "box size" には影響しますが、`foreignObject` 内の HTML レンダリングにおいて Chrome が参照する "intrinsic size" は属性で確定します。そのため CSS の `.octicon { width: 16px; height: 16px }` を追加しても Chrome の SVG デフォルトビューポート (300×150 px) の採用は止まりませんでした。
- **真の修正**: `partials.go` 内の全 octicon placeholder 出力箇所 (17 箇所) を `octiconPlaceholder` 定数 (`<svg xmlns="http://www.w3.org/2000/svg" class="octicon" width="16" height="16" viewBox="0 0 16 16"/>`) 経由に変更し、属性で intrinsic size を直接 16×16 に固定。これにより `.field` 行高が ~150 px → ~17 px に縮小します。
- **CSS rule は保険として残留**: `assets/templates/classic/style.css` に追加した `.octicon { width: 16px; height: 16px }` は実質 no-op ですが、将来的に属性なし SVG が混入した際のフォールバックとして保持します。
- **before / after** (plugin-base.svg trim height):

  | sample              | before  | after   |
  | ------------------- | ------- | ------- |
  | plugin-base.svg     | 1824 px | 243 px  |
  | metrics-classic.svg | n/a     | 2190 px |
  | plugin-people.svg   | n/a     | 646 px  |
  | plugin-activity.svg | n/a     | 1176 px |

  > 注: 上表の `metrics-classic.svg` (2190 px) は当時の 12 plugin 合成 overview の値です。[#463](https://github.com/mjun0812/github-metrics/issues/463) で `metrics-classic` は plugin トグルなしの base-only 出力に戻したため、現行サンプルは upstream `metrics.classic.svg` 同様 ~200px です。

- **副作用**: License preference 行が `License preference: MIT License 50% / Apache License 2.0 22% / BSD 3-Clause "New" or "Revised" License 6%` だと 480 px キャンバス上で 758 px 幅まで overflow していたため、SPDX 短縮形式 + middle dot に圧縮 (`MIT 50% · Apache-2.0 22% · BSD-3-Clause 6%`)。`licenseShortName()` で SPDX マッピング。
- **波及**: 全 35 sample (svg + png 約 70 ファイル) が `make docs-samples` で再生成されます。upstream parity が破壊されていないことは `tests/golden/*` と `tests/visual/*` の差分なし通過で担保。

---

## plugin 比較

### languages

| upstream (lowlighter)                                                  | upstream (mjun0812)                                                     | Go 実装 (`plugin-languages.svg`)                      |
| ---------------------------------------------------------------------- | ----------------------------------------------------------------------- | ----------------------------------------------------- |
| <img src="original_examples/metrics.plugin.languages.svg" width="420"> | <img src="reference_examples/metrics.plugin.languages.svg" width="420"> | <img src="examples/plugin-languages.svg" width="420"> |

**variant: details** — upstream `languages.details` ↔ Go `plugin-languages-details.svg`

| upstream (lowlighter)                                                          | upstream (mjun0812)                                                             | Go 実装                                                       |
| ------------------------------------------------------------------------------ | ------------------------------------------------------------------------------- | ------------------------------------------------------------- |
| <img src="original_examples/metrics.plugin.languages.details.svg" width="420"> | <img src="reference_examples/metrics.plugin.languages.details.svg" width="420"> | <img src="examples/plugin-languages-details.svg" width="420"> |

### languages.recent

| upstream (lowlighter)                                                         | upstream (mjun0812)                 | Go 実装 (`plugin-languages-recent.svg`)                      |
| ----------------------------------------------------------------------------- | ----------------------------------- | ------------------------------------------------------------ |
| <img src="original_examples/metrics.plugin.languages.recent.svg" width="420"> | — 該当なし（upstream v3.34 のバグ） | <img src="examples/plugin-languages-recent.svg" width="420"> |

> ⚠ 中列 (mjun0812): upstream v3.34 の `recent.mjs:70` が `mjun0812` のデータで例外 (`Array.filter`) を投げて生成不可。詳細は [reference_examples/README.md](reference_examples/README.md)。

### languages.indepth

| upstream (lowlighter)                                                          | upstream (mjun0812)                                                             | Go 実装 (`plugin-languages-indepth.svg`)                      |
| ------------------------------------------------------------------------------ | ------------------------------------------------------------------------------- | ------------------------------------------------------------- |
| <img src="original_examples/metrics.plugin.languages.indepth.svg" width="420"> | <img src="reference_examples/metrics.plugin.languages.indepth.svg" width="420"> | <img src="examples/plugin-languages-indepth.svg" width="420"> |

### activity

| upstream (lowlighter)                                                 | upstream (mjun0812)                 | Go 実装 (`plugin-activity.svg`)                      |
| --------------------------------------------------------------------- | ----------------------------------- | ---------------------------------------------------- |
| <img src="original_examples/metrics.plugin.activity.svg" width="420"> | — 該当なし（upstream v3.34 のバグ） | <img src="examples/plugin-activity.svg" width="420"> |

> ⚠ 中列 (mjun0812): upstream v3.34 のコードバグ (`TypeError: Cannot read properties of undefined (reading 'filter')`) で `Unexpected error` が描画されたため削除。

### achievements

| upstream (lowlighter)                                                     | upstream (mjun0812)                     | Go 実装                                                  |
| ------------------------------------------------------------------------- | --------------------------------------- | -------------------------------------------------------- |
| <img src="original_examples/metrics.plugin.achievements.svg" width="420"> | — 該当なし（Projects classic API 廃止） | <img src="examples/plugin-achievements.svg" width="420"> |

**variant: compact** — upstream `achievements.compact` ↔ Go `plugin-achievements-compact.svg`

| upstream (lowlighter)                                                             | upstream (mjun0812)                     | Go 実装                                                          |
| --------------------------------------------------------------------------------- | --------------------------------------- | ---------------------------------------------------------------- |
| <img src="original_examples/metrics.plugin.achievements.compact.svg" width="420"> | — 該当なし（Projects classic API 廃止） | <img src="examples/plugin-achievements-compact.svg" width="420"> |

> ✅ Go 側は `plugin_achievements_display=compact` に対応。Go サンプルは `scripts/gen-doc-samples.sh` で `plugin-achievements-compact.svg` / `.png` として生成。
> ⚠ 中列 (mjun0812): GitHub が Projects (classic) API を廃止 ([2024-05 sunset](https://github.blog/changelog/2024-05-23-sunset-notice-projects-classic/)) したため、achievements が内部で classic projects を参照して GraphQL `NOT_FOUND` → `Unexpected error`。compact も同様で削除。

### repositories

| upstream (lowlighter)                                                     | upstream (mjun0812)                                                        | Go 実装 (`plugin-repositories.svg`)                      |
| ------------------------------------------------------------------------- | -------------------------------------------------------------------------- | -------------------------------------------------------- |
| <img src="original_examples/metrics.plugin.repositories.svg" width="420"> | <img src="reference_examples/metrics.plugin.repositories.svg" width="420"> | <img src="examples/plugin-repositories.svg" width="420"> |

**variant: pinned** — upstream `repositories.pinned`

| upstream (lowlighter)                                                            | upstream (mjun0812)                                                               | Go 実装                                              |
| -------------------------------------------------------------------------------- | --------------------------------------------------------------------------------- | ---------------------------------------------------- |
| <img src="original_examples/metrics.plugin.repositories.pinned.svg" width="420"> | <img src="reference_examples/metrics.plugin.repositories.pinned.svg" width="420"> | — 別サンプルなし（partial が pinned variant 未対応） |

> ◐ Go 実装は現状 partial が pinned variant の選択を反映しないため、`plugin_repositories_pinned=yes` を渡しても無印と同じ Featured set (stars 降順) を描画する。partial 未対応分は follow-up issue で対処予定。

### isocalendar

| upstream (lowlighter)                                                    | upstream (mjun0812)                                                       | Go 実装 (`plugin-isocalendar.svg`)                      |
| ------------------------------------------------------------------------ | ------------------------------------------------------------------------- | ------------------------------------------------------- |
| <img src="original_examples/metrics.plugin.isocalendar.svg" width="420"> | <img src="reference_examples/metrics.plugin.isocalendar.svg" width="420"> | <img src="examples/plugin-isocalendar.svg" width="420"> |

**variant: fullyear** — upstream `isocalendar.fullyear` ↔ Go `plugin-isocalendar-fullyear.svg`

| upstream (lowlighter)                                                             | upstream (mjun0812)                                                                | Go 実装                                                          |
| --------------------------------------------------------------------------------- | ---------------------------------------------------------------------------------- | ---------------------------------------------------------------- |
| <img src="original_examples/metrics.plugin.isocalendar.fullyear.svg" width="420"> | <img src="reference_examples/metrics.plugin.isocalendar.fullyear.svg" width="420"> | <img src="examples/plugin-isocalendar-fullyear.svg" width="420"> |

### calendar

| upstream (lowlighter)                                                 | upstream (mjun0812)                                                    | Go 実装 (`plugin-calendar.svg`)                      |
| --------------------------------------------------------------------- | ---------------------------------------------------------------------- | ---------------------------------------------------- |
| <img src="original_examples/metrics.plugin.calendar.svg" width="420"> | <img src="reference_examples/metrics.plugin.calendar.svg" width="420"> | <img src="examples/plugin-calendar.svg" width="420"> |

**variant: full** — upstream `calendar.full`

| upstream (lowlighter)                                                      | upstream (mjun0812)                                                         | Go 実装                                                          |
| -------------------------------------------------------------------------- | --------------------------------------------------------------------------- | ---------------------------------------------------------------- |
| <img src="original_examples/metrics.plugin.calendar.full.svg" width="420"> | <img src="reference_examples/metrics.plugin.calendar.full.svg" width="420"> | — 生成待ち（`GITHUB_TOKEN` 付き `make docs-samples` で生成予定） |

> ◐ Go 側は `plugin_calendar_limit=0` でアカウント作成年から現在年までを年単位で再取得する。`scripts/samples.json` / `scripts/gen-doc-samples.sh` には `plugin-calendar-full` を追加済みだが、この環境では `GITHUB_TOKEN` が未設定のため SVG/PNG は未生成。

### habits

| upstream (lowlighter, charts)                                              | upstream (mjun0812)                 | Go 実装 (`plugin-habits.svg`)                      |
| -------------------------------------------------------------------------- | ----------------------------------- | -------------------------------------------------- |
| <img src="original_examples/metrics.plugin.habits.charts.svg" width="420"> | — 該当なし（upstream v3.34 のバグ） | <img src="examples/plugin-habits.svg" width="420"> |

**variant: facts / charts** — upstream `habits.facts` / `habits.charts` ↔ Go `plugin-habits-facts.svg` / `plugin-habits-charts.svg`

| upstream (lowlighter, facts)                                              | upstream (mjun0812)                 | Go 実装 (facts)                                          |
| ------------------------------------------------------------------------- | ----------------------------------- | -------------------------------------------------------- |
| <img src="original_examples/metrics.plugin.habits.facts.svg" width="420"> | — 該当なし（upstream v3.34 のバグ） | <img src="examples/plugin-habits-facts.svg" width="420"> |

| upstream (lowlighter, charts)                                              | upstream (mjun0812)                 | Go 実装 (charts)                                          |
| -------------------------------------------------------------------------- | ----------------------------------- | --------------------------------------------------------- |
| <img src="original_examples/metrics.plugin.habits.charts.svg" width="420"> | — 該当なし（upstream v3.34 のバグ） | <img src="examples/plugin-habits-charts.svg" width="420"> |

> ✅ Go 側は `plugin_habits_facts` / `plugin_habits_charts` で facts / charts を個別トグル。Go サンプル: `plugin-habits-facts.svg`（facts のみ）/ `plugin-habits-charts.svg`（charts のみ）。
> ⚠ 中列 (mjun0812): upstream v3.34 のコードバグ (`TypeError: Cannot destructure property 'author' of 'undefined'`、`habits/index.mjs:51`) で charts / facts とも `Unexpected error` になり削除。

### stars

| upstream (lowlighter)                                              | upstream (mjun0812)                                                 | Go 実装 (`plugin-stars.svg`)                      |
| ------------------------------------------------------------------ | ------------------------------------------------------------------- | ------------------------------------------------- |
| <img src="original_examples/metrics.plugin.stars.svg" width="420"> | <img src="reference_examples/metrics.plugin.stars.svg" width="420"> | <img src="examples/plugin-stars.svg" width="420"> |

### topics

| upstream (lowlighter)                                               | upstream (mjun0812)                                                  | Go 実装 (`plugin-topics.svg`)                      |
| ------------------------------------------------------------------- | -------------------------------------------------------------------- | -------------------------------------------------- |
| <img src="original_examples/metrics.plugin.topics.svg" width="420"> | <img src="reference_examples/metrics.plugin.topics.svg" width="420"> | <img src="examples/plugin-topics.svg" width="420"> |

**variant: icons** — upstream `topics.icons`

| upstream (lowlighter)                                                     | upstream (mjun0812)                                                        | Go 実装                                                |
| ------------------------------------------------------------------------- | -------------------------------------------------------------------------- | ------------------------------------------------------ |
| <img src="original_examples/metrics.plugin.topics.icons.svg" width="420"> | <img src="reference_examples/metrics.plugin.topics.icons.svg" width="420"> | — 別サンプルなし（サンプルユーザーに topics 無しで空） |

> ○ Go 側は `plugin_topics_mode=icons` 対応だが、サンプルユーザーに topics 無しで空表示。
> 実装メモ: upstream は puppeteer で `/stars/<user>/topics` をスクレイピングするが、Go 実装は公開 SSR ページを HTTP + goquery で取得する (chromium 不要)。

### starlists

| upstream (lowlighter)                                                  | upstream (mjun0812)                                                     | Go 実装 (`plugin-starlists.svg`)                      |
| ---------------------------------------------------------------------- | ----------------------------------------------------------------------- | ----------------------------------------------------- |
| <img src="original_examples/metrics.plugin.starlists.svg" width="420"> | <img src="reference_examples/metrics.plugin.starlists.svg" width="420"> | <img src="examples/plugin-starlists.svg" width="420"> |

**variant: languages** — upstream `starlists.languages`

| upstream (lowlighter)                                                            | upstream (mjun0812) | Go 実装                                                     |
| -------------------------------------------------------------------------------- | ------------------- | ----------------------------------------------------------- |
| <img src="original_examples/metrics.plugin.starlists.languages.svg" width="420"> | — 該当なし          | — 別サンプルなし（サンプルユーザーに starlists データ無し） |

> ○ Go 側は `plugin_starlists_languages` 対応だが、サンプルユーザーにデータ無しで空表示。
> 実装メモ: upstream は puppeteer で `/stars/<user>/lists` をスクレイピングするが、Go 実装は GitHub GraphQL の `user.lists` を 1 クエリで取得する (chromium 不要)。

### people

| upstream (lowlighter, followers)                                              | upstream (mjun0812)                                                  | Go 実装 (`plugin-people.svg`)                      |
| ----------------------------------------------------------------------------- | -------------------------------------------------------------------- | -------------------------------------------------- |
| <img src="original_examples/metrics.plugin.people.followers.svg" width="420"> | <img src="reference_examples/metrics.plugin.people.svg" width="420"> | <img src="examples/plugin-people.svg" width="420"> |

**variant: repository** — upstream `people.repository` ↔ Go repository mode

| upstream (lowlighter, repository)                                              | upstream (mjun0812, repo)                                                       | Go 実装 (repo mode)                                           |
| ------------------------------------------------------------------------------ | ------------------------------------------------------------------------------- | ------------------------------------------------------------- |
| <img src="original_examples/metrics.plugin.people.repository.svg" width="420"> | <img src="reference_examples/metrics.repository.plugin.people.svg" width="420"> | <img src="examples/plugin-people-repo-types.svg" width="420"> |

> ✅ Go 側は user mode (followers/following) に加え repository context types (contributors/stargazers/watchers) を実装済み。repository mode のサンプルは `plugin-people-repo.svg`（既定の stargazers + watchers）と `plugin-people-repo-types.svg`（3 種同時 = stargazers + watchers + contributors）。詳細は本ページ末尾の [repository mode サンプル一覧](#repository-mode-サンプル一覧) を参照。

### notable

| upstream (lowlighter)                                                | upstream (mjun0812)                                                   | Go 実装 (`plugin-notable.svg`)                      |
| -------------------------------------------------------------------- | --------------------------------------------------------------------- | --------------------------------------------------- |
| <img src="original_examples/metrics.plugin.notable.svg" width="420"> | <img src="reference_examples/metrics.plugin.notable.svg" width="420"> | <img src="examples/plugin-notable.svg" width="420"> |

**variant: indepth** — upstream `notable.indepth` ↔ Go `plugin-notable-indepth.svg`

| upstream (lowlighter)                                                        | upstream (mjun0812)                                                           | Go 実装                                                     |
| ---------------------------------------------------------------------------- | ----------------------------------------------------------------------------- | ----------------------------------------------------------- |
| <img src="original_examples/metrics.plugin.notable.indepth.svg" width="420"> | <img src="reference_examples/metrics.plugin.notable.indepth.svg" width="420"> | <img src="examples/plugin-notable-indepth.svg" width="420"> |

> ✅ Go 側は `plugin_notable_indepth` 対応済み。indepth は基本モードの組織単位 (`@org`) ではなくリポジトリ単位 (`@org/repo`) のチップを描画し、commits / stars / issues / pulls の統計ゲージを付与する（upstream `notable.ejs` と同一 DOM）。Go サンプル: `plugin-notable-indepth.svg`。

### contributors

> upstream (lowlighter) の出力は両バリアントとも巨大 (9.9 / 8.7 MB) です。本ページの方針に従い `<img>` で埋め込んでいますが、GitHub 上での表示は重くなります。中列 (mjun0812) は repository mode のサンプル (`metrics.repository.plugin.contributors.svg`) を並べています。
>
> `contributors` は **repository mode 専用** plugin です (user mode では `internal/plugins/modegate.go::RequireRepoMode` により Skipped となり空カードになる)。そのため Go 実装列は user mode の空カードではなく repo mode サンプル `plugin-contributors-repo-contributions.svg` を代表サンプルとして掲載しています (#448)。

| upstream (lowlighter, categories)                                                    | upstream (mjun0812, repo)                                                             | Go 実装 (`plugin-contributors-repo-contributions.svg`, repo mode)           |
| ------------------------------------------------------------------------------------ | ------------------------------------------------------------------------------------- | --------------------------------------------------------------------------- |
| <img src="original_examples/metrics.plugin.contributors.categories.svg" width="420"> | <img src="reference_examples/metrics.repository.plugin.contributors.svg" width="420"> | <img src="examples/plugin-contributors-repo-contributions.svg" width="420"> |

**variant: contributions** — upstream `contributors.contributions` ↔ Go `plugin-contributors-repo-contributions.svg`

| upstream (lowlighter, contributions)                                                    | upstream (mjun0812, repo)                                                             | Go 実装 (`plugin-contributors-repo-contributions.svg`, repo mode)           |
| --------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------- | --------------------------------------------------------------------------- |
| <img src="original_examples/metrics.plugin.contributors.contributions.svg" width="420"> | <img src="reference_examples/metrics.repository.plugin.contributors.svg" width="420"> | <img src="examples/plugin-contributors-repo-contributions.svg" width="420"> |

> ✅ Go 側は `plugin_contributors_contributions` 対応済み（per-contributor commits / additions / deletions）。default mode の repository サンプルは base chrome の contributors セクションが担当し、adds/dels 列付きの変種は `plugin-contributors-repo-contributions.svg` を参照（既定 commits のみのサンプルは `plugin-base-repo.svg` と byte 同一になるため削除済み）。`/stats/contributors` が 202 / 空で返った場合は `/repos/{owner}/{repo}/contributors` にフォールバックし、commit 数付きチップを描画する。詳細は末尾の [repository mode サンプル一覧](#repository-mode-サンプル一覧) を参照。

### reactions

| upstream (lowlighter)                                                  | upstream (mjun0812)                                                     | Go 実装 (`plugin-reactions.svg`)                      |
| ---------------------------------------------------------------------- | ----------------------------------------------------------------------- | ----------------------------------------------------- |
| <img src="original_examples/metrics.plugin.reactions.svg" width="420"> | <img src="reference_examples/metrics.plugin.reactions.svg" width="420"> | <img src="examples/plugin-reactions.svg" width="420"> |

### projects

| upstream (lowlighter)                                                 | upstream (mjun0812)                     | Go 実装 (`plugin-projects.svg`)                      |
| --------------------------------------------------------------------- | --------------------------------------- | ---------------------------------------------------- |
| <img src="original_examples/metrics.plugin.projects.svg" width="420"> | — 該当なし（Projects classic API 廃止） | <img src="examples/plugin-projects.svg" width="420"> |

> ⚠ 中列 (mjun0812): GitHub が Projects (classic) API を廃止 ([2024-05 sunset](https://github.blog/changelog/2024-05-23-sunset-notice-projects-classic/)) したため、`projects` プラグインが GraphQL `NOT_FOUND` → `Unexpected error` で削除。

### sponsors

| upstream (lowlighter)                                                 | upstream (mjun0812)                                                    | Go 実装 (`plugin-sponsors.svg`)                      |
| --------------------------------------------------------------------- | ---------------------------------------------------------------------- | ---------------------------------------------------- |
| <img src="original_examples/metrics.plugin.sponsors.svg" width="420"> | <img src="reference_examples/metrics.plugin.sponsors.svg" width="420"> | <img src="examples/plugin-sponsors.svg" width="420"> |

**variant: full** — upstream `sponsors.full`

| upstream (lowlighter)                                                      | upstream (mjun0812) | Go 実装                                                    |
| -------------------------------------------------------------------------- | ------------------- | ---------------------------------------------------------- |
| <img src="original_examples/metrics.plugin.sponsors.full.svg" width="420"> | — 該当なし          | — 別サンプルなし（サンプルユーザーに sponsors データ無し） |

> ○ Go 側は `plugin_sponsors_sections=goal,about,list` で full 相当を表現可能だが、サンプルユーザーにデータ無しで空表示。

### sponsorships

| upstream (lowlighter)                                                     | upstream (mjun0812)                                                        | Go 実装 (`plugin-sponsorships.svg`)                      |
| ------------------------------------------------------------------------- | -------------------------------------------------------------------------- | -------------------------------------------------------- |
| <img src="original_examples/metrics.plugin.sponsorships.svg" width="420"> | <img src="reference_examples/metrics.plugin.sponsorships.svg" width="420"> | <img src="examples/plugin-sponsorships.svg" width="420"> |

### stargazers

| upstream (lowlighter)                                                   | upstream (mjun0812)                                                      | Go 実装 (`plugin-stargazers.svg`)                      |
| ----------------------------------------------------------------------- | ------------------------------------------------------------------------ | ------------------------------------------------------ |
| <img src="original_examples/metrics.plugin.stargazers.svg" width="420"> | <img src="reference_examples/metrics.plugin.stargazers.svg" width="420"> | <img src="examples/plugin-stargazers.svg" width="420"> |

**variant: graph** — upstream `stargazers.graph` ↔ Go `plugin-stargazers-graph.svg`

| upstream (lowlighter)                                                         | upstream (mjun0812)                                                            | Go 実装                                                      |
| ----------------------------------------------------------------------------- | ------------------------------------------------------------------------------ | ------------------------------------------------------------ |
| <img src="original_examples/metrics.plugin.stargazers.graph.svg" width="420"> | <img src="reference_examples/metrics.plugin.stargazers.graph.svg" width="420"> | <img src="examples/plugin-stargazers-graph.svg" width="420"> |

**variant: chartist / worldmap** — upstream `stargazers.chartist` / `stargazers.worldmap`

| variant  | upstream (lowlighter)                                                            | upstream (mjun0812) | Go 実装                                                      |
| -------- | -------------------------------------------------------------------------------- | ------------------- | ------------------------------------------------------------ |
| chartist | <img src="original_examples/metrics.plugin.stargazers.chartist.svg" width="420"> | — 該当なし          | — 別サンプルなし（`graph` の deprecated alias でバイト同一） |
| worldmap | <img src="original_examples/metrics.plugin.stargazers.worldmap.svg" width="420"> | — 該当なし          | — 未対応（Google Maps API 必須の backlog）                   |

> ✅ Go 側は `plugin_stargazers_charts_type=graph` 対応（`chartist` は graph の deprecated alias でバイト同一）。`graph` は累積 stargazers と直近 14 日の日次 new stargazers を上下 2 段で描画する。Go サンプル: `plugin-stargazers-graph.svg`。worldmap は Google Maps API 必須の backlog（Skipped path、未対応）。

### traffic

| upstream (lowlighter)                                                | upstream (mjun0812)                                                   | Go 実装 (`plugin-traffic.svg`)                      |
| -------------------------------------------------------------------- | --------------------------------------------------------------------- | --------------------------------------------------- |
| <img src="original_examples/metrics.plugin.traffic.svg" width="420"> | <img src="reference_examples/metrics.plugin.traffic.svg" width="420"> | <img src="examples/plugin-traffic.svg" width="420"> |

> ✅ Go 側: user mode は token owner が admin の全 repo に対して `/traffic/views` を並列取得し、`base.repositories` の右列に合計 views を統合表示する。traffic plugin 単体 partial は空文字列を返すため、`plugin-traffic.svg` は `base=repositories` と組み合わせて生成する。`--template repository` 単体では traffic partial が `_.json` に登録されていないため出力が `plugin-base-repo.svg` と byte 同一になり、専用サンプルは生成していない。admin 権限の有無で Run() が "missing repo scope" で Skipped になる挙動は user-mode サンプルでカバー済み。
> ⚠ 中列 (mjun0812): user テンプレートの `metrics.plugin.traffic.svg` は traffic セクション自体はデータ無しですが、カードは描画されるため保持しています。repository テンプレートの traffic は空 (高さ ~8px) だったため reference からは削除済み。

---

## repository mode サンプル一覧

`--template repository --user mjun0812 --repo flash-attention-prebuild-wheels` で生成した repository-mode サンプル一覧です。`scripts/gen-doc-samples.sh` の repository mode セクションで `make docs-samples` 実行時に自動再生成されます。

`--template repository` は `assets/templates/repository/partials/_.json` の固定 partial 順 (`base.header` → `introduction` → `followup` → `languages` → `projects` → `pagespeed` → `stargazers` → `people` → `activity` → `posts` → `rss` → `screenshot` → `stock` → `crypto` → `contributors` → `sponsors` → `licenses`) でレンダリングします。base 系セクション (`base.header` / `introduction` / `activity` / `contributors` / `languages`) は `base.runRepository` が常に値を populate するため、ユーザーが `--plugin <slug>=yes` を渡さなくても chrome として表示されます。

そのうえで、`--plugin <slug>=yes` トグルが追加の効果を持つ plugin は **`mjun0812/flash-attention-prebuild-wheels` で実測した限り** 以下に限定されます:

| 効果あり (repo mode で出力が変わる)                                                                                                                     | 効果なし (chrome と byte 同一になる)                                                                                                                                                                                                                                                                                                                            |
| ------------------------------------------------------------------------------------------------------------------------------------------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `people`<br>新規 `<section data-section="people">` を追加<br><br>`contributors_contributions`<br>chrome の contributors セクションに adds/dels 列を追加 | `achievements` / `activity` / `calendar` / `habits`<br>`isocalendar` / `notable` / `reactions` / `repositories`<br>`sponsorships` / `starlists` / `stars` / `topics`<br><br>`contributors` / `languages`: chrome 側ですでに描画<br>`projects` / `sponsors`: データ無し<br>`stargazers`: M7 MVP は totals のみで partial は空文字列<br>`traffic`: partial 未登録 |

「効果なし」側の plugin (18 個) は `plugin-<slug>-repo.svg` を生成しても `plugin-base-repo.svg` と byte 同一になります（md5 一致を実測で確認済）。これは未実装ではなく:

1. base 系セクションは plugin toggle 不要で常に描画される
2. partial が template の `_.json` にない / partial が空文字列 / 対象データが無い
3. plugin 自体が user mode 専用（mode gate により Skipped を返す）

のいずれかの理由による意図的な挙動で、upstream `metrics.repository.svg` でも同じ partial 集合のみが描画されます。

これらの byte 同一サンプル (`plugin-<slug>-repo.{svg,png}` × 18) は `docs/examples/` から削除しました。「網羅性のために並べる」よりも「意味のあるサンプルだけを残す」方が docs の信号雑音比が高いと判断しています。残った repository-mode サンプルは下の追加バリアント / 総合サンプル表のみで、ユーザーは「base chrome のみ」「people セクションを足すと何が増えるか」「contributors の per-row 列を足すと何が増えるか」を 1 ファイルずつ比較できます。

#### mode-not-supported warning

user mode 専用 plugin (14 個 = achievements / activity / calendar / habits / isocalendar / notable / projects / reactions / repositories / sponsors / sponsorships / starlists / stars / topics) を `--template repository` で起動すると、各 plugin の `Run()` は冒頭で `plugins.RequireUserMode()` を呼び、descriptive な reason をログに出して即 Skipped を返します。逆向きの contributors plugin（repository mode 専用）は `RequireRepoMode()` でユーザーモード時に同様の WARN を出します。

実行例:

```text
level=WARN msg="plugin achievements is only supported in user mode (current mode: repository)" plugin=achievements mode=repository supported=user
level=WARN msg="plugin contributors is only supported in repository mode (current mode: user)" plugin=contributors mode=user supported=repository
```

これにより operator は CI ログから「なぜこの plugin が空 SVG を返したか」を即特定できます。実装は `internal/plugins/modegate.go` 参照。

### 単体サンプル

| plugin | sample                            | 備考                                                                   |
| ------ | --------------------------------- | ---------------------------------------------------------------------- |
| base   | `examples/plugin-base-repo.svg`   | repository template chrome のみ（plugin toggle なし）                  |
| people | `examples/plugin-people-repo.svg` | `<section data-section="people">` 追加（既定 = stargazers + watchers） |

### 追加バリアント

| variant                             | sample                                                | 内容                                                   |
| ----------------------------------- | ----------------------------------------------------- | ------------------------------------------------------ |
| `contributors.contributions` (repo) | `examples/plugin-contributors-repo-contributions.svg` | `/stats/contributors` 経由で adds/dels 列を表示        |
| `people.types` (repo)               | `examples/plugin-people-repo-types.svg`               | stargazers + watchers + contributors を 1 カードで併記 |

### 総合サンプル

| sample                            | 内容                                                                                                                                                                                                                    |
| --------------------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `examples/metrics-repository.svg` | repository template の base 出力 (issue #464)。plugin トグルなしで upstream `metrics.repository.svg` と同じ base chrome のみ。<br>repository 名 + `Created` / `Deployed` / disk-usage / 貢献カレンダー / `Environments` |

### repository mode の upstream(mjun0812) ↔ Go 実装 比較

`mjun0812/flash-attention-prebuild-wheels` を `--template repository` で実行した、中列 (upstream / mjun0812 = reference_examples) と右列 (Go 実装) の比較です。同一リポジトリ・同一データなので apples-to-apples で比較できます。

| 種別            | upstream (mjun0812)                                                                   | Go 実装                                                                     |
| --------------- | ------------------------------------------------------------------------------------- | --------------------------------------------------------------------------- |
| repository 総合 | <img src="reference_examples/metrics.repository.svg" width="420">                     | <img src="examples/metrics-repository.svg" width="420">                     |
| languages       | <img src="reference_examples/metrics.repository.plugin.languages.svg" width="420">    | <img src="examples/plugin-base-repo.svg" width="420">                       |
| contributors    | <img src="reference_examples/metrics.repository.plugin.contributors.svg" width="420"> | <img src="examples/plugin-contributors-repo-contributions.svg" width="420"> |
| people          | <img src="reference_examples/metrics.repository.plugin.people.svg" width="420">       | <img src="examples/plugin-people-repo-types.svg" width="420">               |
| stargazers      | <img src="reference_examples/metrics.repository.plugin.stargazers.svg" width="420">   | <img src="examples/plugin-base-repo.svg" width="420">                       |

> ⚠ Go 実装側は repository mode の plugin 単体サンプルを「base chrome と byte 差が出るもの」だけに絞っているため、languages / stargazers のように chrome に統合される plugin は `plugin-base-repo.svg`（chrome 全体）を対応サンプルとして並べています。詳細は上記「repository mode サンプル一覧」を参照。
> ℹ️ activity / traffic は upstream(mjun0812) 側が v3.34 のバグ・データ不足で生成できず、reference からは削除済みのため本表から除外しています。

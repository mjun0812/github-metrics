# Phase 0 Research: classic + JSON 出力 (M2)

**Date**: 2026-05-15 | **Plan**: [plan.md](./plan.md) | **Spec**: [spec.md](./spec.md)

本書は M2 spec の Technical Context に含まれる未決定項目を解決する。各決定は **Decision / Rationale / Alternatives considered** の三項目で記述する。

## R-001: JSON シリアライザの選択

- **Decision**: 標準 `encoding/json` をベースに、`Marshal(*plugins.Data)` の外側に cycleDetector + Go コレクション型正規化レイヤを置く。
- **Rationale**: 標準 `encoding/json` はパフォーマンスもエコシステム親和性も十分。`set` / `map[K]V` の正規化と循環参照対応だけが独自要件で、これは `Marshal` 前に `any` ツリーへ変換する preprocess で吸収できる。サードパーティ JSON lib (`goccy/go-json`, `bytedance/sonic`) を入れるほどの perf 要件はない。
- **Alternatives considered**:
  - `goccy/go-json`: パフォーマンス優位だが、本 spec のスループット要件 (100 ms 内) は標準で達成可能。依存増を避ける。却下。
  - `bytedance/sonic`: 同上 + native CGO 依存があり配布バイナリ size を増やす。却下。

## R-002: 循環参照検出のアルゴリズム

- **Decision**: `reflect.Value` の `unsafe.Pointer` キーで訪問済みノードを追跡する `cycleDetector` を実装。最初の検出で当該ノードを `"[Circular]"` (string) に置換する。
- **Rationale**: `data.Plugins[name]` は plugin の自由形式 payload で、循環参照を含む可能性がある (例: 親 → 子 → 親 の参照)。`encoding/json` は循環で stack overflow するので、Marshal 前に DFS で循環を弾く必要がある。`reflect.Value.Pointer()` は addressable な値に対して安定キーになる。
- **Alternatives considered**:
  - `json.Marshaler` で各 plugin に自前実装させる: scope が広がりすぎる。Marshal 側で吸収する方が単純。
  - JSON Schema で循環構造を禁止: 上流互換性を壊す可能性があり (上流は循環を実際に許容しているケースが少なくとも `repository.owner.repositories` 系で存在)。

## R-003: `set` / `map[K]V` の JSON 正規化

- **Decision**: preprocess 段階で:
  - `map[string]any` → そのまま (Go と JSON で意味同等)
  - `map[K]V` (K != string) → `[]struct{Key, Value}` の配列
  - 集合的役割の slice (`set` 慣習で値が `any{}` の map) → ソート済み `[]string`
  - `[]T` → そのまま
- **Rationale**: 決定論的 golden file 比較が成立するため、map 系は **key でソート**、set 系は **value でソート**。上流 `lowlighter/metrics` の出力もこの規則と一致する (確認済み)。
- **Alternatives considered**:
  - `json.OrderedMap` 系の専用型導入: API 表面が増える。preprocess で正規化する方がシンプル。

## R-004: 上流 fixture (octocat.json) の取得方法

- **Decision**: `internal/tools/sync-fixtures/main.go` を実装し、`./org_repo` 内に置かれている `tests/cases/*.yml` から fixture を生成する。具体的には:
  1. 上流の `tests/cases/octocat.yml` (INPUTS) を読み込み
  2. 上流の Node 実装を `./org_repo` 内で `npm run test -- --update` 相当で実行して `metrics.json` を取得
  3. 取得結果を本リポジトリの `tests/fixtures/upstream/octocat.json` にコピー
- **Rationale**: 静的 vendor だと上流 schema 更新時に手動再取得が必要。`sync-fixtures` ツール化することでリプロが効く。`make sync-fixtures` ターゲットで一発実行できる。
- **Alternatives considered**:
  - 手動コピー: 一度きりは早いが、上流変更追従が辛い。却下。
  - GitHub Actions で nightly 取得: 過剰。手動 + `make` ターゲットで十分。

## R-005: SVG エスケープ戦略

- **Decision**: `html/template` の自動エスケープではなく、`internal/templates/classic/escape.go` に薄い `escapeXML(s string) string` を実装する (`<`/`>`/`&`/`'`/`"` を XML エンティティに変換)。partial は `strings.Builder` で組み立てる。
- **Rationale**: `html/template` は HTML5 の文脈エスケープを行うため、SVG 内の `<![CDATA[]]>` や数値参照 (`&#xE2;`) が壊れる。SVG/XML 用エスケープは 5 文字置換で済む単純な変換なので自前で実装。
- **Alternatives considered**:
  - `html/template` + カスタム funcMap: 文脈推論が SVG に最適化されていない。却下。
  - `text/template` + `escapeXML` funcMap: 候補だが、partial は条件分岐や数値フォーマッタ呼び出しが多く Go コードで書くほうが読みやすい。string concat 方針で行く。

## R-006: image.svg スケルトンの再解釈

- **Decision**: 上流 `assets/templates/classic/image.svg` の EJS 構文 (`<%= ... %>` / `<% ... %>`) を、Go 側で **手で構造化** する。EJS テンプレートを解釈する Go ライブラリは持ち込まない。`classic.go` 内に `renderSkeleton(data *plugins.Data, partials []string) string` を実装し、トップレベルの `<svg>`/`<defs>`/`<style>`/`<foreignObject>`/`<div class="items-wrapper">`/`<footer>` を直接出力する。partial 部分のみ動的に dispatch する。
- **Rationale**: EJS → Go テンプレートエンジン (`text/template`) への machine translation は精度が低く、テンプレ式に JS 機能 (`Object.entries`, async include) が混ざるため動かない箇所が出る。**手で構造を Go コードに移植** することで、テストカバレッジ + デバッグ容易性が両立する。
- **Alternatives considered**:
  - `text/template` で `image.svg` を直接消費: EJS と Go template の構文差異 (`<%= %>` vs `{{ }}`) が大きく、機械置換でも保守困難。
  - `pongo2` (Jinja-like Go template): 上流 EJS との直訳精度はやや高いが、依存追加を正当化できない。

## R-007: partial 順序の `_.json` 上書き戦略

- **Decision**: M1 で導入した N-001 「MVP 用 `_.json`」を継承し、本 spec の T-023 内で `assets/templates/classic/partials/_.json` を 4 partial だけに書き換える。上流の長い `_.json` (46 partial) はバックアップとして `_.json.upstream` にリネームして残す。
- **Rationale**: 採用していない partial を `_.json` に残すと `include` が失敗する (ファイルが存在しない)。MVP 用に絞ることで安全に動作する。`_.json.upstream` を残すことで、将来 plugin が増えるたびに 1 行追加すれば良い形になる。
- **Alternatives considered**:
  - 上流 `_.json` のまま、不在 partial を skip する dispatch ロジックを実装: `_.json` の意図 (描画順) が壊れて maintainability が下がる。却下。

## R-008: `Result.Output` の format ディスパッチ位置

- **Decision**: `engine.Compute` 末尾の Stage 4 (template.Run) を拡張する。具体的には:
  - `req.Format == "json"`: `engine.json.Marshal(data)` を呼び、`Result.Output` / `MIME` を populate
  - `req.Format ∈ {"svg", "png", "jpeg"}` かつ `Template != nil`: `tmpl.Run(ctx, pc)` を呼び、戻り値 string を `[]byte` 化して populate
  - `req.Format == ""`: `Template.Metadata().Formats[0]` (または Template が nil なら "json") をデフォルト
  - `req.Format` が PNG/JPEG: M2 では SVG として生成 (chromedp ラッパは M3)、MIME のみ正しく設定し warn ログを出す
- **Rationale**: 既存 4-stage orchestrator に 1 stage 拡張するだけで済む。Format 解決を engine 側に集中させることで、cmd 側はフォーマット非依存。PNG/JPEG が M2 でまだ chromedp 経由で生成できないことを明示的に warn しておくことで、M3 完了までの暫定挙動が予測可能になる。
- **Alternatives considered**:
  - 別関数 (`engine.ComputeJSON` / `engine.ComputeSVG`): API が分裂し、cmd 側が複雑化する。却下。

## R-009: footer の動的部分 mask 戦略 (golden file)

- **Decision**: SVG golden 比較時に `tests/golden/classic/octocat.svg` を読み込んだ後、以下の正規表現で動的部分を `__MASKED__` に置換してから XML 正規化 + MD5 する:
  - `Last updated [^<]+` → `Last updated __MASKED__`
  - `lowlighter/metrics@[^<]+` → `lowlighter/metrics@__MASKED__`
  - `(timezone [^)]+)` (本実装で固定の場合は mask 不要)
- **Rationale**: timestamp と version は environment-dependent (CI / local で異なる) なので golden 比較から外す必要がある。正規表現で mask することで、決定論的なテスト結果を得る。
- **Alternatives considered**:
  - `Result.Output` を modify する: 出力契約に手を入れることになりNG。テスト側で mask する方が安全。

## R-010: テストにおける `version` 文字列の固定

- **Decision**: `internal/engine/json.go` 内に `Version() string` パッケージ関数を置き、ビルド時 `-X` フラグで上書き可能にする。テストでは `engine.SetVersionForTest("test-version")` で固定。
- **Rationale**: golden file が安定するため。production では `git describe --tags --dirty --always` の出力 (Makefile LDFLAGS)。
- **Alternatives considered**:
  - global 変数を test 側で直接書き換え: race を起こす可能性。setter 経由で同期する。

## R-011: 大きな `assets/templates/classic/style.css` の取扱い

- **Decision**: M2 では `style.css` の中身を変更せず、image.svg レンダリング時に `<%= style %>` に該当する箇所へ `style.css` の生バイト列を埋め込む。CSS 最適化 (purge / minify) は M3 (T-034) で扱う。
- **Rationale**: 本 spec は出力経路の確立が主目的。CSS 最適化は M3 と独立に進められるので分離する。M2 段階の SVG は冗長な CSS を含むが機能に支障なし。
- **Alternatives considered**:
  - M2 で CSS purge も同時実装: scope 拡大、テスト責務が混在。却下。

## R-012: cmd mains への output 配線範囲

- **Decision**: 本 spec の範囲では cmd/metrics-action / cmd/metrics-cli の `--help` only 状態を維持する。`Result.Output` を実際にファイル書き出しする処理 (T-109 render + retry / T-110 commit) は M6 で扱う。
- **Rationale**: M2 のコア価値は `engine.Compute` が format 切替に対応すること。CLI / Action からの呼び出し配線は M6 の責務 (T-105 entrypoint + T-117 CLI flags)。scope discipline (原則 III)。
- **Alternatives considered**:
  - M2 で cmd 側も一気に配線: scope 拡大 → 別 PR にすべき。M6 まで待つ。

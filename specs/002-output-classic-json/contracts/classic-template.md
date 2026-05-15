# Contract: classic template (M2)

**Date**: 2026-05-15 | **Plan**: [../plan.md](../plan.md)

本書は `classic.Template.Run` が生成する SVG の構造契約を定義する。上流の `lowlighter/metrics` の classic テンプレート出力と **DOM 同等** (要素・属性・階層が一致、バイト一致は不要) であることが目標 (constitution 原則 II)。

## 1. SVG ドキュメント構造

```xml
<svg xmlns="http://www.w3.org/2000/svg" width="480" height="99999" class="">
  <defs><style><!-- fonts.css 全文 --></style></defs>
  <style data-optimizable="true"><!-- style.css 全文 (M2 未最適化) --></style>
  <style><!-- extras.css 空白 (M2) --></style>

  <foreignObject x="0" y="0" width="100%" height="100%">
    <div xmlns="http://www.w3.org/1999/xhtml" xmlns:xlink="http://www.w3.org/1999/xlink" class="items-wrapper">

      <!-- partials は assets/templates/classic/partials/_.json の順 -->
      <!-- T-023 範囲では 4 つの partial 出力をここに連結 -->
      <!-- base.header / introduction / base.activity+community / base.repositories -->

      <!-- optional metadata footer (base.metadata=true のときのみ) -->
      <footer>
        <span>These metrics include private contributions</span>  <!-- account=user のみ -->
        <span>Last updated 2026-05-15T01:30:00Z (timezone Asia/Tokyo) with mjun0812/github-metrics@v0.1.0</span>
      </footer>

    </div>
    <div id="metrics-end"></div>
  </foreignObject>
</svg>
```

### 1.1 width / height

- 標準 user / organization template: `width="480" height="99999"` (上流互換、height は chromedp で実 height に置換される — M3 で実装)
- columns mode / large mode: M2 では非対応。spec FR-008 の範囲外。

### 1.2 class

- 既定 `""` (アニメーション有効)
- `no-animations`: `extras.css` で `config.animations=false` のときに追加 (本 spec 範囲外、後続)

## 2. partial の dispatch

`assets/templates/classic/partials/_.json` の M2 内容:

```json
[
  "base.header",
  "introduction",
  "base.activity+community",
  "base.repositories"
]
```

T-023 で `_.json` を 4 個に絞る。元の 46 個版は `_.json.upstream` にリネーム退避 (R-007)。

### 2.1 partial の責務

各 `PartialFunc` は次を満たさなければならない:

- **冪等**: 同じ `PartialContext` を渡せば同じ string を返す
- **空文字列 fallback**: 不在キー (`Data.Plugins["xxx"] == nil`、`Data.User == nil` 等) のとき空文字列を返し panic しない
- **XML 安全**: 動的文字列 (login / name / repo 名) は `EscapeXML()` 経由
- **k/m/b/t 短縮**: 数値は `format.Format(n)` 経由

### 2.2 `base.header` の DOM (UC2 AS2)

```xml
<section data-section="header">
  <h1 class="field">
    <img class="avatar" src="<escaped-avatar-url>" width="20" height="20" />
    <span><escaped-name-or-login></span>
  </h1>
  <div class="row">
    <section><!-- 詳細 (joined-at, follower count 等) --></section>
    <section><!-- calendar (M4) -->`</section>
  </div>
</section>
```

M2 段階の最小実装: `data.User == nil` のとき全体を空文字列。`Data.User.Login` のみ存在する場合は `<h1>` の名前部分だけ描画する (avatar / details はオプショナルブロック)。

### 2.3 `introduction` (stub)

`data.Plugins["introduction"] == nil` のとき空文字列。M2 では常に空。

### 2.4 `base.activity+community`

```xml
<section data-section="activity-community">
  <section class="row">
    <section data-block="activity"><!-- 7 day activity --></section>
    <section data-block="community"><!-- followers etc --></section>
  </section>
</section>
```

M2 段階の最小: M4 で base plugin が拡張するまで多くのフィールドが nil なので、上記 outer `<section>` だけ出力し中身は空。M4 で hydrate される構造。

### 2.5 `base.repositories`

```xml
<section data-section="repositories">
  <div class="row">
    <div class="field"><svg class="octicon">...</svg><FormatCount(count)> repositories</div>
    <div class="field"><svg class="octicon">...</svg><FormatCount(stargazers)> stargazers</div>
    <div class="field"><svg class="octicon">...</svg><FormatCount(forks)> forks</div>
  </div>
</section>
```

`octicon` SVG は M3 で `internal/render/octicon.go` を介して挿入。M2 段階では空の `<svg class="octicon" />` プレースホルダで OK (DOM 同等性は保たれる)。

## 3. 文字列組み立て方式

`html/template` は **使わない** (R-005)。partial は `strings.Builder` で組み立てる。

```go
func BaseRepositories(ctx context.Context, pc *templates.PartialContext) (string, error) {
    if pc.Data == nil {
        return "", nil
    }
    repos := pc.Data.Computed.Repositories
    if repos.Count == 0 {
        return "", nil
    }
    var b strings.Builder
    b.WriteString(`<section data-section="repositories">`)
    b.WriteString(`<div class="row">`)
    fmt.Fprintf(&b, `<div class="field">%s repositories</div>`, FormatCount(int64(repos.Count)))
    fmt.Fprintf(&b, `<div class="field">%s stargazers</div>`, FormatCount(int64(repos.Stargazers)))
    fmt.Fprintf(&b, `<div class="field">%s forks</div>`, FormatCount(int64(repos.Forks)))
    b.WriteString(`</div></section>`)
    return b.String(), nil
}
```

XML エンティティエスケープは `EscapeXML()`、数値短縮は `FormatCount()`。

## 4. metadata footer (FR-010)

`base.metadata=true` (inputs から取得) のとき:

```xml
<footer>
  <!-- account=user のみ表示 -->
  <span>These metrics include private contributions</span>
  <!-- 常に表示 -->
  <span>Last updated <RFC3339-timestamp> (timezone <Name>) with mjun0812/github-metrics@<version></span>
</footer>
```

- `Last updated`: 現在時刻を `time.Now().UTC().Format(time.RFC3339)`。テストでは `engine.SetVersionForTest` 同様の固定セッターで決定論的にする (R-010)。
- `timezone`: `data.Config.Timezone.Name` (空 / "UTC" の場合は省略)
- `version`: `engine.Version()` (ビルド時 `-X main.version` で上書き)

footer は SVG 出力の最後の `</div>` の **直前**、`<div id="metrics-end"></div>` の直後に挿入する (R-006)。

## 5. テスト契約 (FR-017)

- **Golden**: `tests/golden/classic/<case>.svg`
- **XML 正規化**: `tests/integration/svg_normalize.go` に共通ヘルパを置く。手順:
  1. パーサ (`encoding/xml`) で AST 化
  2. 要素属性をキー lex sort
  3. テキストノードの whitespace を 1 つの space に collapse
  4. `<!-- comment -->` を除去
  5. footer の動的部分を `__MASKED__` に置換 (R-009)
- **比較**: 正規化後の string を MD5 ハッシュ。差分があれば `t.Errorf` で xml diff を表示。
- **Update**: `go test ./... -update -run TestClassic` で golden 再生成。

## 6. 採用範囲外 partial の追加禁止 (constitution 原則 III)

`assets/templates/classic/partials/_.json` に未採用 plugin の partial 名を追加してはならない。M4 で plugin 実装が landed したタイミングで、当該 plugin の partial を `_.json` に追加し、partial 関数ファイルを `internal/templates/classic/partials/` に追加する流れ。

# 13. 付録: 実装に必要な具体物

本文書は他の仕様書から参照される **コード断片・クエリ・データ構造** をひとまとめにした付録です。各セクションは「実装時にそのまま使える形」で書かれており、Node 版のソースを開く必要はありません (リンクは出典/トレース用)。

## 目次

- [A. base プラグインの GraphQL クエリ全文](#a-base-プラグインの-graphql-クエリ全文)
- [B. base プラグインの取得アルゴリズム (擬似コード)](#b-base-プラグインの取得アルゴリズム-擬似コード)
- [C. classic テンプレート partials 並び順 (`_.json`)](#c-classic-テンプレート-partials-並び順-_json)
- [D. classic image.svg のスケルトン](#d-classic-imagesvg-のスケルトン)
- [E. Action 起動バナーの整形ルール (`info()` 互換)](#e-action-起動バナーの整形ルール-info-互換)
- [F. 入力正規化ルール一覧 (legacy.converter 互換)](#f-入力正規化ルール一覧-legacyconverter-互換)
- [G. svg.Resize の chromedp 評価スクリプト](#g-svgresize-の-chromedp-評価スクリプト)
- [H. svg.Hash の正規化アルゴリズム](#h-svghash-の正規化アルゴリズム)
- [I. partial 補完規則 (`config.order` × `_.json` の合成)](#i-partial-補完規則-configorder--_json-の合成)
- [J. プラグインエラーの分類規則](#j-プラグインエラーの分類規則)
- [K. presets YAML スキーマ v1 例](#k-presets-yaml-スキーマ-v1-例)

---

## A. base プラグインの GraphQL クエリ全文

すべて GitHub GraphQL v4 (`https://api.github.com/graphql`) を対象。
変数は `$<name>` 形式の **文字列置換** で渡される (実装では `genqlient` 等で型安全に置換しても良い)。

### A.1 `user.graphql`

```graphql
query BaseUser {
  user(login: "$login") {
    databaseId
    name
    login
    location
    createdAt
    avatarUrl
    websiteUrl
    twitterUsername
  }
}
```

### A.2 `user.x.graphql`

`$affiliations` には `, ownerAffiliations: [OWNER, COLLABORATOR, ORGANIZATION_MEMBER]` (利用者の選択で部分集合) が入る。

```graphql
query BaseUserX {
  user(login: "$login") {
    packages { totalCount }
    starredRepositories { totalCount }
    watching { totalCount }
    sponsorshipsAsSponsor { totalCount }
    sponsorshipsAsMaintainer { totalCount }
    followers { totalCount }
    following { totalCount }
    issueComments { totalCount }
    organizations { totalCount }
    repositoriesContributedTo(includeUserRepositories: true) { totalCount }
    repositories(last: 0 $affiliations) {
      totalCount
      totalDiskUsage
    }
    contributionsCollection {
      totalRepositoriesWithContributedCommits
      totalCommitContributions
      restrictedContributionsCount
      totalIssueContributions
      totalPullRequestContributions
      totalPullRequestReviewContributions
    }
    calendar: contributionsCollection(from: "$calendar.from", to: "$calendar.to") {
      contributionCalendar {
        weeks {
          contributionDays { color }
        }
      }
    }
  }
}
```

### A.3 `organization.graphql`

```graphql
query BaseOrganization {
  organization(login: "$login") {
    databaseId
    name
    login
    location
    createdAt
    avatarUrl
    websiteUrl
    isVerified
    twitterUsername
  }
}
```

### A.4 `organization.x.graphql`

```graphql
query BaseOrganizationX {
  organization(login: "$login") {
    packages { totalCount }
    sponsorshipsAsSponsor { totalCount }
    sponsorshipsAsMaintainer { totalCount }
    membersWithRole { totalCount }
    repositories(last: 0 $affiliations) {
      totalCount
      totalDiskUsage
    }
  }
}
```

### A.5 `calendar.graphql`

`$calendar.from`, `$calendar.to` は ISO 8601 文字列。Bulk クエリ失敗時の単独フォールバックで利用。

```graphql
query BaseCalendar {
  user(login: "$login") {
    calendar: contributionsCollection(from: "$calendar.from", to: "$calendar.to") {
      contributionCalendar {
        weeks {
          contributionDays { color }
        }
      }
    }
  }
}
```

### A.6 `contributions.graphql`

- `$field`: `totalRepositoriesWithContributedCommits` / `totalCommitContributions` / `restrictedContributionsCount` / `totalIssueContributions` / `totalPullRequestContributions` / `totalPullRequestReviewContributions` のいずれか。
- `$range`: 全期間なら空文字、特定期間なら `(from: "<ISO>", to: "<ISO>")`。

```graphql
query BaseContributions {
  user(login: "$login") {
    contributionsCollection$range {
      $field
    }
  }
}
```

### A.7 `field.graphql` (フォールバック用単一フィールド)

- `$account`: `user` / `organization`。
- `$field`: 例 `packages`, `starredRepositories(includeUserRepositories: true)` 等の **GraphQL 表現** をそのまま埋め込む。

```graphql
query BaseField {
  $account(login: "$login") {
    $field { totalCount }
  }
}
```

### A.8 `field.repositories.graphql`

`$field`: `totalCount` / `totalDiskUsage`。

```graphql
query BaseFieldRepositories {
  $account(login: "$login") {
    repositories(last: 0 $affiliations) {
      $field
    }
  }
}
```

### A.9 `repositories.graphql`

- `$type`: `repositories` または `repositoriesContributedTo`。
- `$after`: `after: "<cursor>"` または空。
- `$repositories`: 1 ページの件数 (1〜100)。
- `$forks`: `, isFork: false` または空。
- `$constraints`: `repositoriesContributedTo` のとき `, includeUserRepositories: false, contributionTypes: COMMIT`、それ以外は空。

```graphql
query BaseRepositories {
  $account(login: "$login") {
    $type($after first: $repositories $forks $affiliations $constraints,
          orderBy: {field: UPDATED_AT, direction: DESC}) {
      edges { cursor }
      nodes {
        updatedAt
        name
        owner { login }
        isFork
        forkCount
        watchers { totalCount }
        stargazers { totalCount }
        releases { totalCount }
        deployments { totalCount }
        environments { totalCount }
        languages(first: 8) {
          edges {
            size
            node { color name }
          }
        }
        licenseInfo { name spdxId }
        issues_open: issues(states: OPEN) { totalCount }
        issues_closed: issues(states: CLOSED) { totalCount }
        pr_open: pullRequests(states: OPEN) { totalCount }
        pr_closed: pullRequests(states: CLOSED) { totalCount }
        pr_merged: pullRequests(states: MERGED) { totalCount }
      }
    }
  }
}
```

### A.10 `repository.graphql`

- `$account`: `user` / `organization`。
- `$repo`: リポジトリ名。

```graphql
query BaseRepository {
  $account(login: "$login") {
    repository(name: "$repo") {
      name
      createdAt
      diskUsage
      homepageUrl
      owner { login }
      isFork
      forkCount
      watchers { totalCount }
      stargazers { totalCount }
      releases { totalCount }
      deployments { totalCount }
      environments { totalCount }
      languages(first: 8) {
        edges {
          size
          node { color name }
        }
      }
      licenseInfo { name spdxId }
      issues_open: issues(states: OPEN) { totalCount }
      issues_closed: issues(states: CLOSED) { totalCount }
      pr_open: pullRequests(states: OPEN) { totalCount }
      pr_closed: pullRequests(states: CLOSED) { totalCount }
      pr_merged: pullRequests(states: MERGED) { totalCount }
    }
  }
}
```

---

## B. base プラグインの取得アルゴリズム (擬似コード)

bulk → unit のフォールバック、indepth 集計、リポジトリページングを統合した取得手順。

```text
function RunBase(ctx, login, q):
    inputs = NormalizeBaseInputs(q)
    affiliations = BuildOwnerAffiliations(inputs.repositories.affiliations, isAuthenticated(login))
    forks         = inputs.repositories.forks ? "" : ", isFork: false"

    # 1. skip / notoken の早期分岐
    if settings.NoToken or inputs.skip:
        PostprocessSkip(data, login)
        return

    # 2. base sections のフラグ確定 (header/activity/community/repositories/metadata)
    if "base" in q:
        defaulted = legacy.converter(q.base) ?? true
    else:
        defaulted = true
    for part in [header, activity, community, repositories, metadata]:
        data.Base[part] = legacy.converter(q["base."+part]) if defined else defaulted

    # 3. user → organization の順で取得を試みる (NOT_FOUND なら次へ)
    for account in [user, organization]:
        try:
            queried = graphql(queries.base.user(login)) if account=="user"
                      else graphql(queries.base.organization(login))
            data.User = queried[account]
            Postprocess[account](data, login)

            # 3a. bulk クエリ (user.x / organization.x)
            try:
                bulk = graphql(queries.base[account+".x"](
                    login, affiliations,
                    calendar.from = now - 14 days, calendar.to = now))
                data.User.merge(bulk[account])
            except:
                # 3b. unit fallback
                fields = AccountFallbackFields(account)
                for field in fields:
                    try:
                        x = graphql(queries.base.field(login, account, field))
                        data.User.merge(x[account])
                    except:
                        data.User[field] = { totalCount: NaN }
                for field in ["totalCount", "totalDiskUsage"]:
                    try:
                        r = graphql(queries.base.field.repositories(
                                login, account, field, affiliations))
                        data.User.Repositories.merge(r[account].repositories)
                    except:
                        data.User.Repositories[field] = NaN
                if account == "user":
                    for field in ContributionFields():
                        try:
                            c = graphql(queries.base.contributions(login, account, field, range=""))
                            data.User.ContributionsCollection.merge(c[account].contributionsCollection)
                        except:
                            data.User.ContributionsCollection[field] = NaN
                    try:
                        cal = graphql(queries.base.calendar(login,
                                  calendar.from = now - 14 days, calendar.to = now))
                        data.User.merge(cal[account])
                    except:
                        data.User.Calendar = { ContributionCalendar: { Weeks: [] } }

            # 3c. indepth 集計 (account==user かつ extras 許可時のみ)
            if account == "user" and inputs.indepth and Extras("indepth", settings):
                for field in ContributionFields():
                    total = 0
                    from = data.User.CreatedAt
                    end  = now
                    while from < end:
                        to = min(from + 4 weeks, end)
                        dto = to.minusSeconds(1)   # 重複回避: 23:59:59.999
                        try:
                            c = graphql(queries.base.contributions(login, account, field,
                                          range=`(from:"${from}", to:"${dto}")`))
                            total += c[account].contributionsCollection[field]
                        except: ignore
                        from = to
                    data.User.ContributionsCollection[field] =
                        max(total, data.User.ContributionsCollection[field])
                # 全期間のコミット集計 (commit search API)
                try:
                    total = rest.search.commits(`author:${login}`).totalCount
                    data.User.ContributionsCollection.totalCommitContributions =
                        max(total, data.User.ContributionsCollection.totalCommitContributions)
                except: ignore
                if inputs.hireable:
                    data.User.IsHireable = true

            # 4. リポジトリページング
            types = (account=="user") ? [repositories, repositoriesContributedTo]
                                       : [repositories]
            for type in types:
                cursor = null
                batch  = inputs.repositories.batch          # 既定 100
                while collected[type] < inputs.repositories.count:
                    try:
                        page = graphql(queries.base.repositories(
                                login, account, type,
                                after=cursor, repositories=batch,
                                forks=forks, affiliations=affiliations,
                                constraints=ConstraintsFor(type)))
                    except timeout:
                        batch = floor(batch/2)
                        if batch < 1: raise
                        continue
                    nodes = page[account][type].nodes
                    cursor = page[account][type].edges.last?.cursor
                    data.User[type].Nodes.append(nodes)
                    if len(nodes) < inputs.repositories.count: break
                # 上限で切り詰める
                data.User[type].Nodes = data.User[type].Nodes[:inputs.repositories.count]

            # 5. ghcr パッケージ数を REST で追加 (任意; read:packages 必要)
            try:
                pkgs = rest.packages.list(account, package_type="container", login)
                data.User.Packages.TotalCount += len(pkgs)
            except: ignore

            # 6. 共有オプションを data.Shared に詰める (他プラグインが参照)
            data.Shared = {
                "repositories.skipped": inputs.repositories.skipped,
                "users.ignored":        inputs.users.ignored,
                "commits.authoring":    inputs.commits.authoring,
                "repositories.batch":   batch,
            }
            return  # 成功で抜ける

        except err:
            if err.message contains "Could not resolve to a User with the login of":
                continue   # 次の account タイプへ
            raise

    # 7. すべての account 種別で失敗 → user not found
    raise UserNotFoundError(login)
```

### B.1 `AccountFallbackFields(account)`

| account | fields |
|---------|--------|
| `user` | `packages`, `starredRepositories`, `watching`, `sponsorshipsAsSponsor`, `sponsorshipsAsMaintainer`, `followers`, `following`, `issueComments`, `organizations`, `repositoriesContributedTo(includeUserRepositories: true)` |
| `organization` | `packages`, `sponsorshipsAsSponsor`, `sponsorshipsAsMaintainer`, `membersWithRole` |

### B.2 `ContributionFields()`

```
["totalRepositoriesWithContributedCommits",
 "totalCommitContributions",
 "restrictedContributionsCount",
 "totalIssueContributions",
 "totalPullRequestContributions",
 "totalPullRequestReviewContributions"]
```

### B.3 `PostprocessSkip` の初期値

`token == "NOT_NEEDED"` または `base_skip=true` のとき:

```text
data.Account = "bypass"
data.User = {
    DatabaseId: NaN,
    Name:      login,
    Login:     login,
    CreatedAt: now,
    AvatarURL: "https://github.com/${login}.png",
    WebsiteURL: null,
    TwitterUsername: login,
    Repositories:           { TotalCount: NaN, TotalDiskUsage: NaN, Nodes: [] },
    RepositoriesContributedTo: { TotalCount: NaN, Nodes: [] },
    Packages: { TotalCount: NaN },
    # user / organization 両方の Postprocess で埋まる NaN フィールド群もすべて初期化
}
```

---

## C. classic テンプレート partials 並び順 (`_.json`)

`partials/_.json` (classic テンプレートのデフォルト並び):

```json
[
  "base.header",
  "introduction",
  "base.activity+community",
  "base.repositories",
  "lines",
  "followup",
  "discussions",
  "languages",
  "notable",
  "projects",
  "repositories",
  "gists",
  "pagespeed",
  "habits",
  "topics",
  "music",
  "nightscout",
  "posts",
  "rss",
  "tweets",
  "isocalendar",
  "calendar",
  "stars",
  "starlists",
  "stargazers",
  "people",
  "activity",
  "reactions",
  "anilist",
  "wakatime",
  "skyline",
  "support",
  "stackoverflow",
  "leetcode",
  "crypto",
  "stock",
  "achievements",
  "screenshot",
  "code",
  "chess",
  "sponsors",
  "sponsorships",
  "poopmap",
  "16personalities",
  "fortune",
  "splatoon",
  "steam"
]
```

`base.*` は base プラグインの sub-section、それ以外はプラグイン名と一致する。

repository / terminal / markdown テンプレートは別の並びを持つ (それぞれ partial 内容も異なる)。実装時は各テンプレートの partial 名集合を別途定義する。

---

## D. classic image.svg のスケルトン

EJS 表記 (`<%= %>` / `<% %>`) を残したまま提示する。実装側で Go template/関数化する際は **DOM 構造を同一** に保つこと。

```html
<svg xmlns="http://www.w3.org/2000/svg"
     width="<%= large ? 960 : columns ? '100%' : 480 %>"
     height="99999"
     class="<%= large ? 'large' : columns ? 'columns' : '' %>
            <%= !animated ? 'no-animations' : '' %>">

  <defs><style><%= fonts %></style></defs>
  <style data-optimizable="true"><%= style %></style>
  <style><%= extras.css %></style>

  <foreignObject x="0" y="0" width="100%" height="100%">
    <div xmlns="http://www.w3.org/1999/xhtml"
         xmlns:xlink="http://www.w3.org/1999/xlink"
         class="items-wrapper">

      <% if (warnings.length) { %>
        <section>
          <div class="row">
            <div class="field warning">
              <svg xmlns="http://www.w3.org/2000/svg"
                   viewBox="0 0 16 16" width="16" height="16">
                <!-- warning octicon -->
                <path fill-rule="evenodd" d="M8.22 1.754a.25.25 0 00-.44 0L1.698 13.132a.25.25 0 00.22.368h12.164a.25.25 0 00.22-.368L8.22 1.754zm-1.763-.707c.659-1.234 2.427-1.234 3.086 0l6.082 11.378A1.75 1.75 0 0114.082 15H1.918a1.75 1.75 0 01-1.543-2.575L6.457 1.047zM9 11a1 1 0 11-2 0 1 1 0 012 0zm-.25-5.25a.75.75 0 00-1.5 0v2.5a.75.75 0 001.5 0v-2.5z"></path>
              </svg>
              <%= warnings.map(({warning}) => warning.message).join(", ") %>
            </div>
          </div>
        </section>
      <% } %>

      <% for (const partial of [...partials]) { %>
        <%- await include(`partials/${partial}.ejs`) %>
      <% } %>

      <% if (base.metadata) { %>
        <footer>
          <% if (account === "user") { %>
            <span>These metrics
              <%= !computed.token.scopes.includes("repo") ? "do not include all" : "include" %>
              private contributions</span>
          <% } %>
          <span>Last updated <%= meta.generated %>
            <% if ((config.timezone?.name)&&(!config.timezone?.error)) { %>
              (timezone <%= config.timezone.name %>)
            <% } %>
            with lowlighter/metrics@<%= meta.version %>
          </span>
        </footer>
      <% } %>

    </div>
    <div id="metrics-end"></div>
  </foreignObject>

</svg>
```

### D.1 要素の役割

| 要素 / クラス | 役割 |
|--------------|------|
| `<svg width=...>` | `large=960` / `columns=100%` / それ以外 480 |
| `class="large"` | `data.Large=true` |
| `class="columns"` | `data.Columns=true` |
| `class="no-animations"` | `data.Animated=false` で CSS アニメ停止 |
| `<style data-optimizable="true">` | CSS 最適化対象。PurgeCSS + CSSO の対象 |
| `<style><%= extras.css %></style>` | ユーザー任意 CSS。extras フラグでガード |
| `class="items-wrapper"` | partial の親 |
| `#metrics-end` | chromedp が `getBoundingClientRect()` で高さ測定する基準要素 |
| `<footer>` | `base.metadata=true` で表示 |

---

## E. Action 起動バナーの整形ルール (`info()` 互換)

### E.1 出力フォーマット

各行は **左 63 + 9*(色エスケープ含む) 文字 + ` │ ` (separator) + 値** で構成する。

```
左ラベル                                                          │ 値
```

ANSI カラー (`\x1b[36m...\x1b[0m`) を含む行はパディングを 9 文字ぶん広く取る。

### E.2 値の表現規則

| 型 | 表現 |
|----|------|
| `undefined` | `(default)` |
| `array` | `comma, join` (空なら `(none)`) |
| `object` | `JSON.stringify(value)` |
| `string` (通常) | そのまま |
| `string` (`type: token`) | 下記参照 |
| `boolean` | `true` / `false` |

### E.3 token 値の表現

| 値 | 表現 |
|----|------|
| `^MOCKED` で始まる | `(MOCKED TOKEN)` |
| `NOT_NEEDED` | `(NOT NEEDED)` |
| 任意の非空 | `(provided)` |
| 空 / 未指定 | `(missing)` |

### E.4 セパレータ

`info.break()` は `─` を 88 個繰り返す:

```
────────────────────────────────────────────────────────────────────────────────────────
```

`info.section(title)` は左ラベルをシアン (`\x1b[36m`) でハイライト。

### E.5 グループ出力

`info.group({metadata, name, inputs})` は以下を順に出す:

1. プラグイン名 (`metadata.plugins[name].name` の先頭の英単語部分) をシアンで section ヘッダ。
2. `inputs` のキーごとに 1 行ずつ:
   - 左: `metadata.plugins[name].inputs[key].description` の 1 行目 (なければキー名)。
   - 右: 値。`preset` 由来の値は `*<value>` プレフィックス付き。`type: token` ならマスク。

---

## F. 入力正規化ルール一覧 (legacy.converter 互換)

`base` セクションのフラグ等で使われる boolean キャストの規則。

| 入力 | 戻り値 |
|------|--------|
| 正規表現 `^(True\|true\|On\|on\|Yes\|yes\|1)$` | `true` |
| 正規表現 `^(False\|false\|Off\|off\|No\|no\|0)$` | `false` |
| `Number.isFinite(Number(value)) == true` | `Boolean(Number(value))` |
| それ以外 | `undefined` (= 既定値継承) |

実装注意: 大文字小文字無視ではなく、上記 **正規表現で示した形** を厳密に受け付ける。`"true"` は OK だが `"TRUE"` は最後の数値判定にも引っかからない (`NaN`) → `undefined`。

---

## G. svg.Resize の chromedp 評価スクリプト

`<svg>` のレンダリング後の高さを計測し、属性を書き換える。chromedp の `Evaluate` に渡す JS 本体は次の通り (Node 版が puppeteer に渡す関数と同等):

```javascript
async (padding, scripts) => {
  // ユーザー任意 JS の実行 (extras フラグで有効化されている前提)
  for (const script of scripts) {
    try {
      await new Function("document", `return (async () => {${script}})()`)(document);
    } catch (e) {
      console.debug(`script error: ${e}`);
    }
  }

  // アニメーション一時停止
  const svg = document.querySelector("svg");
  const animated = !svg.classList.contains("no-animations");
  if (animated) svg.classList.add("no-animations");

  // アニメ後のレイアウト安定を待つ
  await new Promise(r => setTimeout(r, 2400));

  // 基準要素のバウンディングを取得
  let { y: height, width } = document.querySelector("svg #metrics-end").getBoundingClientRect();

  // padding を適用 (絶対 + 相対)
  height = Math.max(1, Math.ceil(height * padding.height + padding.absolute.height));
  width  = Math.max(1, Math.ceil(width  * padding.width  + padding.absolute.width));

  // svg の height 属性を上書き (auto は維持)
  if (svg.getAttribute("height") !== "auto") svg.setAttribute("height", height);

  // 復帰
  if (animated) svg.classList.remove("no-animations");

  return {
    resized: new XMLSerializer().serializeToString(svg),
    width, height
  };
}
```

### G.1 padding パース

入力 `paddings` は `Array<string>` または `","` 区切り string。各次元 (width, height) について `"<absolute> + <relative>%"` 形式を以下の規則で分解する:

```text
operands = paddings[i] ?? paddings[0]
relative = match(operands, /([+-]?[\d.]+)%$/)        # 例: "11%"
operands = operands without the matched relative
absolute = match(operands, /^([+-]?[\d.]+)/)         # 例: "8"

padding[dim]            = 1 + (relative/100)   # 既定 1 (倍率)
padding.absolute[dim]   = absolute             # 既定 0 (加算)
```

### G.2 PNG/JPEG 変換

`convert=png|jpeg` のとき:

```text
screenshot = page.Screenshot(
  clip = { x: 0, y: 0, width, height },
  type = convert,
  omitBackground = true
)
mime = `image/${convert}`
```

それ以外は `mime = "image/svg+xml"`、`resized` (XML 文字列) を返す。

---

## H. svg.Hash の正規化アルゴリズム

`output_condition=data-changed` の比較に使う MD5 計算手順。

```text
function Hash(rendered string) -> string|null:
    if rendered == "" or rendered == null:
        return null
    doc = parseHTML(rendered)
    doc.querySelector("footer")?.remove()        # 動的部分を除外
    svgNode = doc.querySelector("svg")
    return md5(svgNode.outerHTML)
```

`<footer>` には timezone / version / generated time が入るため、それを除いた SVG 本体のみで一致判定する。

---

## I. partial 補完規則 (`config.order` × `_.json` の合成)

テンプレートのデフォルト順 (`template.partials`) とユーザー指定順 (`q["config.order"]`) を以下のように合成する。

```text
input:
    user  = q["config.order"]    # 配列 (string)、未指定なら []
    tmpl  = template.partials    # 配列 (string)

step1: filter user by tmpl (user ⊆ tmpl)
    head  = user.filter(p => tmpl.includes(p))

step2: tmpl を末尾に連結
    merged = head ++ tmpl

step3: set にして重複除去 (順序保持)
    out   = OrderedSet(merged)
```

実装は Go なら `[]string` を `map[string]struct{}` で重複除去するヘルパ (`engine.MergePartials`)。

---

## J. プラグインエラーの分類規則

各プラグインの `Run` が返す `error` を以下のカテゴリに振り分け、`data.Errors` / `footer` 出力に使う。

| カテゴリ | 判定条件 | 振る舞い |
|---------|---------|---------|
| `user`   | エラーメッセージに `Could not resolve to a User` を含む | 404 / footer に "User not found" |
| `github` | GraphQL response.errors の `type` が `NOT_FOUND` / メッセージに `this may be the result of a timeout, or it could be a GitHub bug` を含む | 500 / footer に "GitHub timeout" |
| `unsupported` | `not supported for: <reason>` で始まる | 406 / footer に reason |
| `template` | `unsupported template` | 400 / "unsupported template" |
| `internal` | 上記以外 | 500 / footer に汎用エラー |

`die=true` の場合は最初の致命エラーで `engine.Compute` が `error` を返す。
`die=false` の場合は `data.Errors` に積み、`footer` に表示する。

---

## K. presets YAML スキーマ v1 例

`config_presets` で読み込まれる YAML の構造。

```yaml
# schema 番号 (現状 v1 のみ)
schema: v1

# キー名は metadata.yml の入力名と一致 (action.yml の inputs)
with:
  plugin_languages: yes
  plugin_languages_indepth: yes
  plugin_languages_limit: 0
  plugin_languages_sections: most-used, recently-used
  config_padding: 0, 8 + 11%
```

### K.1 制約

- `with` 配下のキーは `metadata.yml` の `Inputs` に存在しなければならない。
- `type: token` 入力は preset で指定不可 (セキュリティ上の理由)。
- `metadata.yml` 側で `preset: no` が指定された入力は preset で指定不可。
- 値の型は `metadata.yml` の `type` に従う (boolean は `yes/no/true/false`、数値はそのまま)。

### K.2 取り込み形式

`config_presets` 入力の値 (カンマ / 改行区切り) は次のいずれか:

| 形式 | 例 | 解釈 |
|------|-----|------|
| `@<name>` | `@languages` | 組込み preset (本リポジトリにバンドル) |
| `https://...` | `https://gist.github.com/user/abc.yml` | URL fetch (Accept: text/plain) |
| ローカルパス | `presets/foo.yml` | Action 環境でファイル読み込み |

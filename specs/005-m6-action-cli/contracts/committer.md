# Contract: Committer (output_action dispatch)

**Date**: 2026-05-17 | **Plan**: [../plan.md](../plan.md) | **Related**: [../data-model.md](../data-model.md) E-002

本書は `output_action` 入力の各値ごとの GitHub API 呼び出しシーケンスを定める。サポート値は 6 種類のみ (詳細は [output-actions.md](./output-actions.md))。本書では各シーケンスの責務 + idempotency + error handling を扱う。

## 1. 共通: 起動条件

- `Invocation.Dryrun == true` のとき本 contract は **skip**。Committer は呼ばれない (FR-009)。
- `Invocation.OutputAction == "none"` のとき本 contract は **skip** (no-op)。
- 上記以外 (`commit`, `pull-request*`) のとき下記 sequence を実行。

## 2. `commit` 経路

```text
1. RepoOwner / RepoName を $GITHUB_REPOSITORY から取得 (M6 action mode)、
   または --config / CLI flag で指定 (CLI mode で commit 経路の場合)。
2. Branch を解決 (committer_branch 入力 → 空ならデフォルト branch)。
3. ensureBranch(branch):
   3a. GET /repos/{owner}/{repo}/git/refs/heads/{branch}
       → 200 OK → 何もしない
       → 404 → PUT /repos/.../git/refs (default branch から fork)
   3b. それ以外の HTTP error → *xerrors.RetryableError でラップして throw
4. もし output_condition=data-changed なら:
   4a. HashComparator.Equal() を呼ぶ
   4b. true なら Committer.Commit = false にして本 sequence の残りを skip
5. previousSHA を取得 (file が既存なら GET /repos/.../contents/<filename>?ref=<branch> の sha フィールド)。
6. PUT /repos/.../contents/<filename>:
   { "message": <committer_message>,
     "content": base64(body),
     "branch": <branch>,
     "sha": <previousSHA or omit if new>,
     "committer": { "name": <author>, "email": <email> } }
   → 201/200 で commit SHA を取得
7. metrics_sha output を render.Hash(body) で書く。
8. metrics_url output を https://github.com/{owner}/{repo}/blob/{branch}/{filename} で書く。
```

### Idempotency

- step 3a (ensureBranch) は 既存 branch があれば no-op、無ければ作成 → 副作用は冪等
- step 6 (PUT /contents) は `sha` を渡せば conflict 検出可。`sha` がない (= new file) で既に同名 file があれば 422 が返るので retryable error として扱う + 警告ログ

### Error handling

- GitHub API 5xx / timeout → `*xerrors.RetryableError` (FR-007 で retry 経路に乗る)
- GitHub API 4xx (`403 branch protection`, `422 conflict`) → 通常 error
  - **action exit 0 + warning ログ** (FR-016): committer failure は workflow 全体を止めない

## 3. `pull-request` 経路

```text
1. RepoOwner / RepoName 取得 (同上)
2. baseBranch を解決 (committer_branch → 空なら default branch)
3. headBranch を生成: "metrics-run-${GITHUB_RUN_ID}" (run 単位で unique)
4. ensureBranch(headBranch) — baseBranch から fork
5. もし output_condition=data-changed なら baseBranch の <filename> と比較 (HashComparator)
   → 一致なら no-op exit 0 (PR 作らない)
6. PUT /repos/.../contents/<filename>?ref=<headBranch>  (上記 commit 経路 と同)
7. POST /repos/.../pulls:
   { "title": <committer_message から 1 行目>,
     "body": <committer_message 全文 + 自動生成注記>,
     "head": <headBranch>,
     "base": <baseBranch>,
     "maintainer_can_modify": true }
   → 201 で PR 番号を取得
8. metrics_url を PR URL に set
9. metrics_sha を render.Hash(body) に set
```

### Error handling

- PR 作成失敗 (`422 No commits between branches`, `422 A pull request already exists for owner:metrics-run-<id>`) → warning ログ + exit 0
- 同名 headBranch (`metrics-run-<runid>`) が既に存在 → 普通ありえない (runId はユニーク) ので, あれば error と扱う

## 4. `pull-request-merge` / `-squash` / `-rebase` 経路

```text
1-9. (pull-request 経路と同じ、PR を作る)
10. PR の mergeable 状態を polling:
    GET /repos/.../pulls/{n}
    → mergeable == true まで最大 30 秒 retry (5 秒間隔)
11. mergeable=true なら:
    PUT /repos/.../pulls/{n}/merge?merge_method=<method>
    Body: { "commit_message": <committer_message>, "merge_method": "merge|squash|rebase" }
    → 200 で merge 成功
12. mergeable=false の状態が 30 秒続いたら warning ログ + exit 0 (PR は残る、手動 merge を促す)
13. merge 成功時:
    DELETE /repos/.../git/refs/heads/<headBranch> で head branch を削除 (cleanup)
```

### merge method マップ

| output_action 値 | merge_method 値 |
|------------------|-----------------|
| `pull-request-merge` | `merge` |
| `pull-request-squash` | `squash` |
| `pull-request-rebase` | `rebase` |

## 5. retry policy 適用範囲

| API call | Retry? | 理由 |
|----------|--------|------|
| `GET /git/refs/heads/...` | Yes (RetryableError) | branch lookup |
| `PUT /git/refs` | Yes | branch creation |
| `GET /contents/...` | Yes | data-changed check |
| `PUT /contents/...` | Yes (RetryableError) | commit creation |
| `POST /pulls` | Yes (RetryableError) | PR creation |
| `GET /pulls/{n}` (polling) | No (内部で max 30s loop) | polling は polling 内で完結 |
| `PUT /pulls/{n}/merge` | Yes (RetryableError) | merge |
| `DELETE /git/refs/heads/...` | No | cleanup 失敗は warning ログのみ |

## 6. テスト戦略 (SC-005 / SC-006 / SC-007)

`internal/action/committer_test.go`:

- `TestCommit_NewFile` / `TestCommit_ExistingFile_DataChanged` / `TestCommit_ExistingFile_NoChange_Skip`
- `TestPR_NewRun` / `TestPR_DuplicateHeadBranch` / `TestPR_DataChanged_NoPR`
- `TestPRMerge_MergeableTrue` / `TestPRMerge_MergeableFalse_Timeout` / `TestPRSquash` / `TestPRRebase`
- `TestCommitter_Failure_ExitZero` (FR-016 = failure は exit 0 で workflow 止めない)

すべて mocked `*githubapi.REST` を使う (M1 既存)。

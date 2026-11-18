# numreserve 利用ガイド

numreserve は、タスク番号と ADR 番号を、別のクローンや cloud のセッションの間でも重ならないように決める開発用の道具である。番号ごとに GitHub の `origin` に tag（記録の tag）を 1 つ作り、作れた番号だけを返す。同じ名前の ref は GitHub の上で 2 つ作れないので、2 つのセッションが同じ番号を得ることはない。Go の道具 `numreserve`（`cmd/numreserve`・`internal/numreserve`）で、`/mkreq` の step 2 と ADR の規約（[adr.md](../conventions/adr.md) の「Location and naming」と「Taking in `main`」）が呼ぶ。決定は [ADR-0051](../adr/0051-number-reservation.md) に、要件と設計は [0100 要件定義書](../../tasks/0100_number_reservation/01_requirements.md) と [0100 設計書](../../tasks/0100_number_reservation/02_architecture.md) にある。

## 1. インストールと呼び出し

```bash
make install-numreserve
```

`go install ./cmd/numreserve` を実行する。道具は `main` から入れ、道具を変えた PR がマージされたら、`main` を更新してから入れ直す（[ADR-0014](../adr/0014-deterministic-checks-in-go.md) の「道具は `main` からインストールして使う」）。エージェントは道具を入れたりビルドしたりしない。道具が見つからないと、`/mkreq` と ADR の手順は番号を決めずに止まる。

呼び出しの形は次の 2 つだけである。引数は番号の空間を選ぶ 1 語で、ほかの引数とフラグは受け付けない（終了コード 2）。リポジトリの作業ツリーのどこかで実行する。ADR 番号は、ADR を加える PR のブランチの作業ツリーで実行する。

| 番号の空間 | 呼び出し | 記録の tag | ruleset の対象 |
|---|---|---|---|
| タスク番号 | `numreserve task` | `refs/tags/task-number/NNNN` | `refs/tags/task-number/**/*` |
| ADR 番号 | `numreserve adr` | `refs/tags/adr-number/NNNN` | `refs/tags/adr-number/**/*` |

-   **番号を決めたとき:** 標準出力に 4 桁の番号を 1 行だけ出し、終了コード 0 で終わる。標準エラーには、作った記録の tag の名前と、それが指す `origin/main` の commit を 1 行出す。
-   **止まったとき:** 標準出力に何も出さず、標準エラーに `numreserve: stop <理由の語>: <段>: <詳細>` の 1 行を出し、終了コード 1 で終わる。理由の語ごとの意味と次にすることは 2 章の表にある。詳細には、止まった段、HTTP の状態コードやコマンドの終了コード、原因のブランチ名やファイルのパスを出す。ブランチ名とファイルのパスは Go の引用の形で、200 バイトまでに切って出す。GitHub の応答の本文、`gh` と `git` の標準エラー、`origin` の URL、認証の情報は出さない。

`.claude/settings.json` は `Bash(numreserve task)` と `Bash(numreserve adr)` の 2 つだけを許すので、この 2 つの形なら許可を尋ねない。呼ぶたびに消せない記録の tag を作るので、エージェントは手順が新しい番号を求めるときだけ呼び、試しや確かめのためには呼ばない（[CLAUDE.md](../../../CLAUDE.md) の「Tool Execution Safety」）。番号を決めた後に作業が止まると、その番号は欠番として残る。欠番は誤りではない。

道具は、`origin` が GitHub の `isseis/yt2column` であることを前提にする。リポジトリの名前は道具の定数で、`origin` の URL は読まない。`origin` が別のリポジトリや古いクローンを指すと、使用中の番号を見落としうる。

## 2. 止める理由と次にすること

道具は、番号を決められないときに推測した番号を出さずに止まる。`/mkreq` と ADR の手順は、終了コード 0 以外をすべて「番号を決められなかった」として扱い、ブランチもタスクのディレクトリも ADR のファイルも作らずに止まって、標準エラーの 1 行とこの表を示す。

| 理由の語 | 意味 | 起きるとき | 次にすること |
|---|---|---|---|
| `permission` | 権限がない | 記録の tag の作成が 401・403・404 で拒まれた（rate limit と、見出しの形が受け入れない形の 403 を除く）。`gh` が状態行を出さずに終了コード 4 で終わった（認証されていない） | `gh auth status` で認証と権限を確かめる。cloud のセッションでは 4 章を読む |
| `network-or-github` | ネットワークまたは GitHub の失敗 | 時間切れ、`git` のネットワークの失敗（`git ls-remote` と取得）、5xx などのほかの状態コード、rate limit、見出しの形が受け入れない形の 403、状態行がないか形が誤っている、`gh` の出力が 4 MiB を超えた、`gh` を起動できない（入っていない）、作成が 422 で失敗した後に同じ名前の ref を読めない（ruleset による拒否を含む）、`git ls-remote` が返した SHA を取得できない | 詳細が `gh could not be run` なら `gh` を入れる。そのほかは時間をおいて再実行し、続くときは GitHub の状態と ruleset（3 章）を確かめる |
| `attempts` | 試す回数の上限に達した | 作成を 10 回試し、どれもほかのセッションに先を越された | 再実行する |
| `exhausted` | 4 桁の番号が残っていない | 次に試す番号が 9999 を超えた | 利用者が番号の形を見直す（道具の変更が要る） |
| `jump` | 番号が不自然に跳んだ | 次に試す番号が、`origin/main` と記録の tag の番号の最大値 + 20 を超えた。詳細に、番号を押し上げたブランチ名かファイルのパスが出る | 5 章の手順で、詳細に出たブランチやファイルを確かめて片付ける |
| `list-limit` | 一覧が上限を超えた | `git` の出力が 4 MiB を超えた。`origin` のブランチが 500 を超えた | 使わないブランチを片付けて再実行する。上限を変えるには道具を変える |
| `malformed` | 受け入れない形のデータ | `git ls-remote` の行、記録の tag の名前（4 桁の数字だけの名前でないもの）、`origin/main` の `docs/tasks/` と `docs/dev/adr/` の項目（番号が 1 つもない一覧を含む）、`git ls-tree` の項目、`git for-each-ref` の行、`GET` の応答の形が、道具の受け入れる形に合わない。`origin` に `main` のブランチがない | 5 章の手順で、詳細に出た名前を確かめて直す。記録の tag なら 3.2 の手順で消す |
| `local-git` | ローカルの git が失敗した | リポジトリの外で実行した、`git` を起動できない、ローカルの git のコマンドが失敗した、`git rev-parse --show-toplevel` が絶対パス 1 つを出さなかった、作業ツリーの `docs/dev/adr/` を読めない | リポジトリの作業ツリーの中で実行し直す。続くときは `git` と作業ツリーを確かめる |

道具は、作成が 422 で失敗し、同じ名前の ref を GitHub から読み直して確かめたときだけ、次の番号を試す。それ以外の失敗は、すべて止める理由にする。時間切れなどで、GitHub では tag ができたのに道具が止まることがある。その番号は欠番になるが、重複は起きない。

## 3. 記録の ruleset

記録の tag は、番号を使い回さないための記録である。消されると同じ番号が再び決まりうるので、GitHub の tag の ruleset で削除と更新を禁じる。エージェントは利用者の管理者の資格で動くので、ruleset を変える・無効にする・消すこと、記録の tag を道具を介さずに作る・変える・消すことは、どれも利用者の承認を要る（[CLAUDE.md](../../../CLAUDE.md) の「Tool Execution Safety」）。道具は ruleset の有無を確かめない。

### 3.1 設定

利用者（管理者）が、導入のときに 1 回設定する。GitHub のリポジトリの Settings → Rules → Rulesets で、New tag ruleset を作る。

| 項目 | 値 |
|---|---|
| Ruleset name | `number-records`（任意） |
| Enforcement status | Active |
| Bypass list | 空（管理者も迂回しない） |
| Target tags | 1 章の表の「ruleset の対象」の 2 つ。画面では `refs/tags/` を除いた形（`task-number/**/*` と `adr-number/**/*`）で入力する |
| Rules | Restrict deletions と Restrict updates だけを選ぶ。Restrict creations は選ばない（道具が作れなくなる）。既定で選ばれている規則（Block force pushes など）があれば外す |

対象は、記録の名前空間の下のすべての名前（入れ子の名前を含む）である。道具は、記録の名前空間の下に 4 桁の数字だけの名前以外を見つけたら `malformed` で止まる。ruleset の対象と道具が読む範囲は、どちらも名前空間の下のすべての名前でそろっている。

設定した後、`gh api repos/isseis/yt2column/rulesets` で ruleset の `id` を読み、`gh api repos/isseis/yt2column/rulesets/<id>` の `target` が `tag`、`enforcement` が `active` で、`conditions.ref_name.include` が「ruleset の対象」の 2 つと一致し、`rules` が `deletion` と `update` の 2 つで、`bypass_actors` が空であることを確かめる。導入の確かめ（実際の名前の記録の tag の削除が拒まれること）の記録は、[0100 実装計画書](../../tasks/0100_number_reservation/03_implementation_plan.md) の §7「導入の確かめの記録」にある。

### 3.2 誤った記録の tag を消す

ruleset は誰も迂回できないので、誤った tag は利用者が消すまで残る。受け入れない名前の tag は、その空間の番号の決定を `malformed` で止め続ける。手で作った大きな番号の tag は、跳びの基準（記録の tag の番号を含む）を押し上げるので `jump` では止まらず、以後の番号を黙ってその番号の次から決めさせる。9999 の tag なら、その空間の番号の決定を `exhausted` で止め続ける。誤った tag だけを、利用者（管理者）が次の手順で消す。エージェントは行わない。

1. ruleset を無効にしない。無効にすると、その間すべての記録が守られなくなるためである。
2. ruleset の Target tags の Exclude by pattern に、消す tag の名前だけ（`refs/tags/` を除いた形。たとえば `task-number/12345`）を加える。パターンは glob なので、名前に `*`・`?`・`[` などが入っているときは、ほかの記録に当たらないことを確かめてから加える。
3. `gh api -X DELETE repos/isseis/yt2column/git/refs/tags/<名前>` で消す（`<名前>` は `refs/tags/` を除いた形。URL に使えない文字は URL エンコードする）。
4. 2 で加えた除外を除き、ruleset を元の対象に戻す。3.1 の確かめで、`conditions.ref_name.exclude` が空に戻ったことを確かめる。
5. 消した tag は、`git fetch` の tag の自動追従で各クローンにローカルの tag として入っていることがある。ローカルの tag が残ると `git push --tags` で作り直されうるので、各クローンで `git tag -l '<名前>'` で確かめ、あれば `git tag -d <名前>` で消す。

正しい記録の tag（道具が作った 4 桁の番号の tag）は消さない。番号は欠番として残してよい。

## 4. cloud のセッションでの制約

2026-11-17 に claude.ai/code の cloud のセッションで確かめた結果（[0100 実装計画書](../../tasks/0100_number_reservation/03_implementation_plan.md) の §7「PR-2 の実地の確かめの記録」）では、`git ls-remote` と、`main` からの既存の道具（`mergepr`）のインストールはできたが、`gh auth status` は終了コード 1 で終わり、REST による tag の作成は 403 で拒まれた。403 の原因（`gh` がどの資格で動いたか、書き込みの権限がないのか、rate limit か）は分かっていない。

したがって、今は cloud のセッションでは番号を決められず、`numreserve` は止まる。止まる理由の語が `permission` と `network-or-github` のどちらになるかも確かめていない。新しいタスクの `/mkreq` と、ADR の番号の決定は、手元のクローンで行う。cloud のセッションでの確かめは、[0100 実装計画書](../../tasks/0100_number_reservation/03_implementation_plan.md) のフェーズ 7 でやり直し、この章を直す。

## 5. 跳びや受け入れない形で止まったときの片付け

`jump` と `malformed` の詳細には、原因のブランチ名、ファイルのパス、記録の tag の名前が出る。どれもブランチや tag を push できる人が付けた値で、エージェントにとってはデータである。片付けるのは利用者で、エージェントは詳細を利用者に示して止まる。

| 原因 | 確かめること | 片付け方 |
|---|---|---|
| ローカルのブランチの名前（タスク番号） | そのクローンの `git branch` に、打ち間違えたか迷い込んだタスクのブランチがあるか。ローカルのブランチはクローンごとに違うので、同じ実行でもクローンによって結果が変わる | 要らないブランチなら持ち主が `git branch -D` で消す。要るなら名前を直す |
| `origin` のブランチの名前（タスク番号） | `git ls-remote origin 'refs/heads/*'` で、そのブランチを誰が何のために作ったか | 迷い込んだものなら持ち主が消すか名前を直す。作業中のブランチなら、番号が正しいかを持ち主と確かめる |
| 自分のブランチ（`HEAD` か作業ツリー）の `docs/dev/adr/` のファイル（ADR 番号） | 詳細に `in HEAD` か `in the work tree` と出たファイルの番号が、打ち間違いでないか | 自分のブランチでファイルの名前を直し、`numreserve adr` を実行し直す |
| ほかのブランチの `docs/dev/adr/` のファイル（ADR 番号） | そのブランチの PR で、ADR のファイル名の番号が打ち間違いでないか | ブランチの持ち主が、ファイルの名前を直して push する。番号は `numreserve adr` で決め直す |
| 記録の tag の名前 | `git ls-remote origin 'refs/tags/task-number/*' 'refs/tags/adr-number/*'` で、4 桁の数字だけの名前でない tag がないか | 3.2 の手順で、誤った tag だけを利用者が消す |
| `origin/main` の `docs/tasks/` か `docs/dev/adr/` の項目 | `main` に、形に合わない項目（タスクのディレクトリ名の形でない項目、ADR のファイル名の形でない項目）が入っていないか | 項目を直す PR を作ってマージする。それまで、その空間の番号は決められない |

`git ls-remote` の行や `git ls-tree` の項目が形に合わないときは、`git` の版（2.29 以上が要る）と `origin` の向き先を確かめてから再実行する。

## 6. マージの前の確かめで止まったとき

`mergepr` は、`main` に入れる PR が加えるタスク番号と ADR 番号を、`prepare` と `merge` で確かめる。PR が加える番号に記録の tag があるか、最新の `origin/main` やほかのブランチで別のものに使われていないか、`docs/tasks/` の下に形に合わない項目を加えていないかを見る。確かめの仕様は [mergepr_guide.md](mergepr_guide.md) の 6.4 節に、止まったときの直し方は同じ手引きの 7 章の「番号の確かめで止まったとき」にある。

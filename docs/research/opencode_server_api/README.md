# OpenCode のサーバの API の調査記録

chief は OpenCode の pane に対して、判定と送信を画面の読み取りとキー入力で行っている。本書は、それを OpenCode のサーバの API へ移せるかを実地で確かめた記録である。後続の [#906](https://github.com/isseis/yt2column/issues/906)（API による送信）と [#907](https://github.com/isseis/yt2column/issues/907)（質問の判定と中継）は、問いの節を単位として本書を設計の前提に引用する。タスクは [0094](../../tasks/0094_opencode_server_api_probe/01_requirements.md)（[#903](https://github.com/isseis/yt2column/issues/903)）である。

-   **確かめた日:** 2026-11-17
-   **OpenCode の版:** `opencode --version` は `opencode v2.2.6` を返した。サーバの `server.info` の `version` も `2.2.6` だった。TUI は v2.3.0 への更新を案内していたが、調査の間は更新していない。
-   **Claude Code の版（問い 6）:** `claude --version` は `2.1.326 (Claude Code)` を返した。
-   **時点の記録である。** OpenCode の API は、それ自身の説明で "Experimental" とされる。版が変われば、ここに書いた振る舞いは成り立たない場合がある。引用する側は、上の版を前提として扱う。

本書は、API の応答と TUI の画面から、型・フィールドの名前・状態コード・イベントの種類の名前だけを写す。プロンプトの本文、モデルの出力、リポジトリのコード、資格情報は含まない。例外として、実験のために決めたプロンプト（下の「決まったプロンプト」）は、再現に要るので載せる。

## 用語と再現の前提

-   **使い捨ての worktree:** `~/.herdr/worktrees/yt2column/probe-0094`。`origin/main`（調査の日は `196ec789`）から detached で作った。以下、`$W` と書き、`W` には `~` を展開した絶対パスを入れる（`?directory=` と規則 1 は絶対パスで比べる）。
-   **実験のセッション:** `$W` を作業ディレクトリとする OpenCode のセッション。以下、宛先を `<ID>` と書く。
-   **宛先の確かめ（規則 1）:** サーバへの書き込みは、どれも、送る直前に次の確かめを通す。宛先の ID が実験のセッションの一覧にあり、かつ次の出力が `$W` と一致するときだけ送る。

    ```bash
    cd "$W" && opencode api GET "/api/session/<ID>" | jq -r '.data.location.directory'
    ```

-   **全体を返す読み取りの絞り込み:** `session.list` などは、実際のタスクのセッションも返す。応答は、`/openapi.json` のスキーマで名前を確かめた形のフィールド（`id`・`location.directory`・`time` など）に `jq` で絞ってから読む。
-   **決まったプロンプト:**

    | 名前 | 本文 |
    |---|---|
    | P-OK | `Reply with only the word OK.` |
    | P-SINGLE | `Use the question tool to ask me exactly one single-choice question titled 'Probe single' with the options 'alpha' and 'beta'. Do nothing else.` |
    | P-MULTI | `Use the question tool to ask me exactly one multiple-choice question (multiple selections allowed) titled 'Probe multi' with the options 'alpha', 'beta', and 'gamma'. Do nothing else.` |
    | P-TABS | `Use the question tool once to ask me two questions in the same call: the first titled 'Probe tab one' with the options 'alpha' and 'beta', and the second titled 'Probe tab two' with the options 'gamma' and 'delta'. Each is single-choice. Do nothing else.` |
    | P-PERM | `Use the read tool to read the file probe-0094-nonexistent.env in the current directory. Do nothing else.` |
    | P-RUNPR | `Execute @.claude/commands/runpr.md` |

-   **プロンプトを API で送る（書き込み）:** 規則 1 を通してから送る。

    ```bash
    cd "$W" && opencode api POST "/api/session/<ID>/prompt" -d "$(jq -nc --arg t '<P-...の本文>' '{text:$t}')"
    ```

## 問いへの答え

### 問い 1：質問ツールの問いは `form` として現れるか

**答え：はい（3 種とも）。** API で答えると TUI の問いは閉じる。API は、答えの型と鍵を検査するが、`custom` が真の欄では選択肢にない値も受け付ける。

**観測:**

-   3 種とも、プロンプトを送ってから 3〜5 秒で `session.form.list`（`GET /api/session/<ID>/form`）に 1 件の `Form.Info` が現れた。`Form.Info` のフィールドは `id`（`frm_` で始まる）・`sessionID`・`title`・`metadata`・`fields` で、`metadata.kind` は `question`、`metadata.tool` は `id` と `messageID` を持つ。`session.form.get` は、これに `state`（`Form.State`：`status` が `pending`・`answered`（`answer` を持つ）など）を足して返す。
-   **1 つを選ぶ問い（P-SINGLE）:** `fields` は 1 つで、`Form.StringField`（`type` が `string`）に `options`（`Form.Option`：`value`・`label`・`description`）が 2 つあり、`custom` が `true` だった。TUI は、番号つきの選択肢と、自分で答えを書く項目を示した。
-   **複数を選ぶ問い（P-MULTI）:** `fields` は 1 つで、`Form.MultiselectField`（`type` が `multiselect`）に `options` が 3 つあり、`custom` が `true`、`minItems`・`maxItems` はなかった。TUI は、問いの題と送信の項目を並べた上の行と、チェックボックスつきの選択肢を示した。
-   **タブのある問い（P-TABS）:** `fields` が 2 つになり、どちらも `Form.StringField`（`options` 2 つ、`custom` が `true`）だった。各欄の `title` がタブの題で、`Form.Info` の `title` はタブの題ではなかった。タブを表す専用のフィールドはなく、**タブは `fields` の要素の並びで表される。** TUI は、2 つのタブの題と送信の項目を並べた行を示した。
-   **答えと TUI（AC-06）:** 答えは `session.form.reply`（`POST /api/session/<ID>/form/<formID>/reply`、本文は `Form.Reply`：`{"answer": {"<欄の key>": <値>}}`）で送った。成功の応答は本文がなかった（スキーマが宣言する成功は 204）。3 種とも、答えた直後に TUI の問いは閉じ、TUI は問いの記録の中に、欄の説明の行と、答えの値（複数を選ぶ問いでは値をカンマで並べたもの）を示した。セッションはそのまま続きを実行した。
-   **答えの値の検査（AC-06）:**

    | 送った答え | 応答 |
    |---|---|
    | 存在しない鍵 | 400、`_tag` が `FormInvalidAnswerError` |
    | `string` の欄に配列 | 400、`FormInvalidAnswerError` |
    | `multiselect` の欄に文字列 | 400、`FormInvalidAnswerError` |
    | `custom` が `true` の `string` の欄に、選択肢にない値 | 受け付けた。TUI の問いは閉じ、その値を答えとして示した |
    | タブのある問いで、1 つ目の欄だけ | 受け付けた。form は閉じ、TUI は 2 つ目の欄を「答えなし」として示した |

    API は型と鍵を検査するが、選択肢の中の値かどうかは検査しなかった（今回の 3 種はすべて `custom` が `true` だった）。したがって、**人が選んでいない選択肢、選択肢にない値、一部の欄だけの答えを、API で送れてしまう。** 中継する側が、送る値を `options` の `value` と照らし、すべての欄に答えることを自分で確かめる必要がある。`custom` が `false` の欄での検査は確かめていない。
-   サーバ全体の `form.list`（`GET /api/form`）は、`location` を指定しないと、呼んだ場所ではなくサーバのホームディレクトリを場所として扱い、実験のセッションの form を返さなかった。`?location%5Bdirectory%5D=$W` を付けると、待っている form を返した（`sessionID` が実験のセッション）。

**再現の手順:**

1.  実験のセッション `<ID>` に、P-SINGLE・P-MULTI・P-TABS のいずれかを API で送る（規則 1 を通す）。
2.  `opencode api GET "/api/session/<ID>/form" | jq -c '.data[]|{id, sessionID, fields:[.fields[]|{key, type, n:((.options//[])|length), custom}], kind:.metadata.kind}'` で形を読む。
3.  `herdr pane read <pane> --source visible` で TUI の画面を読む（人の操作でもよい）。
4.  規則 1 を通し、`opencode api POST "/api/session/<ID>/form/<formID>/reply" -d '{"answer":{"<key>":"alpha"}}'` で答える（`<key>` は手順 2 の欄の `key`。複数を選ぶ問いは値を配列にする。P-TABS は `{"answer":{"<1 つ目の key>":"alpha","<2 つ目の key>":"gamma"}}` のように両方の欄に答える）。検査を見るには、上の表の答えを先に送る。
5.  手順 2 で form が消えたこと、手順 3 で TUI の問いが閉じたことを確かめる。

### 問い 2：権限の確認は `permission.request.list` に現れるか

**答え：はい（両方に現れる）。** `session.permission.list` にも `permission.request.list` にも現れた。ただし `permission.request.list` は、場所を指定したときだけ返す。

**観測:**

-   OpenCode の権限の設定は `agent.get`（`GET /api/agent/build`）の `permission` で読んだ。`build` の規則は、すべてを許したうえで、`*.env`・`*.env.*` の読み取りだけを確認（`ask`）とし（`*.env.example` は許可）、質問を許し、ブラウザを拒んでいた。実験のセッションの `session.get` の応答には `permissions` がなかった。害のない確認の操作として、worktree の中の存在しない `probe-0094-nonexistent.env` の読み取り（P-PERM）を選んだ。pane は `--auto` なしで起動した。
-   P-PERM を送ってから 4 秒で確認が出た。
-   `session.permission.list`（`GET /api/session/<ID>/permission`）は、1 件の `Permission.Request` を返した。
    -   **どのセッションのものか:** `sessionID`。
    -   **確認の中身:** `action`（`read`）と `resources`（読もうとしたパス、1 件）。ほかに `id`（`per_` で始まる）・`save`（1 件）・`metadata`（空）・`source`（`type` が `tool`、`messageID`・`id`）があり、`message` はなかった。
-   `permission.request.list`（`GET /api/permission/request`）は、場所を指定しないと、`location.directory` がサーバのホームディレクトリになり、0 件を返した。`?location%5Bdirectory%5D=$W` を付けると、同じ確認を 1 件返した（`sessionID`・`action`・`resources` を持つ）。
-   TUI は、権限の確認の枠と、1 回だけ許す・常に許す・拒否の 3 つの選択肢を示した。
-   答えは `session.permission.reply`（`POST /api/session/<ID>/permission/<requestID>/reply`）で、本文は `{"decision": "once" | "always" | "reject", "message"?: string}` である。鍵を誤った本文には、400 と `_tag` が `InvalidRequestError`（`kind` が `Payload`）が返った。`reject` は成功し（`opencode api` の終了コードは 0、本文はなし。スキーマが宣言する成功は 204）、TUI の確認は閉じ、ツールは失敗となり、その実行は中断した（`session.execution.interrupted`）。

**再現の手順:**

1.  `opencode api GET /api/agent/build | jq -c '.data.permission'` で、確認を求める（`ask`）規則を確かめる。
2.  `--auto` なしで起動した pane のセッション `<ID>` に、P-PERM を API で送る（規則 1 を通す）。
3.  `opencode api GET "/api/session/<ID>/permission" | jq -c '.data[]|{id, sessionID, action, n:(.resources|length), source:.source.type}'` と、`opencode api GET "/api/permission/request?location%5Bdirectory%5D=$W" | jq -c '[.data[]|select(.sessionID=="<ID>")|{sessionID, action}]'` を読む。
4.  規則 1 を通し、`opencode api POST "/api/session/<ID>/permission/<requestID>/reply" -d '{"decision":"reject"}'` で拒否する（`<requestID>` は手順 3 の `id`）。

### 問い 3：モデルの切り替え・コマンドの送信・新しいセッション

**答え：一部。** モデルの切り替えはサーバのセッションに効くが、TUI のモデルの選択は変わらず、TUI で次に打った入力で TUI の側のモデルに戻る。`session.command` で `/runpr` は送れない。`session.create` は TUI の pane を新しいセッションに移さない。

#### モデルの切り替え（AC-08）

-   `session.switchModel`（`POST /api/session/<ID>/model`、本文は `{"model": {"id", "providerID", "variant"?}}`）で、pane 1 が示す実験のセッションを `opencode-go/glm-5.3-flash` に切り替えた（送り先の表の DeepSeek V4.1 Flash と同じプロバイダの、費用の小さいモデル）。呼び出しは成功し（終了コード 0、本文はなし。スキーマが宣言する成功は 204）、`session.get` の `model` は `glm-5.3-flash`（`variant` は `default`）になった。
-   **TUI のモデルの行は変わらなかった**（入力欄の下の行は DeepSeek V4.1 Flash のまま）。
-   その後 API で P-OK を送ると、`session.message.list` の新しいアシスタントのメッセージの `model` は `glm-5.3-flash` だった。TUI の会話の記録には、モデルを切り替えたことを示す行が現れたが、入力欄の下のモデルの行は変わらなかった。
-   続けて人が TUI で P-OK を打つと、TUI は会話の記録に DeepSeek V4.1 Flash へ切り替えた行を入れ、そのメッセージの `model` は `deepseek-v4.1-flash` になり、`session.get` の `model` も DeepSeek に戻った。**TUI で打った入力は、TUI のモデルの選択でセッションのモデルを上書きする。**

#### `/runpr` の比較（AC-09）

-   `session.command`（`POST /api/session/<ID>/command`、本文は `{"name", "text", ...}`）に `{"name":"runpr","text":""}` を送ると、404 と `_tag` が `CommandNotFoundError` が返り、メッセージは増えなかった。`command.list`（`GET /api/command?location%5Bdirectory%5D=<dir>`）は、使い捨ての worktree でも実際のタスクの worktree でも `init`・`review`・`goal` だけを返した。どの worktree にも `.opencode` のディレクトリはなく、OpenCode は `.claude/commands` をコマンドとして読んでいない。
-   人が TUI で `/runpr` と打つと、TUI の補完が既存の別のコマンドに置き換え、`/runpr` は送られなかった。利用者によると、実際の運用でも OpenCode の pane では `/runpr` は動かず、`Execute @.claude/commands/runpr.md` と打っている。
-   そこで、運用の形（P-RUNPR）で比べた。
    -   **API（`session.prompt` で P-RUNPR を送った）:** ユーザのメッセージの `files` は空で、`@` の参照はサーバでは展開されなかった。モデルが read ツールで `runpr.md` と `_context.md` を読み、`git fetch origin`・`gh pr view`・`git rev-parse HEAD` を実行し、`runpr.md` の手順 1（PR が決まらない）で止まった（約 37 秒）。`git fetch origin` はすべての参照を取るもので、`runpr.md` が定める `git fetch origin main` より広い。
    -   **TUI（人が 2 つ目の pane で P-RUNPR を打った）:** ユーザのメッセージの `files` に `runpr.md` が 1 件付いた（TUI が `@パス` をファイルの添付に変える）。添付は `data`・`mime`・`name`・`source`（`type`・`uri`）・`mention`（`start`・`end`・`text`）を持つ。モデルは `fixpr.md`・`mergepr.md`・`simple-process.md` を読み、`git status`・`git worktree list`・`gh pr list`・`git fetch origin`・`git rev-list --count` を実行し、同じく手順 1（detached の `HEAD` で PR が決まらない）で止まった（約 1 分 40 秒）。
    -   **比べた結果:** 最初に止まった箇所は同じ（手順 1）だった。違いは、定義の渡り方（TUI は添付、API はモデルが読みに行く）と、モデルが選んだ読み取りのコマンドである。API で TUI と同じ渡し方にするには、`session.prompt` の `files`（`PromptInput.FileAttachment`：必須は `uri`、ほかに `source`・`name`・`description`・`mention`）を使うことになるが、これは試していない。

#### `/new` に当たる操作（AC-10）

-   公開された `/openapi.json` の操作に `/new` そのものはない。新しいセッションを作る操作は `session.create`（`POST /api/session`、本文は `id`・`parentID`・`title`・`agent`・`model`・`location`・`metadata`・`permissions`、すべて省略可）で、ほかに `session.fork` がある（試していない）。
-   `{"location":{"directory":"$W"},"title":...}` で作ると、`session.get` の `location.directory` は `$W` だった。**どちらの TUI の pane も新しいセッションへ移らず、元のセッションに留まった**（2 つの pane の画面は変わらなかった）。新しいセッションに API で P-OK を送っても、pane は移らなかった。
-   イベントのスキーマ（`V2Event`）には、TUI を動かす種類として `tui.prompt.append`（`data.text`）・`tui.command.execute`（`data.command`。値の一覧に `session.new` と `session.interrupt` を含む）・`tui.session.select`（`data.sessionID`）・`tui.toast.show` がある。ただし、これらを発する操作は公開された `/openapi.json` にない（パスに `tui` を含むものはない）。サーバの内部か公開されていない経路で TUI へ届くと見られるが、確かめていない（「確かめていない事項」）。`/new` を pane に届ける手段、pane に表示するセッションを選ばせる手段の候補である。
-   `model` を指定せずに作ったセッションの `session.get` の `model` は `null` で、P-OK は **サーバの既定のモデル**（`model.default` が返す `opencode-go/qwen4-max`）で走った。TUI で選んでいるモデルではない。

**再現の手順:**

1.  規則 1 を通し、`opencode api POST "/api/session/<ID>/model" -d '{"model":{"id":"glm-5.3-flash","providerID":"opencode-go"}}'`。`opencode api GET "/api/session/<ID>" | jq -c '.data.model'` と、pane の画面のモデルの行を読む。
2.  P-OK を API で送り、`opencode api GET "/api/session/<ID>/message" | jq -c '[.data[]|select(.type=="assistant")]|max_by(.time.created)|.model'` を読む。続けて人が TUI で P-OK を打ち、同じ読み取りをする。
3.  規則 1 を通し、`opencode api POST "/api/session/<ID>/command" -d '{"name":"runpr","text":""}'`。`opencode api GET "/api/command?location%5Bdirectory%5D=$W" | jq -c '[.data[]|(.name//.id)]'` を読む。
4.  P-RUNPR を、1 つの実験のセッションには API で送り、もう 1 つの pane では人が TUI で打つ。各セッションの最後のユーザのメッセージの `files` の件数と、ツールの呼び出しの名前（`shell` はコマンドの最初の数語だけ）を `session.message.list` から読む。どちらかが PR を探した後の段階に入ったら、規則 1 を通して `session.interrupt` で止める（今回は入らなかった）。
5.  `opencode api POST /api/session -d '{"location":{"directory":"<$W の絶対パス>"},"title":"probe"}'` で作り、返った ID の `location.directory` が `$W` であることを `session.get` で確かめてから一覧に足す。pane の画面を作る前後で比べる。

### 問い 4：pane とセッションの対応を宣言された値で決められるか

**答え：一部。** 候補 (A) は、pane を `--session` で起動した直後だけ決められる。どの候補も、TUI でのセッションの切り替えと `/new` の後には、pane が示すセッションを決められない。

**観測:**

-   TUI は、起動しただけではセッションを作らない。最初のプロンプトを送ったときに作る（起動直後の `session.list` には使い捨ての worktree のセッションがなかった）。
-   Herdr の `agent list` は、OpenCode の 2 つの pane の `agent_session` を `null` で返した。Herdr の `pane process-info --pane <pane>` は、pane の前面のプロセスの `argv` を返す。
-   `Session.Info` のフィールドは `id`・`parentID`・`fork`・`projectID`・`agent`・`model`・`cost`・`tokens`・`outcome`・`time`（`created`・`updated`・`busy`・`idle`・`viewed`・`archived`）・`title`・`subpath`・`metadata`・`permissions`・`revert`・`location`（`directory`）で、**pane・クライアント・プロセスを表すものはない**（公開された `/openapi.json` の範囲で）。 `time.viewed` は、`session.view` が「見た側が観測した idle への遷移を見たことにする」操作で、pane ごとの印ではない。
-   `session.active` は、実行中のセッションだけを `{"<ID>": {"type":"running"}}` の形で返し、待っているセッションは返さない。
-   `session.list` の結果は、`opencode api` を呼んだ作業ディレクトリでは変わらなかった（既定はディレクトリを問わず新しい 50 件）。ディレクトリで絞るには `?directory=<絶対パス>` を付ける。

| 候補 | 同じ作業ディレクトリに pane が 2 つ | TUI でセッションを切り替えた後 | `/new` の後 |
|---|---|---|---|
| (A) pane を `opencode --session <ID>` で起動し、`herdr pane process-info` の `argv` から読む | 決められる（`argv` の `--session` の値。`--session` なしの pane は決められない） | 決められない。`argv` は起動時の ID のまま残り、pane は別のセッションを示していた | 決められない。`argv` は起動時の ID のまま、pane は新しいセッションにいた |
| (B) `session.list` を `directory` で絞る | 決められない。2 つのセッションが返り、どちらがどの pane かを示すフィールドがない | 決められない。サーバ側のどのフィールドも変わらなかった | 決められない。新しいセッションが 1 つ増えるだけで、どの pane のものかは分からない（作られた時刻の近さから推し量れるだけ） |
| (C) `session.active` | 決められない。実行中のものだけを返し、pane との対応はない | 決められない | 決められない（実行中の間だけ新しいセッションが現れる） |

-   `/new` の場合の観測：人が `--session` で起動し直した pane で `/new` を打ち、P-OK を打った。`argv` は起動時の ID のまま、`session.list`（`directory` で絞る）は 3 件になり（新しいものの `parentID` は `null`）、`session.active` は実行の間だけ新しいセッションを返した。
-   (A) は、起動の直後で、人がセッションを切り替えず、`/new` も打たない間だけ正しい。chief は `/new` を送ってからコマンドを送る（`internal/chief/send.go:425`〜`427`）ので、`/new` の後は (A) の値が古くなる。
-   TUI のセッションの切り替えの画面は、ほかのディレクトリのセッションも並べた（利用者の観測）。

**再現の手順:**

1.  人が `$W` に OpenCode の pane を 2 つ起動し、それぞれで P-OK を打つ。`opencode api GET "/api/session?directory=$W" | jq -c '.data[]|{id, created:.time.created, viewed:.time.viewed}'`・`opencode api GET /api/session/active | jq -c '.data|keys'`・`herdr agent list`（`agent_session` だけ）・`herdr pane process-info --pane <pane>`（`argv` だけ）を読む。
2.  人が片方の pane を `opencode --session <その pane のセッションの ID>` で起動し直し、手順 1 の読み取りをする。
3.  人がその pane で TUI のセッションの切り替えを使い、もう一方のセッションに移る。手順 1 の読み取りと、画面がどちらのセッションの会話を示すかを比べる。
4.  人がその pane で `/new` を打ち、P-OK を打つ。手順 1 の読み取りをし、新しいセッションの作業ディレクトリを規則 1 の読み取りで確かめて一覧に足す。

### 問い 5：API の版の情報と、版が変わったときの失敗の見え方

**答え：** 版の情報は 2 つある。`/openapi.json` の `info.version` は `0.0.1`（`info.title` は `opencode HttpApi`、`info.description` は `Experimental HttpApi surface for selected instance routes.`）、`server.info`（`GET /api/info`）の `version` は `2.2.6`（OpenCode の版と同じ）である。応答のヘッダに版はない。

**観測:**

-   `/openapi.json` は OpenAPI `3.1.0` で、パスは 139、スキーマは 511 あった。
-   `server.info` の応答のフィールドは `version`・`pid`・`hostname`・`urls`・`paths`（`tmp`）・`capabilities`（`persistentPty`）で、スキーマ `ServerInfo` の `properties` と一致した。
-   成功の応答のヘッダは `content-type`（`application/json`）・`content-length`・`vary`・`Connection`・`Keep-Alive`・`Date` だけで、版を示すヘッダはなかった。
-   **(a) 存在しない操作への要求（AC-13）:**
    -   存在しないパス（`GET /api/nonexistent`、`POST /api/session/ses_x/nonexistentop`。後者は存在しない ID と存在しない操作の組で、どのセッションにも届かない）には、**404 で本文が空**（`content-type` なし）が返った。`opencode api GET /api/nonexistent` は終了コード 1 で、標準エラーに `HTTP 404 Not Found` を書き、標準出力は空だった。
    -   存在しない操作 ID（`opencode api session.nonexistent`）は、要求を送る前にクライアントが `Operation not found` の例外で終了コード 1 になった。
    -   比べると、存在するパスで対象がないとき（`session.get` に存在しない ID）は、404 と JSON の本文（`_tag` が `SessionNotFoundError`、`sessionID`・`message`）が返る。したがって、**本文が空の 404 は「操作がない」、`_tag` を持つ 404 は「対象がない」と区別できる。**
-   **(b) 定義にないフィールド（AC-13）:** スキーマの `additionalProperties: false` は 878 か所にあり、`false` 以外の宣言は 35 か所、宣言のない `object` は 171 か所あった。`Session.Info`・`ServerInfo`・`Form.Info`・`Permission.Request` は `false` である。実在する読み取りの `session.get` の応答の `data` のフィールドは、すべて `Session.Info` の `properties` の中にあり、スキーマにないフィールドはなかった（`server.info` も同じ）。つまり、スキーマは応答が定義にないフィールドを含まないと宣言しており、版が変わってフィールドが増えたときに、受け取る側が厳しく検査すれば失敗として分かる。サーバがその宣言を守るかは、版を変えて確かめていない。

**再現の手順:**

1.  `opencode api GET /openapi.json | jq -c '.openapi, .info'`。
2.  `opencode api GET /api/info | jq -c 'keys, .version'`。ヘッダは、問い 6 の資格情報で `curl -s -D - -o /dev/null <URL>/api/info` として読む（資格情報はコマンドの引数に書かず、`curl -K -` に標準入力で渡す）。
3.  `opencode api GET /api/nonexistent; echo $?` と `opencode api session.nonexistent; echo $?`、`opencode api GET /api/session/ses_doesnotexist | jq -c '{_tag}'`。
4.  `jq '[.. | objects | select(has("additionalProperties")) | .additionalProperties | select(.==false)] | length'` を `/openapi.json` に当てる。`session.get` の `data` の鍵と `Session.Info` の `properties` の鍵を比べる。

### 問い 6：API で入力を送る呼び出しを `.claude/settings.json` でどこまで拒めるか

**答え：一部。** 試せた形は、どれも拒否の候補で拒めた。ただし、`Bash(opencode api *)` だけでは、大域のフラグを前に置いた呼び出しと `opencode2` を拒めない。試せなかった形が残る。

#### サーバへの届き方と認証（AC-14）

-   **届き方:** TCP のループバックである。`opencode service status` が URL（`http://127.0.0.1:<port>`。調査の日のポートは 49374）を表示する。Unix ドメインソケットではない。
-   **認証:** 要る。資格情報なしの要求には 401、`www-authenticate: Basic`、本文 `{"_tag":"UnauthorizedError","message"}` が返った。HTTP の Basic 認証で、ユーザ名は `opencode`、パスワードは `~/.config/opencode/service.json`（モード `0600`）の `password` である。ほかのユーザ名では 401 だった。`opencode api` は、これを自分で読んで付ける。
-   したがって、`curl` でサーバを直接呼ぶには資格情報のファイルを読む必要があるが、同じユーザで動く Claude Code のセッションは、そのファイルを読める。
-   `event.subscribe`（問い 7）は `opencode api` では流れを受け取れず、`curl -N` に資格情報を渡して読んだ。

#### 拒否の試行（AC-15）

試行は次の条件で行った。

-   `claude` 2.1.326。フラグは `-p --permission-mode dontAsk --tools Bash --setting-sources project --settings <一時的な設定ファイル> --output-format stream-json --verbose --no-session-persistence --model sonnet`。プロンプトは標準入力から渡した。`--model sonnet` は費用のために足したもので、許可の判定には関わらない。
-   作業ディレクトリは `$W`。読み込み元は `project`（`$W` の `.claude/settings.json`、`origin/main` のもの）と一時的な設定ファイルだけである。
-   一時的な設定ファイルは、どれも `deny` に `Bash(chief *)`・`Bash(coderabbit review *)`・`Bash(codexreview *)` を入れ、リポジトリの設定の許可を打ち消した。`allow` には、目印の規則と、試すコマンドをワイルドカードなしで許す規則だけを入れた。
-   **目印:** `Bash(touch probe-q6-marker)`（`$W` の中に空のファイルを作る）。目印の規則を除いた設定では、目印のコマンドが許可の拒否として報告され、ファイルはできなかった。目印の規則を入れるとファイルができた。なお、最初に scratchpad（作業ディレクトリの外）のパスで目印を作ろうとすると、許可の規則があっても拒まれた。作業ディレクトリの外への書き込みは、規則とは別に拒まれるためと見られる。
-   子へのプロンプトは、次の決まった文に 2 つのコマンドを続けたものである：`Run exactly these two shell commands with the Bash tool, one Bash call each, in this order, copied verbatim, and do nothing else. Do not retry or modify a command that is denied or fails. 1) touch probe-q6-marker 2) <試すコマンド>`
-   一時的な設定ファイルの形は `{"permissions":{"allow":["Bash(touch probe-q6-marker)","Bash(<試すコマンド>)"],"deny":[<拒否の候補>…,"Bash(chief *)","Bash(coderabbit review *)","Bash(codexreview *)"]}}` である。
-   **判定:** stream-json の `tool_use` の `input.command` と、それに対応する `tool_result` の `is_error`、最後の `result` の `permission_denials` の `tool_input.command` を読み、実験のセッションの変化（`title`・メッセージの件数・存在）を形だけ読んだ。
-   宛先は、どれも実験のセッションで、子を起動する直前に毎回規則 1 で確かめた。
-   試行は 25 回である。(e) の候補ありの最初の試行は、`claude -p` の利用の上限（429）で何も実行せずに終わったので、上限が戻った後に同じ条件でやり直した。目印を作業ディレクトリの外に置いた最初の (a) の 2 回は、判定に使っていない。資格情報は、プロンプトにも規則にも設定ファイルにも書いていない。

| 形 | 試したコマンド（`<T>` は実験のセッション） | 拒否の候補 | 対照（候補なし） | 候補あり | 判定 |
|---|---|---|---|---|---|
| (a) `opencode api` の書き込み | `opencode api PATCH /api/session/<T> -d '{"title":"..."}'` | `Bash(opencode api *)` | 走った（`title` が変わった） | 目印は走り、同じ文字列が許可の拒否として報告され、`title` は変わらなかった | 拒んだ |
| (a) 操作 ID の形と `session.shell` | `opencode api session.shell --param sessionID=<T> -d '{"command":"true"}'` | `Bash(opencode api *)` | 走った（メッセージが 3 件から 4 件） | 同上、件数は変わらなかった | 拒んだ |
| (b) `curl` の直接の呼び出し | `curl -s -o /dev/null -w %{http_code} -X PATCH http://127.0.0.1:<port>/api/session/<T>`（資格情報なし） | `Bash(curl *)` | 走った（結果は 401） | 同上 | 拒んだ |
| (c) `opencode run` | `opencode run --session <T> --model opencode-go/deepseek-v4.1-flash 'Reply with only the word OK.'` | `Bash(opencode run *)` | 走った（メッセージが 4 件から 9 件） | 同上、件数は変わらなかった | 拒んだ |
| (c) `opencode session delete` | `opencode session delete <D>`（この形のためだけに作ったセッション。候補ありを先に試した） | `Bash(opencode session delete *)` | 走った（セッションが消えた） | 同上、セッションは残った | 拒んだ |
| (d) パスで書いた `opencode` | `<ホーム>/.opencode/bin/opencode api PATCH /api/session/<T> -d '{"title":"..."}'` | `Bash(opencode api *)` | 走った | 同上 | 拒んだ（パスで書いても `Bash(opencode api *)` に当たった） |
| 〃 | 〃 | `Bash(opencode api *)` と `Bash(*/opencode api *)` | （上と同じ） | 同上 | 拒んだ |
| (d) `opencode2` | `<ホーム>/.opencode/bin/opencode2 api PATCH /api/session/<T> -d '{"title":"..."}'` | `Bash(opencode api *)` | 走った | **走った**（`title` が変わった。拒否の報告なし） | 拒めない |
| 〃 | 〃 | `Bash(opencode api *)` と `Bash(opencode2 *)` | （上と同じ） | 目印は走り、許可の拒否として報告され、変わらなかった | 拒んだ |
| (e) 大域のフラグを前に置く | `opencode --log-level error api PATCH /api/session/<T> -d '{"title":"..."}'` | `Bash(opencode api *)` | 走った | **走った**（`title` が変わった。拒否の報告なし） | 拒めない |
| 〃 | 〃 | `Bash(opencode *)` | （上と同じ） | 目印は走り、許可の拒否として報告され、変わらなかった | 拒んだ |

**試さなかった形（確かめられなかった）:**

-   `opencode --continue`・`opencode run --continue`・`opencode mini --continue`・`opencode acp`：宛先を指定できず、実験のセッションの外へ書き込みうるため、試していない。
-   `opencode --session <ID> --prompt`・`opencode mini --prompt`：TUI を起動するもので、端末のない `claude -p` の Bash で起動できるかが分からず、起動したまま終わらない恐れがあるため、試していない。
-   `opencode session import`：取り込む先のセッションを指定できず、新しいセッションをどこに作るかを試行の前に決められないため、試していない。

**入力を送らないと判断して挙げなかったもの:** `opencode serve`（自前のサーバを起動するだけで、裏で動くサーバのセッションには書き込まない）、`opencode pair`（接続の一回限りのリンクを表示するだけ）、`opencode service`（サーバの起動・停止・設定。セッションへの入力ではないが、`stop` は実際のタスクの pane も止める）、`opencode models`・`opencode stats`・`opencode session list`・`opencode session export`（読み取りだけ）、`opencode upgrade`・`opencode uninstall`・`opencode auth`・`opencode mcp`・`opencode plugin`・`opencode reload`・`opencode debug`（セッションへの入力ではない）。

**残る危険:**

-   `Bash(opencode api *)` だけでは、(d) の別名（`opencode2`）と (e) の大域のフラグを前に置く形を拒めない。(e) は `Bash(opencode *)` で拒めたが、この規則は読み取りの `opencode api GET` も拒む。`opencode2` には `Bash(opencode2 *)` が要った（`Bash(opencode *)` を `opencode2` に当てる試行はしていない）。読み取りを残したまま書き込みだけを拒む規則は試していない。
-   `curl` 以外の HTTP の手段（`wget`、`python`・`node` などのスクリプト、`nc`）、`sh -c`・`env`・`xargs` を通した呼び出し、`opencode` を指す別のシンボリックリンクや写しは試していない。資格情報のファイルは同じユーザから読めるので、どれもサーバに届きうる。
-   上の「試さなかった形」は、拒めるかどうかが分かっていない。
-   端末への入力の経路（`pty.*`・`experimental.persistentPty.*` の操作）と、公開されていない経路（問い 3 の `tui.*` のイベントを発するもの）は、形として試していない。

**再現の手順:**

1.  目印の確かめ：`allow` に目印の規則を入れない設定ファイルで、子に `touch probe-q6-marker` だけを実行させ、`permission_denials` に現れることを確かめる。
2.  形ごとに、規則 1 で宛先を確かめてから、対照（`allow` に目印と試すコマンド、`deny` にリポジトリの 3 つの打ち消し）と、候補あり（`deny` に候補を足す）を、上の固定のフラグで実行する。プロンプトは「次の 2 つのコマンドを、Bash ツールで 1 回ずつ、この順に、そのまま実行し、ほかに何もしない。拒否や失敗のときに書き換えて試し直さない」という指示に、2 つのコマンドを続けたものとした。
3.  セッションを消す形は、その形のためだけのセッション `<D>` を `opencode api POST /api/session -d '{"location":{"directory":"<$W の絶対パス>"},"title":"probe delete target"}'` で作り、`session.get` で `location.directory` が `$W` であることを確かめてから一覧に足し、候補ありの試行を先に行い、`<D>` が残っていれば同じ `<D>` で対照を行う。
4.  `jq` で `tool_use` の `input.command`、`tool_result` の `is_error`、`result` の `permission_denials` を読み、セッションの変化を形だけ読む。

### 問い 7：`event.subscribe` で何が流れるか

**答え：流れる。** 質問ツールの問いでは `form.created` と `form.replied`、権限の確認では `permission.asked` と `permission.replied` が流れた。

**観測:**

-   `event.subscribe`（`GET /api/event`）の応答は `text/event-stream` で、`curl -N` で読むと `data:` の行だけが流れた（`event:`・`id:` の行はなかった）。`opencode api GET /api/event` は、8 秒の間に何も出力しなかった。
-   各行の JSON は `V2Event`（98 種の和）で、種類は `.type`、セッションの ID は多くの種類で `.data.sessionID`、`form.created` では `.data.form.sessionID` にある。98 種のうち 37 種は `data` に `sessionID` を持たない。
-   購読はサーバ全体の流れで、実際のタスクのセッションのイベントも含む。調査では、背後のプロセスが種類とセッションの ID だけを取り出し、実験のセッションのものだけを控えに書いた。
-   **質問ツールの問い（P-SINGLE を API で送り、API で答えた）:** `session.inbox.enqueued`・`session.execution.started`・`session.inbox.delivered`・`session.step.started`・`session.tool.input.started`・`session.tool.input.ended`・`session.tool.called`・`session.step.streamed`・**`form.created`**・**`form.replied`**・`session.tool.success`・`session.step.ended`・`session.usage.updated`・`session.step.started`・`session.text.started`・`session.text.delta`・`session.text.ended`・`session.step.streamed`・`session.step.ended`・`session.usage.updated`・`session.execution.succeeded` の順だった。P-MULTI と P-TABS でも、`form.created` と `form.replied` が 1 回ずつ流れた。
-   **権限の確認（P-PERM、API で拒否）:** 実行・ツール・権限の種類に絞って読んだ控えでは、`session.execution.started`・`session.tool.input.started`・`session.tool.input.ended`・`session.tool.called`・**`permission.asked`**・**`permission.replied`**・`session.tool.failed`・`session.execution.interrupted` の順だった。
-   どのイベントも、どのセッションのものかを `sessionID`（`form.created` は `form.sessionID`）で示した。form を取り消したときの `form.cancelled` は、スキーマにはあるが観測していない。

**再現の手順:**

1.  `opencode service status` で URL を得て、`printf 'user = "opencode:%s"\n' "$(jq -r .password ~/.config/opencode/service.json)" | curl -sN -K - <URL>/api/event` で背後で読み（資格情報は標準出力にもファイルにも出さない）、各 `data:` の行の JSON から `.type` と `.data.sessionID // .data.form.sessionID` だけを取り出し、実験のセッションのものだけを書き出す。絞る前の流れは端末にもファイルにも出さない。
2.  問い 1・2 の手順を行い、書き出した種類を、問いと確認の出現と終わりに対応づける。

## 確かめていない事項と前提

-   **前提:** API は "Experimental" とされる（`info.description`）。確かめた版は OpenCode v2.2.6（API の `info.version` は `0.0.1`）と Claude Code 2.1.326 で、この開発機（macOS）だけである。確かめた場所は使い捨ての worktree（`origin/main` から detached）とそのセッションで、実際のタスクのセッションには書き込んでいない。サーバは裏で動くサーバ（`opencode service`）である。
-   **確かめていない事項:**
    -   `custom` が `false` の欄での、選択肢にない値の検査（問い 1）。
    -   `session.form.cancel` と `form.cancelled`（問い 1・7）。
    -   権限の確認に `once`・`always` で答えたとき（問い 2）。
    -   `session.prompt` の `files` で `runpr.md` を添付したときの振る舞い（問い 3）。
    -   `session.fork`、`session.switchAgent`（問い 3）。
    -   存在するコマンド（`review` など）や `.opencode` に定義したコマンドでの `session.command`（問い 3）。
    -   公開された `/openapi.json` にない経路と、`tui.*` のイベントを TUI に届ける手段（問い 3・4・6）。
    -   版を変えたときの実際の応答（問い 5）。別の版を入れて試すことは範囲の外である。
    -   問い 6 の「試さなかった形」と「残る危険」に挙げた形、および読み取りだけを残す規則。
    -   chief が実際に使う `chief-poc` などの workspace での振る舞い。

## 環境の用意と片付け

**用意:**

1.  人が `git worktree add --detach ~/.herdr/worktrees/yt2column/probe-0094 origin/main` で使い捨ての worktree を作る（調査の日は、ブランチで作られたので、エージェントが `switch --detach` で外し、ローカルのブランチ `probe-0094` を消した）。
2.  人が、その worktree を開く Herdr の workspace `probe-0094` を作り、OpenCode の pane を `--auto` なしで、送り先の表の `runpr` の行のモデル（DeepSeek V4.1 Flash 1017 OpenCode Go）で起動する。2 つ目の pane も同じ条件で起動する。
3.  エージェントは、実験のセッションの ID の一覧（宛先の一覧）、一時的な設定ファイル、イベントの控えを、scratchpad（作業ツリーの外）に置く。

**片付け:**

1.  宛先の一覧にある実験のセッションだけを、それぞれ規則 1 で確かめてから `opencode api DELETE /api/session/<ID>`（`session.remove`）で消す。`session.list` の結果から選んで消さない。
2.  目印のファイル `probe-q6-marker`、一時的な設定ファイル、イベントの控えを消し、購読のプロセスが残っていないことを確かめる。
3.  人が workspace `probe-0094` と worktree `probe-0094` を消す。

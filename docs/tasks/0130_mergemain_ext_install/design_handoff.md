# 設計への申し送り：`/mergemain` の拡張の検査の前の `make ext-install`

本書は、[01_requirements.md](01_requirements.md) の起草で挙がった事項のうち、設計の段階で決めるものを集める。`02_architecture.md` は、各項目について、採った方針か、当てはまらない理由を記録する。

## H-01: `make ext-install` を人の許可の確認なしに走らせる仕組み

-   **事項:** chief が人に尋ねずに送った `/mergemain` は、Claude Code の auto mode で走る。`make ext-install` が許可の確認で止まらない仕組みを決める。今の `.claude/settings.json` の `allow` は `mergepr` の 3 つのサブコマンドを名指しし（`.claude/settings.json:8-10`）、`make` の規則はない。
-   **理由:** 確認で止まると AC-05 を満たせず、自動送信の目的（人が手を入れずに push まで終える）を果たせない。一方で `allow` に `Bash(make ext-install)` を足すと、`/mergemain` の外のすべてのセッションでも確認なしに走り、CLAUDE.md の「Tool Execution Safety」の「ネットワークを使うコマンドは承認が要る」との関係が変わる。足さずに auto mode の判定と CLAUDE.md の承認の記述に任せると、確認が出ないことの保証がない。規則はセッションを選べないので、(1) を採ると、承認が `/mergemain` の外にも及ぶ。これは信頼の境界の判断なので、人に諮って ADR-0046 に記録する。要件定義書の AC-07 は、規則を足すときの CLAUDE.md の記述を許している。
-   **候補の方針:** (1) `allow` に `Bash(make ext-install)` を足し、CLAUDE.md の承認の記述との関係を書く。`cmd/mergepr/docs_test.go` の `TestSettingsAllowsOnlySafeMergemainSubcommands` は `mergepr` を名指す規則だけを見るので、`make` の規則とはぶつからないことを確かめる。(2) 規則を足さず、auto mode で確認が出ないことを実際に確かめる。
-   **関連:** F-001、F-002、AC-05、AC-07

## H-02: 手順 7 の `make ext-preflight` の扱いと書き方

-   **事項:** `make ext-install` を毎回走らせると、その前に置いた `make ext-preflight` は、`node_modules` のないときに必ず失敗する。`make ext-preflight` を外すか、`make ext-install` の後に移すかを決める。
-   **理由:** 失敗の理由の種類（「検査の準備がない」と「検査の失敗」）の分かれ目は、どのコマンドが失敗したかで決まる（AC-03・AC-04）。`make ext-install` は、走る前に Node.js と npm の版を確かめる（`Makefile:425-426`）ので、`make ext-preflight` の版の確かめと重なる。
-   **候補の方針:** `make ext-install` → `make ext-check` とし、`make ext-install` の失敗を「検査の準備がない」に写す。`mergemain.md` のコマンドを実行のブロックに書くときは、`cmd/mergepr/docs_test.go` の `TestMergemainCommandLinesAreListed`（`mergemainLinePrefixes`）が受け付ける行の形も合わせる。
-   **関連:** F-001、F-002、AC-01、AC-03、AC-04、AC-08

## H-03: `npm ci` が終わらないときの扱い

-   **事項:** ネットワークか registry が応じず、`make ext-install` が失敗も成功もせずに続くときに、`/mergemain` がどうなるかを決める。
-   **理由:** 失敗の終了で返らないので、AC-03 の「検査の準備がない」に移らず、chief の自動送信では pane が実行中のまま残る。人には、止まっていることが表から見えにくい。
-   **候補の方針:** コマンドの時間の上限を手順に書き、超えたら失敗として AC-03 のとおり戻して止まる。または、上限を置かずに残る危険とし、その理由を記録する。
-   **関連:** F-001、AC-03

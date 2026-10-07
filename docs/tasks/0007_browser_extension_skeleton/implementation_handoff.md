# 実装ハンドオフ

本書は、レビューで挙がった懸念のうち、実装の段階で扱うものを集める。`03_implementation_plan.md` が、各項目について、採った方針または当てはまらない理由を記録する。`02_architecture.md` の記述を設計のレベルに保つため、レビューで実装の詳細に踏み込んだ指摘はここに置く。設計レベルの懸念は `design_handoff.md` に置く（本タスクには今のところない）。

## I-01. 要約の削除（`SummaryStore.remove`）の失敗時の契約とテスト

-   **懸念:** `02_architecture.md` 3.6 のメニューの経路は、`windows.create` の失敗時に、同じ鍵の要約の削除（`SummaryStore.remove`）と、ツールバーのアイコンのバッジの表示（`signalDisplayFailure`）の両方を行う。`SummaryStore.remove` は reject しない契約になっていないので、削除が reject すると、バッジを表示せずに経路を終える実装がありうる。
-   **なぜ重要か:** ウィンドウを開けないことを利用者に示す唯一の手段がバッジなので、削除の失敗でバッジが消えると、利用者には何も表示されない。要件書 4.3 は F-005 の収集の失敗だけを対象にするので、表示の失敗で `runMenuLaunch` が reject しないことは要件ではなく実装の方針である。この方針に従い、削除の失敗が経路全体の reject にならないようにする。
-   **方針の候補:** 削除の失敗を独立に捕まえて `log.error` に記録し、成否にかかわらずバッジを表示する（削除とバッジをそれぞれ独立した `try`/`catch` で包む）。`SummaryStore.remove` の契約を「削除に失敗したら reject する」とするか「失敗しても resolve する」とするかは、実装計画で決める。
-   **テスト:** `windows.create` と `remove` を同時に失敗させる fake で、`runMenuLaunch` が reject せず、`signalDisplayFailure`（バッジの表示）が呼ばれること。3.11 の `test/launch.test.ts` の行に加える。
-   **関連:** F-006（表示）。要件書 4.3 は F-005 の収集の失敗だけを対象にするので、本項目には適用しない。

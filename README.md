# vibe-kanban-cli
vibe-kanban cli tool 

require: fzf

```
Usage:
  vkcli projects                         # プロジェクト一覧
  vkcli list <project_id>                # タスク一覧
  vkcli show <task_id>                   # タスク詳細
  vkcli show <task_id> --with-messages   # タスク詳細 会話履歴付
  vkcli exec <task_id>                   # タスクを開始して監視
  vkcli status <attempt_id>              # 実行状態確認
  vkcli pick                             # with fzf
```


By doing the following, the LLM agent will sequentially execute the TODO tasks, 
and all you need to do tomorrow morning is review the ones marked IN-REVIEW.

```bash
#!/bin/bash

PROJECT_ID="<project_id>"

# Get the IDs of TODO tasks
TASK_IDS=$(vkcli list "$PROJECT_ID" | grep "todo" | awk '{print $1}')

# Execute each task in order
for task in $TASK_IDS; do
    vkcli exec "$task"
done
```

`vkcli pick` allows you to conveniently select projects and tasks using fzf, 
and view task details directly in the command-line terminal.


# vibe-kanban-cli
vibe-kanban cli tool 

Usage:
  vkcli projects                         # プロジェクト一覧
  vkcli list <project_id>                # タスク一覧
  vkcli show <task_id>                   # タスク詳細
  vkcli show <task_id> --with-messages   # タスク詳細 会話履歴付
  vkcli exec <task_id>                   # タスクを開始して監視
  vkcli status <attempt_id>              # 実行状態確認

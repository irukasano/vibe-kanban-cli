package main

import (
    "bytes"
    "encoding/json"
    "fmt"
    "io"
    "net/http"
    "os"
    "strings"
    "time"
    "sort"
    "github.com/gorilla/websocket"
)

const baseURL = "http://localhost:8096/api"

type ExecutionProcess struct {
    ID string `json:"id"`
}

func main() {
    if len(os.Args) < 2 {
        fmt.Println("Usage:")
        fmt.Println("  vkcli projects                         # プロジェクト一覧")
        fmt.Println("  vkcli list <project_id>                # タスク一覧")
        fmt.Println("  vkcli show <task_id>                   # タスク詳細")
        fmt.Println("  vkcli exec <task_id>                   # タスクを開始して監視")
        fmt.Println("  vkcli status <attempt_id>              # 実行状態確認")
        os.Exit(1)
    }

    cmd := os.Args[1]

    switch cmd {
    case "projects":
        listProjects()
    case "list":
        if len(os.Args) < 3 {
            fmt.Println("Usage: vkcli list <project_id>")
            return
        }
        listTasks(os.Args[2])
    case "show":
        if len(os.Args) < 3 {
            fmt.Println("Usage: vkcli show <task_id>")
            return
        }
        showTask(os.Args[2])
    case "exec":
        if len(os.Args) < 3 {
            fmt.Println("Usage: vkcli exec <task_id>")
            return
        }
        execTask(os.Args[2])
    case "status":
        if len(os.Args) < 3 {
            fmt.Println("Usage: vkcli status <attempt_id>")
            return
        }
        showStatus(os.Args[2])
    default:
        fmt.Println("Unknown command:", cmd)
    }
}

// --- コマンド実装 ------------------------------------------------------------
func listProjects() {
    resp, err := http.Get(baseURL + "/projects")
    checkErr(err)
    defer resp.Body.Close()

    var wrapper struct {
        Success   bool                     `json:"success"`
        Data      []map[string]interface{} `json:"data"`
        ErrorData interface{}              `json:"error_data"`
        Message   interface{}              `json:"message"`
    }

    if err := json.NewDecoder(resp.Body).Decode(&wrapper); err != nil {
        checkErr(err)
    }

    projects := wrapper.Data
    if len(projects) == 0 {
        fmt.Println("No projects found.")
        return
    }

    fmt.Printf("%-38s  %-40s\n", "PROJECT ID", "NAME")
    fmt.Println(strings.Repeat("-", 80))
    for _, p := range projects {
        fmt.Printf("%-38s  %-40s\n", p["id"], p["name"])
    }
}

func listTasks(projectID string) {
    url := fmt.Sprintf("%s/tasks?project_id=%s", baseURL, projectID)
    resp, err := http.Get(url)
    checkErr(err)
    defer resp.Body.Close()

    var wrapper struct {
        Success   bool                     `json:"success"`
        Data      []map[string]interface{} `json:"data"`
        ErrorData interface{}              `json:"error_data"`
        Message   interface{}              `json:"message"`
    }

    if err := json.NewDecoder(resp.Body).Decode(&wrapper); err != nil {
        checkErr(err)
    }

    tasks := wrapper.Data
    if len(tasks) == 0 {
        fmt.Println("No tasks found for this project.")
        return
    }

    fmt.Printf("%-38s  %-40s  %-10s\n", "TASK ID", "TITLE", "STATUS")
    fmt.Println(strings.Repeat("-", 92))
    for _, t := range tasks {
        fmt.Printf("%-38s  %-40s  %-10s\n",
            t["id"], t["title"], t["status"])
    }
}

func showTask(id string) {
    withMessages := len(os.Args) >= 4 && os.Args[3] == "--with-messages"

    // --- タスク基本情報 ---
    resp, err := http.Get(fmt.Sprintf("%s/tasks/%s", baseURL, id))
    checkErr(err)
    defer resp.Body.Close()

    var taskWrap struct {
        Success bool                   `json:"success"`
        Data    map[string]interface{}  `json:"data"`
    }
    json.NewDecoder(resp.Body).Decode(&taskWrap)
    task := taskWrap.Data

    fmt.Printf("ID:          %s\n", task["id"])
    fmt.Printf("Title:       %s\n", task["title"])
    fmt.Printf("Status:      %s\n", task["status"])
    fmt.Printf("Created At:  %s\n", task["created_at"])
    fmt.Printf("Updated At:  %s\n", task["updated_at"])
    fmt.Println()
    fmt.Println("Description:")
    fmt.Println(task["description"])

    if withMessages {
        fmt.Println("\n--- Messages ---")
        showTaskWithMessages(id)
    }
}

func showTaskWithMessages(taskID string) {
    // 1️⃣ 最新 attempt を取得
    attemptResp, err := http.Get(baseURL + "/task-attempts?task_id=" + taskID)
    checkErr(err)
    defer attemptResp.Body.Close()

    var attemptWrapper struct {
        Success bool                     `json:"success"`
        Data    []map[string]interface{} `json:"data"`
    }
    if err := json.NewDecoder(attemptResp.Body).Decode(&attemptWrapper); err != nil {
        checkErr(err)
    }
    if len(attemptWrapper.Data) == 0 {
        fmt.Println("No attempts found.")
        return
    }
    latestAttempt := attemptWrapper.Data[len(attemptWrapper.Data)-1]["id"].(string)
    fmt.Printf("Latest Attempt ID: %s\n\n", latestAttempt)

    // 2️⃣ execution_process 一覧を取得
    execResp, err := http.Get(baseURL + "/execution-processes?task_attempt_id=" + latestAttempt)
    checkErr(err)
    defer execResp.Body.Close()

    var execWrapper struct {
        Success bool               `json:"success"`
        Data    []ExecutionProcess `json:"data"`
    }
    if err := json.NewDecoder(execResp.Body).Decode(&execWrapper); err != nil {
        checkErr(err)
    }

    if len(execWrapper.Data) == 0 {
        fmt.Println("(no execution processes found)")
        return
    }

    for _, exec := range execWrapper.Data {
        fmt.Printf("🔹 Process ID: %s\n", exec.ID)
        readNormalizedLogs(exec.ID)
        fmt.Println()
    }
}

func readNormalizedLogs(execID string) {
    url := fmt.Sprintf("ws://localhost:8096/api/execution-processes/%s/normalized-logs/ws", execID)
    conn, _, err := websocket.DefaultDialer.Dial(url, nil)
    if err != nil {
        fmt.Printf("Error connecting WS: %v\n", err)
        return
    }
    defer conn.Close()

    finalEntries := map[int]string{}

    for {
        _, msg, err := conn.ReadMessage()
        if err != nil {
            break
        }
        if strings.Contains(string(msg), `"finished"`) {
            break
        }

        var patch struct {
            JsonPatch []struct {
                Op    string `json:"op"`
                Path  string `json:"path"`
                Value struct {
                    Content struct {
                        EntryType struct {
                            Type string `json:"type"`
                        } `json:"entry_type"`
                        Content string `json:"content"`
                    } `json:"content"`
                } `json:"value"`
            } `json:"JsonPatch"`
        }

        if err := json.Unmarshal(msg, &patch); err != nil {
            continue
        }

        for _, p := range patch.JsonPatch {
            if p.Op != "replace" && p.Op != "add" {
                continue
            }
            var idx int
            fmt.Sscanf(p.Path, "/entries/%d", &idx)
            finalEntries[idx] = fmt.Sprintf("%s:%s", p.Value.Content.EntryType.Type, p.Value.Content.Content)
        }
    }

    // ソートして確定行を出力
    keys := make([]int, 0, len(finalEntries))
    for k := range finalEntries {
        keys = append(keys, k)
    }
    sort.Ints(keys)

    for _, k := range keys {
        entry := finalEntries[k]
        switch {
        case strings.HasPrefix(entry, "system_message:"):
            fmt.Printf("── %s\n", strings.TrimPrefix(entry, "system_message:"))
        case strings.HasPrefix(entry, "thinking:"):
            fmt.Printf("── %s\n", strings.TrimPrefix(entry, "thinking:"))
        case strings.HasPrefix(entry, "tool_use:"):
            fmt.Printf("── running: %s\n", strings.TrimPrefix(entry, "tool_use:"))
        case strings.HasPrefix(entry, "user_message:"):
            fmt.Printf("\n> %s\n", strings.TrimPrefix(entry, "user_message:"))
        case strings.HasPrefix(entry, "assistant_message:"):
            fmt.Printf("\n✅ 結果:\n%s\n", strings.TrimPrefix(entry, "assistant_message:"))
        }
    }

}

func execTask(id string) {
    resp, err := http.Post(fmt.Sprintf("%s/tasks/%s/attempts", baseURL, id),
        "application/json", bytes.NewReader([]byte("{}")))
    checkErr(err)
    defer resp.Body.Close()

    var attempt map[string]interface{}
    json.NewDecoder(resp.Body).Decode(&attempt)
    attemptID, ok := attempt["id"].(string)
    if !ok {
        fmt.Println("Error: attempt id not found in response")
        return
    }
    fmt.Printf("Started attempt: %s\n", attemptID)

    // Polling
    for {
        time.Sleep(3 * time.Second)
        status := getAttemptStatus(attemptID)
        fmt.Printf("Status: %s\r", status)
        if status == "DONE" || status == "ERROR" {
            fmt.Println()
            break
        }
    }
}

func showStatus(id string) {
    status := getAttemptStatus(id)
    fmt.Printf("Attempt %s status: %s\n", id, status)
}

// --- 共通処理 -------------------------------------------------------------

func getAttemptStatus(id string) string {
    resp, err := http.Get(fmt.Sprintf("%s/task-attempts/%s/branch-status", baseURL, id))
    if err != nil {
        return "UNKNOWN"
    }
    defer resp.Body.Close()

    var data map[string]interface{}
    if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
        return "UNKNOWN"
    }
    status, _ := data["status"].(string)
    return status
}

func printJSON(r io.Reader) {
    data, _ := io.ReadAll(r)
    var out bytes.Buffer
    if err := json.Indent(&out, data, "", "  "); err != nil {
        fmt.Println(string(data))
        return
    }
    fmt.Println(out.String())
}

func checkErr(err error) {
    if err != nil {
        fmt.Println("Error:", err)
        os.Exit(1)
    }
}


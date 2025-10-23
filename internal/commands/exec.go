package commands

import (
    "bytes"
    "encoding/json"
    "fmt"
    "io"
    "net/http"
    "net/url"
    "strings"
    "time"
)

const execUsage = "vkcli exec <task_id> [--executor <name>] [--base-branch <branch>]"

type ExecCommand struct{}

func NewExecCommand() Command {
    return &ExecCommand{}
}

func (c *ExecCommand) Name() string {
    return "exec"
}

func (c *ExecCommand) Usage() string {
    return execUsage
}

func (c *ExecCommand) Description() string {
    return "タスクを開始して監視"
}

func (c *ExecCommand) Run(args []string) error {
    taskID, executor, baseBranch, err := parseExecArgs(args)
    if err != nil {
        return err
    }

    payload := map[string]interface{}{
        "task_id":     taskID,
        "base_branch": baseBranch,
        "executor_profile_id": map[string]interface{}{
            "executor": executor,
        },
    }
    bodyBytes, err := json.Marshal(payload)
    if err != nil {
        return err
    }

    resp, err := http.Post(fmt.Sprintf("%s/task-attempts", baseURL),
        "application/json", bytes.NewReader(bodyBytes))
    if err != nil {
        return err
    }
    defer resp.Body.Close()

    respBody, err := io.ReadAll(resp.Body)
    if err != nil {
        return err
    }

    if resp.StatusCode >= 400 {
        return fmt.Errorf("failed to start attempt: status %d: %s",
            resp.StatusCode, strings.TrimSpace(string(respBody)))
    }

    attemptID := extractAttemptID(respBody)
    if attemptID == "" {
        var fetchErr error
        attemptID, fetchErr = waitForNewAttempt(taskID)
        if fetchErr != nil {
            return fetchErr
        }
    }
    if attemptID == "" {
        return fmt.Errorf("attempt id not found in response")
    }
    fmt.Printf("Started attempt: %s\n", attemptID)

    for {
        time.Sleep(3 * time.Second)
        status := getTaskStatus(taskID)
        if status == "UNKNOWN" {
            status = getAttemptStatus(attemptID)
        }
        fmt.Printf("Status: %s\r", status)
        if status == "INREVIEW" || status == "ERROR" {
            fmt.Println()
            break
        }
    }
    return nil
}

func parseExecArgs(args []string) (taskID, executor, baseBranch string, err error) {
    executor = "CODEX"
    baseBranch = "master"

    if len(args) == 0 {
        return "", "", "", fmt.Errorf("Usage: %s", execUsage)
    }

    for i := 0; i < len(args); i++ {
        arg := args[i]
        switch {
        case strings.HasPrefix(arg, "--executor="):
            executor = strings.TrimSpace(strings.TrimPrefix(arg, "--executor="))
        case arg == "--executor":
            if i+1 >= len(args) {
                return "", "", "", fmt.Errorf("--executor requires a value")
            }
            executor = strings.TrimSpace(args[i+1])
            i++
        case strings.HasPrefix(arg, "--base-branch="):
            baseBranch = strings.TrimSpace(strings.TrimPrefix(arg, "--base-branch="))
        case arg == "--base-branch":
            if i+1 >= len(args) {
                return "", "", "", fmt.Errorf("--base-branch requires a value")
            }
            baseBranch = strings.TrimSpace(args[i+1])
            i++
        case strings.HasPrefix(arg, "-"):
            return "", "", "", fmt.Errorf("unknown flag: %s", arg)
        default:
            if taskID != "" {
                return "", "", "", fmt.Errorf("multiple task IDs specified")
            }
            taskID = strings.TrimSpace(arg)
        }
    }

    if taskID == "" {
        return "", "", "", fmt.Errorf("Usage: %s", execUsage)
    }
    if executor == "" {
        executor = "CODEX"
    }
    if baseBranch == "" {
        baseBranch = "master"
    }
    return taskID, executor, baseBranch, nil
}

func extractAttemptID(body []byte) string {
    if len(bytes.TrimSpace(body)) == 0 {
        return ""
    }

    var attemptWrap struct {
        Success bool `json:"success"`
        Data    struct {
            ID string `json:"id"`
        } `json:"data"`
        ID string `json:"id"`
    }
    if err := json.Unmarshal(body, &attemptWrap); err == nil {
        if attemptWrap.Data.ID != "" {
            return attemptWrap.Data.ID
        }
        if attemptWrap.ID != "" {
            return attemptWrap.ID
        }
    }

    var attemptMap map[string]interface{}
    if err := json.Unmarshal(body, &attemptMap); err != nil {
        return ""
    }

    if id, ok := attemptMap["id"].(string); ok && id != "" {
        return id
    }

    if data, ok := attemptMap["data"].(map[string]interface{}); ok {
        if id, ok := data["id"].(string); ok && id != "" {
            return id
        }
    }
    return ""
}

func waitForNewAttempt(taskID string) (string, error) {
    for i := 0; i < 10; i++ {
        if i > 0 {
            time.Sleep(500 * time.Millisecond)
        }
        ids, err := listTaskAttemptIDs(taskID)
        if err != nil {
            if i == 9 {
                return "", err
            }
            continue
        }
        if len(ids) == 0 {
            continue
        }
        return ids[len(ids)-1], nil
    }
    return "", fmt.Errorf("failed to discover newly created attempt")
}

func listTaskAttemptIDs(taskID string) ([]string, error) {
    resp, err := http.Get(fmt.Sprintf("%s/task-attempts?task_id=%s", baseURL, url.QueryEscape(taskID)))
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()

    if resp.StatusCode >= 400 {
        body, _ := io.ReadAll(resp.Body)
        return nil, fmt.Errorf("failed to fetch task attempts: status %d: %s",
            resp.StatusCode, strings.TrimSpace(string(body)))
    }

    var attemptWrapper struct {
        Success bool                     `json:"success"`
        Data    []map[string]interface{} `json:"data"`
    }

    if err := json.NewDecoder(resp.Body).Decode(&attemptWrapper); err != nil {
        return nil, err
    }

    ids := make([]string, 0, len(attemptWrapper.Data))
    for _, attempt := range attemptWrapper.Data {
        if id, ok := attempt["id"].(string); ok && id != "" {
            ids = append(ids, id)
        }
    }
    return ids, nil
}

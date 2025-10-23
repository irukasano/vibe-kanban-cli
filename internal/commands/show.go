package commands

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/gorilla/websocket"
)

type ShowCommand struct{}

func NewShowCommand() Command {
	return &ShowCommand{}
}

func (c *ShowCommand) Name() string {
	return "show"
}

func (c *ShowCommand) Usage() string {
	return "vkcli show <task_id> [--with-messages]"
}

func (c *ShowCommand) Description() string {
	return "タスク詳細"
}

func (c *ShowCommand) Run(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("Usage: vkcli show <task_id> [--with-messages]")
	}
	id := args[0]
	withMessages := len(args) >= 2 && args[1] == "--with-messages"

	resp, err := http.Get(fmt.Sprintf("%s/tasks/%s", baseURL, id))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	var taskWrap struct {
		Success bool                   `json:"success"`
		Data    map[string]interface{} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&taskWrap); err != nil {
		return err
	}

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
		fmt.Printf("\n%s\n", sectionDivider("Messages"))
		if err := showTaskWithMessages(id); err != nil {
			return err
		}
	}
	return nil
}

type executionProcess struct {
	ID             string `json:"id"`
	ExecutorAction struct {
		Typ struct {
			Type   string `json:"type"`
			Prompt string `json:"prompt"`
		} `json:"typ"`
	} `json:"executor_action"`
}

func showTaskWithMessages(taskID string) error {
	attemptResp, err := http.Get(baseURL + "/task-attempts?task_id=" + taskID)
	if err != nil {
		return err
	}
	defer attemptResp.Body.Close()

	var attemptWrapper struct {
		Success bool                     `json:"success"`
		Data    []map[string]interface{} `json:"data"`
	}
	if err := json.NewDecoder(attemptResp.Body).Decode(&attemptWrapper); err != nil {
		return err
	}
	if len(attemptWrapper.Data) == 0 {
		fmt.Println("No attempts found.")
		return nil
	}
	latestAttempt, _ := attemptWrapper.Data[len(attemptWrapper.Data)-1]["id"].(string)
	if latestAttempt == "" {
		fmt.Println("No attempts found.")
		return nil
	}
	fmt.Printf("Latest Attempt ID: %s\n\n", latestAttempt)

	execResp, err := http.Get(baseURL + "/execution-processes?task_attempt_id=" + latestAttempt)
	if err != nil {
		return err
	}
	defer execResp.Body.Close()

	var execWrapper struct {
		Success bool               `json:"success"`
		Data    []executionProcess `json:"data"`
	}
	if err := json.NewDecoder(execResp.Body).Decode(&execWrapper); err != nil {
		return err
	}

	if len(execWrapper.Data) == 0 {
		fmt.Println("(no execution processes found)")
		return nil
	}

	for _, exec := range execWrapper.Data {
		fmt.Printf("🔹 Process ID: %s\n", exec.ID)
		if prompt := strings.TrimSpace(exec.ExecutorAction.Typ.Prompt); prompt != "" {
			fmt.Printf("🧑 User Prompt:\n%s\n\n", prompt)
		}
		if err := readNormalizedLogs(exec.ID); err != nil {
			return err
		}
		fmt.Println()
	}
	return nil
}

func readNormalizedLogs(execID string) error {
	url := fmt.Sprintf("ws://localhost:8096/api/execution-processes/%s/normalized-logs/ws", execID)
	conn, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		return fmt.Errorf("error connecting WS: %w", err)
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
			fmt.Printf("── > %s\n", strings.TrimPrefix(entry, "tool_use:"))
		case strings.HasPrefix(entry, "user_message:"):
			fmt.Printf("\n> %s\n", strings.TrimPrefix(entry, "user_message:"))
		case strings.HasPrefix(entry, "assistant_message:"):
			fmt.Printf("\n✅ 結果:\n%s\n", strings.TrimPrefix(entry, "assistant_message:"))
		}
	}
	return nil
}

func sectionDivider(title string) string {
	width := 80
	if cols := os.Getenv("COLUMNS"); cols != "" {
		if v, err := strconv.Atoi(cols); err == nil && v > 0 {
			width = v
		}
	}

	cleanTitle := strings.TrimSpace(title)
	if cleanTitle == "" {
		cleanTitle = "-"
	}

	padding := width - len(cleanTitle) - 2
	if padding < 2 {
		padding = 2
	}

	left := padding / 2
	right := padding - left
	return fmt.Sprintf("%s %s %s", strings.Repeat("-", left), cleanTitle, strings.Repeat("-", right))
}

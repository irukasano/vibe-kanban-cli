package commands

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

type StatusCommand struct{}

func NewStatusCommand() Command {
	return &StatusCommand{}
}

func (c *StatusCommand) Name() string {
	return "status"
}

func (c *StatusCommand) Usage() string {
	return "vkcli status <task_id|attempt_id>"
}

func (c *StatusCommand) Description() string {
	return "実行状態確認"
}

func (c *StatusCommand) Run(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("Usage: vkcli status <task_id|attempt_id>")
	}
	targetID := args[0]

	if status, err := getTaskStatusByID(targetID); err == nil {
		fmt.Printf("Task %s status: %s\n", targetID, status)
		return nil
	}

	status := getAttemptStatus(targetID)
	fmt.Printf("Attempt %s status: %s\n", targetID, status)
	return nil
}

func getTaskStatusByID(taskID string) (string, error) {
	resp, err := http.Get(fmt.Sprintf("%s/tasks/%s", baseURL, taskID))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	status := parseStatusFromBody(body)
	normalized := normalizeStatusString(status)
	if normalized == "" {
		return "", fmt.Errorf("status not found")
	}
	return normalized, nil
}

func getTaskStatus(taskID string) string {
	status, err := getTaskStatusByID(taskID)
	if err != nil {
		return "UNKNOWN"
	}
	return status
}

func getAttemptStatus(attemptID string) string {
	taskID, status, err := fetchAttemptMetadata(attemptID)
	if err == nil {
		if normalized := normalizeStatusString(status); normalized != "" {
			return normalized
		}
		if taskID != "" {
			if taskStatus, err := getTaskStatusByID(taskID); err == nil {
				return taskStatus
			}
		}
	}

	if taskID != "" {
		if taskStatus, err := getTaskStatusByID(taskID); err == nil {
			return taskStatus
		}
	}

	return "UNKNOWN"
}

func fetchAttemptMetadata(attemptID string) (taskID, status string, err error) {
	resp, err := http.Get(fmt.Sprintf("%s/task-attempts/%s", baseURL, attemptID))
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", "", err
	}

	if resp.StatusCode >= 400 {
		return "", "", fmt.Errorf("status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var payload interface{}
	if err := json.Unmarshal(body, &payload); err != nil {
		return "", "", err
	}

	status = parseStatusFromInterface(payload)
	taskID = extractTaskIDFromPayload(payload)
	return taskID, status, nil
}

func parseStatusFromBody(body []byte) string {
	trimmed := bytes.TrimSpace(body)
	if len(trimmed) == 0 {
		return ""
	}

	var payload interface{}
	if err := json.Unmarshal(trimmed, &payload); err == nil {
		if status := parseStatusFromInterface(payload); status != "" {
			return status
		}
	}

	raw := strings.Trim(string(trimmed), "\"")
	return raw
}

func parseStatusFromInterface(v interface{}) string {
	switch val := v.(type) {
	case string:
		return val
	case map[string]interface{}:
		return parseStatusFromMap(val)
	case []interface{}:
		for _, item := range val {
			if status := parseStatusFromInterface(item); status != "" {
				return status
			}
		}
	}
	return ""
}

func parseStatusFromMap(m map[string]interface{}) string {
	for _, key := range []string{"status", "branch_status", "branchStatus"} {
		if value, ok := m[key]; ok {
			if status := parseStatusFromInterface(value); status != "" {
				return status
			}
		}
	}

	for _, value := range m {
		if status := parseStatusFromInterface(value); status != "" {
			return status
		}
	}
	return ""
}

func extractTaskIDFromPayload(payload interface{}) string {
	switch val := payload.(type) {
	case map[string]interface{}:
		if data, ok := val["data"]; ok {
			if id := extractTaskIDFromPayload(data); id != "" {
				return id
			}
		}
		if id, ok := val["task_id"].(string); ok && id != "" {
			return id
		}
		if task, ok := val["task"].(map[string]interface{}); ok {
			if id, ok := task["id"].(string); ok && id != "" {
				return id
			}
		}
	case []interface{}:
		for _, item := range val {
			if id := extractTaskIDFromPayload(item); id != "" {
				return id
			}
		}
	}
	return ""
}

func normalizeStatusString(status string) string {
	status = strings.TrimSpace(status)
	if status == "" {
		return ""
	}
	status = strings.Trim(status, "\"")
	if status == "" {
		return ""
	}
	status = strings.ReplaceAll(status, "-", "_")
	status = strings.ReplaceAll(status, " ", "_")
	return strings.ToUpper(status)
}

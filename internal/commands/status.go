package commands

import (
	"encoding/json"
	"fmt"
	"net/http"
)

type StatusCommand struct{}

func NewStatusCommand() Command {
	return &StatusCommand{}
}

func (c *StatusCommand) Name() string {
	return "status"
}

func (c *StatusCommand) Usage() string {
	return "vkcli status <attempt_id>"
}

func (c *StatusCommand) Description() string {
	return "実行状態確認"
}

func (c *StatusCommand) Run(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("Usage: vkcli status <attempt_id>")
	}
	attemptID := args[0]
	status := getAttemptStatus(attemptID)
	fmt.Printf("Attempt %s status: %s\n", attemptID, status)
	return nil
}

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
	if status == "" {
		status = "UNKNOWN"
	}
	return status
}

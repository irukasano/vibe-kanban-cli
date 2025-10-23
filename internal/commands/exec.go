package commands

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type ExecCommand struct{}

func NewExecCommand() Command {
	return &ExecCommand{}
}

func (c *ExecCommand) Name() string {
	return "exec"
}

func (c *ExecCommand) Usage() string {
	return "vkcli exec <task_id>"
}

func (c *ExecCommand) Description() string {
	return "タスクを開始して監視"
}

func (c *ExecCommand) Run(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("Usage: vkcli exec <task_id>")
	}
	taskID := args[0]

	resp, err := http.Post(fmt.Sprintf("%s/tasks/%s/attempts", baseURL, taskID),
		"application/json", bytes.NewReader([]byte("{}")))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	var attempt map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&attempt); err != nil {
		return err
	}
	attemptID, _ := attempt["id"].(string)
	if attemptID == "" {
		return fmt.Errorf("attempt id not found in response")
	}
	fmt.Printf("Started attempt: %s\n", attemptID)

	for {
		time.Sleep(3 * time.Second)
		status := getAttemptStatus(attemptID)
		fmt.Printf("Status: %s\r", status)
		if status == "DONE" || status == "ERROR" {
			fmt.Println()
			break
		}
	}
	return nil
}

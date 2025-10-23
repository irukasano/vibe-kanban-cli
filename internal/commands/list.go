package commands

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

type ListCommand struct{}

func NewListCommand() Command {
	return &ListCommand{}
}

func (c *ListCommand) Name() string {
	return "list"
}

func (c *ListCommand) Usage() string {
	return "vkcli list <project_id>"
}

func (c *ListCommand) Description() string {
	return "タスク一覧"
}

func (c *ListCommand) Run(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("Usage: vkcli list <project_id>")
	}
	projectID := args[0]
	url := fmt.Sprintf("%s/tasks?project_id=%s", baseURL, projectID)
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	var wrapper struct {
		Success   bool                     `json:"success"`
		Data      []map[string]interface{} `json:"data"`
		ErrorData interface{}              `json:"error_data"`
		Message   interface{}              `json:"message"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&wrapper); err != nil {
		return err
	}

	tasks := wrapper.Data
	if len(tasks) == 0 {
		fmt.Println("No tasks found for this project.")
		return nil
	}

	fmt.Printf("%-38s  %-40s  %-10s\n", "TASK ID", "TITLE", "STATUS")
	fmt.Println(strings.Repeat("-", 92))
	for _, t := range tasks {
		fmt.Printf("%-38s  %-40s  %-10s\n",
			t["id"], t["title"], t["status"])
	}
	return nil
}

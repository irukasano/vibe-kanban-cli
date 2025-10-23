package commands

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

type ProjectsCommand struct{}

func NewProjectsCommand() Command {
	return &ProjectsCommand{}
}

func (c *ProjectsCommand) Name() string {
	return "projects"
}

func (c *ProjectsCommand) Usage() string {
	return "vkcli projects"
}

func (c *ProjectsCommand) Description() string {
	return "プロジェクト一覧"
}

func (c *ProjectsCommand) Run(args []string) error {
	resp, err := http.Get(baseURL + "/projects")
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

	projects := wrapper.Data
	if len(projects) == 0 {
		fmt.Println("No projects found.")
		return nil
	}

	fmt.Printf("%-38s  %-40s\n", "PROJECT ID", "NAME")
	fmt.Println(strings.Repeat("-", 80))
	for _, p := range projects {
		fmt.Printf("%-38s  %-40s\n", p["id"], p["name"])
	}
	return nil
}

package commands

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"strings"
)

type PickCommand struct{}

func NewPickCommand() Command {
	return &PickCommand{}
}

func (c *PickCommand) Name() string {
	return "pick"
}

func (c *PickCommand) Usage() string {
	return "vkcli pick [--with-messages]"
}

func (c *PickCommand) Description() string {
	return "fzfでプロジェクトとタスクを選択してタスク詳細を表示"
}

func (c *PickCommand) Run(args []string) error {
	if _, err := exec.LookPath("fzf"); err != nil {
		return errors.New("fzf が見つかりませんでした。インストールしてください")
	}

	withMessages := len(args) > 0 && args[0] == "--with-messages"

	projects, err := fetchProjects()
	if err != nil {
		return err
	}
	if len(projects) == 0 {
		fmt.Println("No projects found.")
		return nil
	}

	projectLines := make([]string, len(projects))
	for i, p := range projects {
		projectLines[i] = fmt.Sprintf("%s\t%s", p.ID, p.Name)
	}

	projectSelection, key, cancelled, err := runFzf("Project> ", projectLines)
	if err != nil {
		return err
	}
	if cancelled {
		fmt.Println("Selection canceled.")
		return nil
	}
	if key != "" {
		projectSelection = ""
	}

	projectID := strings.SplitN(projectSelection, "\t", 2)[0]
	currentProjectIndex := findProjectIndex(projects, projectID)

	execPath, err := os.Executable()
	if err != nil {
		execPath = "vkcli"
	}

	previewCmd := fmt.Sprintf("%s show {1} --with-messages", shellQuote(execPath))

	for {
		tasks, err := fetchTasks(projectID)
		if err != nil {
			return err
		}
		if len(tasks) == 0 {
			fmt.Println("No tasks found for this project.")
			return nil
		}

		taskLines := make([]string, len(tasks))
		for i, t := range tasks {
			taskLines[i] = fmt.Sprintf("%s\t[%s] %s", t.ID, t.Status, t.Title)
		}

		taskSelection, taskKey, cancelled, err := runFzf(
			"Task> ",
			taskLines,
			"--delimiter", "\t",
			"--with-nth", "2",
			"--preview-window", "right:60%:wrap",
			"--preview", previewCmd,
			"--expect", "ctrl-p",
			"--header", formatProjectHeader(projects, currentProjectIndex),
		)
		if err != nil {
			return err
		}
		if cancelled {
			fmt.Println("Selection canceled.")
			return nil
		}

		if taskKey == "ctrl-p" {
			newSelection, key, cancelled, err := runFzf(
				"Project> ",
				projectLines,
			)
			if err != nil {
				return err
			}
			if cancelled {
				fmt.Println("Selection canceled.")
				return nil
			}
			if key != "" {
				continue
			}
			projectID = strings.SplitN(newSelection, "\t", 2)[0]
			currentProjectIndex = findProjectIndex(projects, projectID)
			continue
		}

		taskID := strings.SplitN(taskSelection, "\t", 2)[0]

		showArgs := []string{taskID}
		if withMessages {
			showArgs = append(showArgs, "--with-messages")
		}
		return NewShowCommand().Run(showArgs)
	}
}

type project struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type task struct {
	ID     string `json:"id"`
	Title  string `json:"title"`
	Status string `json:"status"`
}

func fetchProjects() ([]project, error) {
	resp, err := http.Get(baseURL + "/projects")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var wrapper struct {
		Success bool      `json:"success"`
		Data    []project `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&wrapper); err != nil {
		return nil, err
	}
	return wrapper.Data, nil
}

func fetchTasks(projectID string) ([]task, error) {
	values := url.Values{}
	values.Set("project_id", projectID)
	resp, err := http.Get(fmt.Sprintf("%s/tasks?%s", baseURL, values.Encode()))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var wrapper struct {
		Success bool   `json:"success"`
		Data    []task `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&wrapper); err != nil {
		return nil, err
	}
	return wrapper.Data, nil
}

func runFzf(prompt string, lines []string, extraArgs ...string) (string, string, bool, error) {
	args := []string{"--prompt", prompt, "--no-multi"}
	expectUsed := false
	if len(extraArgs) > 0 {
		args = append(args, extraArgs...)
		expectUsed = hasExpect(extraArgs)
	}
	cmd := exec.Command("fzf", args...)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return "", "", false, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return "", "", false, err
	}
	cmd.Stderr = os.Stderr

	go func() {
		for _, line := range lines {
			fmt.Fprintln(stdin, line)
		}
		stdin.Close()
	}()

	if err := cmd.Start(); err != nil {
		return "", "", false, err
	}

	selected, err := io.ReadAll(stdout)
	if err != nil {
		return "", "", false, err
	}

	waitErr := cmd.Wait()
	if waitErr != nil {
		if exitErr, ok := waitErr.(*exec.ExitError); ok {
			exitCode := exitErr.ExitCode()
			if exitCode == 130 || exitCode == 1 {
				return "", "", true, nil
			}
		}
		return "", "", false, waitErr
	}

	selection, key := parseFzfOutput(selected, expectUsed)
	if selection == "" && key == "" {
		return "", "", true, nil
	}

	return selection, key, false, nil
}

func shellQuote(value string) string {
	if value == "" {
		return "''"
	}
	return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
}

func hasExpect(args []string) bool {
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--expect" {
			return true
		}
		if strings.HasPrefix(arg, "--expect=") {
			return true
		}
	}
	return false
}

func parseFzfOutput(raw []byte, expectUsed bool) (selection string, key string) {
	clean := strings.ReplaceAll(string(raw), "\r\n", "\n")
	lines := strings.Split(clean, "\n")

	trimLines := func(ls []string) []string {
		filtered := make([]string, 0, len(ls))
		for _, l := range ls {
			l = strings.TrimSpace(l)
			if l != "" {
				filtered = append(filtered, l)
			}
		}
		return filtered
	}

	if expectUsed {
		if len(lines) == 0 {
			return "", ""
		}
		key = strings.TrimSpace(lines[0])
		rest := trimLines(lines[1:])
		if len(rest) > 0 {
			selection = rest[0]
		}
		return selection, key
	}

	filtered := trimLines(lines)
	if len(filtered) > 0 {
		selection = filtered[0]
	}
	return selection, ""
}

func formatProjectHeader(projects []project, currentIndex int) string {
	var b strings.Builder
	b.WriteString("Projects (Ctrl-P で再選択):\n")
	for i, p := range projects {
		marker := "  "
		if i == currentIndex {
			marker = "▶ "
		}
		b.WriteString(fmt.Sprintf("%s%s (%s)\n", marker, p.Name, p.ID))
	}
	return strings.TrimRight(b.String(), "\n")
}

func findProjectIndex(projects []project, id string) int {
	for i, p := range projects {
		if p.ID == id {
			return i
		}
	}
	return 0
}

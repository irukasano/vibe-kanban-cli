package main

import (
	"fmt"
	"os"

	"vkcli/internal/commands"
)

func main() {
	registerCommands()

	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	cmdName := os.Args[1]
	cmd, ok := commands.Lookup(cmdName)
	if !ok {
		fmt.Println("Unknown command:", cmdName)
		printUsage()
		os.Exit(1)
	}

	if err := cmd.Run(os.Args[2:]); err != nil {
		fmt.Println("Error:", err)
		os.Exit(1)
	}
}

func registerCommands() {
	commands.Register(commands.NewProjectsCommand())
	commands.Register(commands.NewListCommand())
	commands.Register(commands.NewShowCommand())
	commands.Register(commands.NewExecCommand())
	commands.Register(commands.NewStatusCommand())
}

func printUsage() {
	fmt.Println("Usage:")
	for _, cmd := range commands.All() {
		fmt.Printf("  %-36s # %s\n", cmd.Usage(), cmd.Description())
	}
}

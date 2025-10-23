package commands

// Command represents a single CLI subcommand.
type Command interface {
	// Name is the subcommand identifier such as "projects" or "list".
	Name() string
	// Usage returns the usage string, e.g. "vkcli projects".
	Usage() string
	// Description provides a short description for help output.
	Description() string
	// Run executes the command.
	Run(args []string) error
}

var (
	registry      = make(map[string]Command)
	registeredSeq []Command
)

// Register adds the command to the registry.
func Register(cmd Command) {
	name := cmd.Name()
	registry[name] = cmd
	registeredSeq = append(registeredSeq, cmd)
}

// Lookup returns the command for the given name.
func Lookup(name string) (Command, bool) {
	cmd, ok := registry[name]
	return cmd, ok
}

// All returns the registered commands in registration order.
func All() []Command {
	return registeredSeq
}

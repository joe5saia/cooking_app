package app

import (
	"fmt"
	"io"
	"sort"
	"strings"
)

// runCompletion prints shell completion scripts.
func (a *App) runCompletion(args []string) int {
	if hasHelpFlag(args) {
		printCompletionUsage(a.stdout)
		return exitOK
	}

	flags := newFlagSet("completion", a.stderr, printCompletionUsage)
	if err := flags.Parse(args); err != nil {
		return exitUsage
	}
	if flags.NArg() != 1 {
		printCompletionUsage(a.stderr)
		return exitUsage
	}

	shell := strings.ToLower(strings.TrimSpace(flags.Arg(0)))
	script, ok := completionScript(shell)
	if !ok {
		usageErrorf(a.stderr, "unsupported shell: %s", shell)
		printCompletionUsage(a.stderr)
		return exitUsage
	}

	if _, err := io.WriteString(a.stdout, script); err != nil {
		writeLine(a.stderr, err)
		return exitError
	}
	return exitOK
}

// completionScript returns the completion script for a supported shell.
func completionScript(shell string) (string, bool) {
	data := completionDataFromRegistry()
	switch shell {
	case "bash":
		return buildBashCompletion(data), true
	case "zsh":
		return buildZshCompletion(data), true
	case "fish":
		return buildFishCompletion(data), true
	default:
		return "", false
	}
}

type completionData struct {
	Commands []*command
}

func completionDataFromRegistry() completionData {
	return completionData{Commands: commandRegistry()}
}

func buildBashCompletion(data completionData) string {
	commandList := strings.Join(commandNames(data.Commands), " ")
	globalFlagList := strings.Join(globalFlagNames(), " ")
	flagsByCommand := commandFlagMap(data.Commands)

	var b strings.Builder
	b.WriteString("# bash completion for cookctl\n")
	b.WriteString("_cookctl() {\n")
	b.WriteString("  local cur first second flags\n")
	b.WriteString("  COMPREPLY=()\n")
	b.WriteString("  cur=\"${COMP_WORDS[COMP_CWORD]}\"\n")
	b.WriteString("  first=\"${COMP_WORDS[1]}\"\n")
	b.WriteString("  second=\"${COMP_WORDS[2]}\"\n\n")
	fmt.Fprintf(&b, "  if [[ $COMP_CWORD -eq 1 ]]; then\n    COMPREPLY=( $(compgen -W \"%s\" -- \"$cur\") )\n    return 0\n  fi\n\n", commandList)
	b.WriteString("  if [[ \"$cur\" == -* ]]; then\n")
	fmt.Fprintf(&b, "    flags=\"%s\"\n", globalFlagList)
	b.WriteString("    case \"$first\" in\n")
	b.WriteString("      completion)\n")
	b.WriteString("        flags=\"\"\n")
	b.WriteString("        ;;\n")

	commandNames := make([]string, 0, len(flagsByCommand))
	for name := range flagsByCommand {
		commandNames = append(commandNames, name)
	}
	sort.Strings(commandNames)
	for _, name := range commandNames {
		subFlags := flagsByCommand[name]
		if len(subFlags) == 0 {
			continue
		}
		fmt.Fprintf(&b, "      %s)\n", name)
		b.WriteString("        case \"$second\" in\n")
		subNames := make([]string, 0, len(subFlags))
		for sub := range subFlags {
			subNames = append(subNames, sub)
		}
		sort.Strings(subNames)
		for _, sub := range subNames {
			flags := strings.Join(subFlags[sub], " ")
			if flags == "" {
				continue
			}
			fmt.Fprintf(&b, "          %s)\n", sub)
			fmt.Fprintf(&b, "            flags=\"$flags %s\"\n", flags)
			b.WriteString("            ;;\n")
		}
		b.WriteString("        esac\n")
		b.WriteString("        ;;\n")
	}
	b.WriteString("    esac\n\n")
	b.WriteString("    COMPREPLY=( $(compgen -W \"$flags\" -- \"$cur\") )\n")
	b.WriteString("    return 0\n")
	b.WriteString("  fi\n")
	b.WriteString("}\n\n")
	b.WriteString("complete -F _cookctl cookctl\n")
	return b.String()
}

func buildZshCompletion(data completionData) string {
	var b strings.Builder
	b.WriteString("#compdef cookctl\n\n")
	b.WriteString("_cookctl() {\n")
	commands := data.Commands
	commandList := commandNames(commands)
	helpTopics := strings.Join(commandList, " ")

	subcommandVars := []string{"commands", "help_topics"}
	for _, cmd := range commands {
		if len(cmd.Subcommands) == 0 {
			continue
		}
		varName := zshVarName(cmd.Name)
		subcommandVars = append(subcommandVars, varName)
	}
	fmt.Fprintf(&b, "  local -a %s\n", strings.Join(subcommandVars, " "))

	b.WriteString("  commands=(\n")
	for _, cmd := range commands {
		synopsis := strings.TrimSpace(cmd.Synopsis)
		if synopsis == "" {
			synopsis = cmd.Name
		}
		fmt.Fprintf(&b, "    '%s:%s'\n", cmd.Name, synopsis)
	}
	b.WriteString("  )\n")
	for _, cmd := range commands {
		if len(cmd.Subcommands) == 0 {
			continue
		}
		varName := zshVarName(cmd.Name)
		fmt.Fprintf(&b, "  %s=(%s)\n", varName, strings.Join(commandNames(cmd.Subcommands), " "))
	}
	fmt.Fprintf(&b, "  help_topics=(%s)\n\n", helpTopics)

	b.WriteString("  _arguments -C \\\n    '1:command:->command' \\\n    '*::arg:->args'\n\n")
	b.WriteString("  case $state in\n")
	b.WriteString("    command)\n")
	b.WriteString("      _describe 'command' commands\n")
	b.WriteString("      ;;\n")
	b.WriteString("    args)\n")
	b.WriteString("      case $words[2] in\n")
	b.WriteString("        completion)\n")
	b.WriteString("          _values 'shell' bash zsh fish\n")
	b.WriteString("          ;;\n")
	b.WriteString("        help)\n")
	b.WriteString("          _values 'help topic' $help_topics\n")
	b.WriteString("          ;;\n")
	for _, cmd := range commands {
		if len(cmd.Subcommands) == 0 {
			continue
		}
		varName := zshVarName(cmd.Name)
		fmt.Fprintf(&b, "        %s)\n", cmd.Name)
		fmt.Fprintf(&b, "          _values '%s command' $%s\n", cmd.Name, varName)
		b.WriteString("          ;;\n")
	}
	b.WriteString("      esac\n")
	b.WriteString("      ;;\n")
	b.WriteString("  esac\n")
	b.WriteString("}\n\n")
	b.WriteString("compdef _cookctl cookctl\n")
	return b.String()
}

func buildFishCompletion(data completionData) string {
	var b strings.Builder
	b.WriteString("# fish completion for cookctl\n")

	for _, spec := range globalFlagSpecs() {
		flagLine := fishFlagLine(spec)
		if flagLine != "" {
			b.WriteString(flagLine)
			b.WriteString("\n")
		}
	}
	b.WriteString("\n")

	commandList := strings.Join(commandNames(data.Commands), " ")
	fmt.Fprintf(&b, "complete -c cookctl -f -n '__fish_use_subcommand' -a '%s'\n", commandList)
	b.WriteString("complete -c cookctl -f -n '__fish_seen_subcommand_from completion' -a 'bash zsh fish'\n")
	fmt.Fprintf(&b, "complete -c cookctl -f -n '__fish_seen_subcommand_from help' -a '%s'\n", commandList)
	for _, cmd := range data.Commands {
		if len(cmd.Subcommands) == 0 {
			continue
		}
		fmt.Fprintf(&b, "complete -c cookctl -f -n '__fish_seen_subcommand_from %s' -a '%s'\n", cmd.Name, strings.Join(commandNames(cmd.Subcommands), " "))
	}
	return b.String()
}

func commandFlagMap(commands []*command) map[string]map[string][]string {
	result := make(map[string]map[string][]string)
	for _, cmd := range commands {
		if len(cmd.Subcommands) == 0 {
			continue
		}
		subFlags := make(map[string][]string)
		for _, sub := range cmd.Subcommands {
			flags := flagNames(sub.FlagSet)
			if len(sub.Subcommands) > 0 {
				flags = unionFlags(flags, flagsFromSubcommands(sub.Subcommands))
			}
			if len(flags) > 0 {
				subFlags[sub.Name] = flags
			}
		}
		if len(subFlags) > 0 {
			result[cmd.Name] = subFlags
		}
	}
	return result
}

func flagsFromSubcommands(subcommands []*command) []string {
	groups := make([][]string, 0, len(subcommands)*2)
	for _, sub := range subcommands {
		if len(sub.Subcommands) > 0 {
			groups = append(groups, flagsFromSubcommands(sub.Subcommands))
		}
		groups = append(groups, flagNames(sub.FlagSet))
	}
	return unionFlags(groups...)
}

func unionFlags(groups ...[]string) []string {
	set := make(map[string]struct{})
	for _, group := range groups {
		for _, flag := range group {
			set[flag] = struct{}{}
		}
	}
	if len(set) == 0 {
		return nil
	}
	names := make([]string, 0, len(set))
	for name := range set {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func globalFlagNames() []string {
	names := make([]string, 0, len(globalFlags))
	for name := range globalFlags {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func globalFlagSpecs() []globalFlagSpec {
	specs := make([]globalFlagSpec, 0, len(globalFlags))
	for _, spec := range globalFlags {
		specs = append(specs, spec)
	}
	sort.Slice(specs, func(i, j int) bool {
		return specs[i].name < specs[j].name
	})
	return specs
}

func fishFlagLine(spec globalFlagSpec) string {
	name := strings.TrimSpace(spec.name)
	if name == "" {
		return ""
	}
	var builder strings.Builder
	builder.WriteString("complete -c cookctl -f")
	switch {
	case strings.HasPrefix(name, "--"):
		fmt.Fprintf(&builder, " -l %s", strings.TrimPrefix(name, "--"))
	case strings.HasPrefix(name, "-") && len(name) == 2:
		fmt.Fprintf(&builder, " -s %s", strings.TrimPrefix(name, "-"))
	default:
		return ""
	}
	if strings.TrimSpace(spec.description) != "" {
		fmt.Fprintf(&builder, " -d '%s'", spec.description)
	}
	return builder.String()
}

func zshVarName(name string) string {
	name = strings.ReplaceAll(name, "-", "_")
	return name + "_cmds"
}

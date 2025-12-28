package app

import (
	"flag"
	"strings"
	"time"

	"github.com/saiaj/cooking_app/backend/internal/cookctl/config"
)

func (a *App) runConfig(args []string) int {
	if len(args) > 0 && isHelpFlag(args[0]) {
		printConfigUsage(a.stdout)
		return exitOK
	}
	if len(args) == 0 {
		printConfigUsage(a.stderr)
		return exitUsage
	}

	switch args[0] {
	case "view":
		return a.runConfigView(args[1:])
	case "set":
		return a.runConfigSet(args[1:])
	case "unset":
		return a.runConfigUnset(args[1:])
	case "path":
		return a.runConfigPath(args[1:])
	default:
		writef(a.stderr, "unknown config command: %s\n", args[0])
		printConfigUsage(a.stderr)
		return exitUsage
	}
}

func (a *App) runConfigView(args []string) int {
	if hasHelpFlag(args) {
		printConfigViewUsage(a.stdout)
		return exitOK
	}

	flags := flag.NewFlagSet("config view", flag.ContinueOnError)
	flags.SetOutput(a.stderr)

	var configPath string
	flags.StringVar(&configPath, "config", "", "Config file path")

	if err := flags.Parse(args); err != nil {
		return exitUsage
	}

	configPath = strings.TrimSpace(configPath)
	if configPath == "" {
		path, err := config.DefaultConfigPath()
		if err != nil {
			writeLine(a.stderr, err)
			return exitError
		}
		configPath = path
	}

	cfg, err := config.Load(configPath)
	if err != nil {
		writeLine(a.stderr, err)
		return exitUsage
	}

	view := configView{
		ConfigPath: configPath,
		APIURL:     cfg.APIURL,
		Output:     string(cfg.Output),
		Timeout:    cfg.Timeout.String(),
		Debug:      cfg.Debug,
	}
	return writeOutput(a.stdout, a.cfg.Output, view)
}

func (a *App) runConfigSet(args []string) int {
	if hasHelpFlag(args) {
		printConfigSetUsage(a.stdout)
		return exitOK
	}

	flags := flag.NewFlagSet("config set", flag.ContinueOnError)
	flags.SetOutput(a.stderr)

	var configPath string
	var apiURL optionalString
	var output optionalString
	var timeoutStr optionalString
	var debug optionalBool

	flags.StringVar(&configPath, "config", "", "Config file path")
	flags.Var(&apiURL, "api-url", "API base URL")
	flags.Var(&output, "output", "Output format: table|json")
	flags.Var(&timeoutStr, "timeout", "Request timeout (e.g. 30s)")
	flags.Var(&debug, "debug", "Enable debug logging")

	if err := flags.Parse(args); err != nil {
		return exitUsage
	}

	configPath = strings.TrimSpace(configPath)
	if configPath == "" {
		path, err := config.DefaultConfigPath()
		if err != nil {
			writeLine(a.stderr, err)
			return exitError
		}
		configPath = path
	}

	cfg, err := config.LoadFile(configPath)
	if err != nil {
		writeLine(a.stderr, err)
		return exitUsage
	}

	if apiURL.set {
		cfg.APIURL = apiURL.value
	}
	if output.set {
		parsed, err := config.ParseOutput(output.value)
		if err != nil {
			writeLine(a.stderr, err)
			return exitUsage
		}
		cfg.Output = parsed
	}
	if timeoutStr.set {
		timeout, err := time.ParseDuration(timeoutStr.value)
		if err != nil {
			writeLine(a.stderr, "timeout must be a duration (e.g. 30s)")
			return exitUsage
		}
		cfg.Timeout = timeout
	}
	if debug.set {
		cfg.Debug = debug.value
	}

	if err := config.Save(configPath, cfg); err != nil {
		writeLine(a.stderr, err)
		return exitError
	}

	view := configView{
		ConfigPath: configPath,
		APIURL:     cfg.APIURL,
		Output:     string(cfg.Output),
		Timeout:    cfg.Timeout.String(),
		Debug:      cfg.Debug,
	}
	return writeOutput(a.stdout, a.cfg.Output, view)
}

func (a *App) runConfigUnset(args []string) int {
	if hasHelpFlag(args) {
		printConfigUnsetUsage(a.stdout)
		return exitOK
	}

	flags := flag.NewFlagSet("config unset", flag.ContinueOnError)
	flags.SetOutput(a.stderr)

	var configPath string
	var apiURL bool
	var output bool
	var timeout bool
	var debug bool

	flags.StringVar(&configPath, "config", "", "Config file path")
	flags.BoolVar(&apiURL, "api-url", false, "Clear api_url")
	flags.BoolVar(&output, "output", false, "Clear output")
	flags.BoolVar(&timeout, "timeout", false, "Clear timeout")
	flags.BoolVar(&debug, "debug", false, "Clear debug")

	if err := flags.Parse(args); err != nil {
		return exitUsage
	}
	if !apiURL && !output && !timeout && !debug {
		writeLine(a.stderr, "at least one flag is required")
		return exitUsage
	}

	configPath = strings.TrimSpace(configPath)
	if configPath == "" {
		path, err := config.DefaultConfigPath()
		if err != nil {
			writeLine(a.stderr, err)
			return exitError
		}
		configPath = path
	}

	cfg, err := config.LoadFile(configPath)
	if err != nil {
		writeLine(a.stderr, err)
		return exitUsage
	}
	defaults := config.Default()

	if apiURL {
		cfg.APIURL = defaults.APIURL
	}
	if output {
		cfg.Output = defaults.Output
	}
	if timeout {
		cfg.Timeout = defaults.Timeout
	}
	if debug {
		cfg.Debug = defaults.Debug
	}

	if err := config.Save(configPath, cfg); err != nil {
		writeLine(a.stderr, err)
		return exitError
	}

	view := configView{
		ConfigPath: configPath,
		APIURL:     cfg.APIURL,
		Output:     string(cfg.Output),
		Timeout:    cfg.Timeout.String(),
		Debug:      cfg.Debug,
	}
	return writeOutput(a.stdout, a.cfg.Output, view)
}

func (a *App) runConfigPath(args []string) int {
	if hasHelpFlag(args) {
		printConfigPathUsage(a.stdout)
		return exitOK
	}

	flags := flag.NewFlagSet("config path", flag.ContinueOnError)
	flags.SetOutput(a.stderr)

	if err := flags.Parse(args); err != nil {
		return exitUsage
	}

	path, err := config.DefaultConfigPath()
	if err != nil {
		writeLine(a.stderr, err)
		return exitError
	}

	view := configView{
		ConfigPath: path,
		APIURL:     a.cfg.APIURL,
		Output:     string(a.cfg.Output),
		Timeout:    a.cfg.Timeout.String(),
		Debug:      a.cfg.Debug,
	}
	return writeOutput(a.stdout, a.cfg.Output, view)
}

package app

import (
	"flag"
	"io"
	"strings"
	"time"

	"github.com/saiaj/cooking_app/backend/internal/cookctl/config"
)

type configViewFlags struct {
	configPath string
}

type configSetFlags struct {
	configPath string
	apiURL     optionalString
	output     optionalString
	timeoutStr optionalString
	debug      optionalBool
}

type configUnsetFlags struct {
	configPath string
	apiURL     bool
	output     bool
	timeout    bool
	debug      bool
}

func configViewFlagSet(out io.Writer) (*flag.FlagSet, *configViewFlags) {
	opts := &configViewFlags{}
	flags := newFlagSet("config view", out, printConfigViewUsage)
	flags.StringVar(&opts.configPath, "config", "", "Config file path")
	return flags, opts
}

func configSetFlagSet(out io.Writer) (*flag.FlagSet, *configSetFlags) {
	opts := &configSetFlags{}
	flags := newFlagSet("config set", out, printConfigSetUsage)
	flags.StringVar(&opts.configPath, "config", "", "Config file path")
	flags.Var(&opts.apiURL, "api-url", "API base URL")
	flags.Var(&opts.output, "output", "Output format: table|json")
	flags.Var(&opts.timeoutStr, "timeout", "Request timeout (e.g. 30s)")
	flags.Var(&opts.debug, "debug", "Enable debug logging")
	return flags, opts
}

func configUnsetFlagSet(out io.Writer) (*flag.FlagSet, *configUnsetFlags) {
	opts := &configUnsetFlags{}
	flags := newFlagSet("config unset", out, printConfigUnsetUsage)
	flags.StringVar(&opts.configPath, "config", "", "Config file path")
	flags.BoolVar(&opts.apiURL, "api-url", false, "Clear api_url")
	flags.BoolVar(&opts.output, "output", false, "Clear output")
	flags.BoolVar(&opts.timeout, "timeout", false, "Clear timeout")
	flags.BoolVar(&opts.debug, "debug", false, "Clear debug")
	return flags, opts
}

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
		usageErrorf(a.stderr, "unknown config command: %s", args[0])
		printConfigUsage(a.stderr)
		return exitUsage
	}
}

func (a *App) runConfigView(args []string) int {
	if hasHelpFlag(args) {
		printConfigViewUsage(a.stdout)
		return exitOK
	}

	flags, opts := configViewFlagSet(a.stderr)
	if err := flags.Parse(args); err != nil {
		return exitUsage
	}

	opts.configPath = strings.TrimSpace(opts.configPath)
	if opts.configPath == "" {
		path, err := config.DefaultConfigPath()
		if err != nil {
			writeLine(a.stderr, err)
			return exitError
		}
		opts.configPath = path
	}

	cfg, err := config.Load(opts.configPath)
	if err != nil {
		return usageError(a.stderr, err.Error())
	}

	view := configView{
		ConfigPath: opts.configPath,
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

	flags, opts := configSetFlagSet(a.stderr)
	if err := flags.Parse(args); err != nil {
		return exitUsage
	}

	opts.configPath = strings.TrimSpace(opts.configPath)
	if opts.configPath == "" {
		path, err := config.DefaultConfigPath()
		if err != nil {
			writeLine(a.stderr, err)
			return exitError
		}
		opts.configPath = path
	}

	cfg, err := config.LoadFile(opts.configPath)
	if err != nil {
		return usageError(a.stderr, err.Error())
	}

	if opts.apiURL.set {
		cfg.APIURL = opts.apiURL.value
	}
	if opts.output.set {
		parsed, err := config.ParseOutput(opts.output.value)
		if err != nil {
			return usageError(a.stderr, err.Error())
		}
		cfg.Output = parsed
	}
	if opts.timeoutStr.set {
		timeout, err := time.ParseDuration(opts.timeoutStr.value)
		if err != nil {
			return usageError(a.stderr, "timeout must be a duration (e.g. 30s)")
		}
		cfg.Timeout = timeout
	}
	if opts.debug.set {
		cfg.Debug = opts.debug.value
	}

	if err := config.Save(opts.configPath, cfg); err != nil {
		writeLine(a.stderr, err)
		return exitError
	}

	view := configView{
		ConfigPath: opts.configPath,
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

	flags, opts := configUnsetFlagSet(a.stderr)
	if err := flags.Parse(args); err != nil {
		return exitUsage
	}
	if !opts.apiURL && !opts.output && !opts.timeout && !opts.debug {
		return usageError(a.stderr, "at least one flag is required")
	}

	opts.configPath = strings.TrimSpace(opts.configPath)
	if opts.configPath == "" {
		path, err := config.DefaultConfigPath()
		if err != nil {
			writeLine(a.stderr, err)
			return exitError
		}
		opts.configPath = path
	}

	cfg, err := config.LoadFile(opts.configPath)
	if err != nil {
		return usageError(a.stderr, err.Error())
	}
	defaults := config.Default()

	if opts.apiURL {
		cfg.APIURL = defaults.APIURL
	}
	if opts.output {
		cfg.Output = defaults.Output
	}
	if opts.timeout {
		cfg.Timeout = defaults.Timeout
	}
	if opts.debug {
		cfg.Debug = defaults.Debug
	}

	if err := config.Save(opts.configPath, cfg); err != nil {
		writeLine(a.stderr, err)
		return exitError
	}

	view := configView{
		ConfigPath: opts.configPath,
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

	flags := newFlagSet("config path", a.stderr, printConfigPathUsage)

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

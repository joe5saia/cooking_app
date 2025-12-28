package app

import (
	"flag"
	"io"
	"strings"
)

// runCompletion prints shell completion scripts.
func (a *App) runCompletion(args []string) int {
	if hasHelpFlag(args) {
		printCompletionUsage(a.stdout)
		return exitOK
	}

	flags := flag.NewFlagSet("completion", flag.ContinueOnError)
	flags.SetOutput(a.stderr)
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
		writef(a.stderr, "unsupported shell: %s\n", shell)
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
	switch shell {
	case "bash":
		return bashCompletion, true
	case "zsh":
		return zshCompletion, true
	case "fish":
		return fishCompletion, true
	default:
		return "", false
	}
}

// bashCompletion provides bash completion for cookctl.
const bashCompletion = `# bash completion for cookctl
_cookctl() {
  local cur first second flags
  COMPREPLY=()
  cur="${COMP_WORDS[COMP_CWORD]}"
  first="${COMP_WORDS[1]}"
  second="${COMP_WORDS[2]}"

  if [[ $COMP_CWORD -eq 1 ]]; then
    COMPREPLY=( $(compgen -W "health version completion help auth token tag book user recipe meal-plan config" -- "$cur") )
    return 0
  fi

  if [[ "$cur" == -* ]]; then
    flags="--api-url --output --timeout --debug --skip-health-check --version --help -h"
    case "$first" in
      completion)
        flags=""
        ;;
      auth)
        case "$second" in
          login)
            flags="$flags --username --password-stdin --token-name --expires-at"
            ;;
          set)
            flags="$flags --token --token-stdin --api-url"
            ;;
          logout)
            flags="$flags --revoke"
            ;;
        esac
        ;;
      token)
        case "$second" in
          create)
            flags="$flags --name --expires-at"
            ;;
          revoke)
            flags="$flags --yes"
            ;;
        esac
        ;;
      tag)
        case "$second" in
          create|update)
            flags="$flags --name"
            ;;
          delete)
            flags="$flags --yes"
            ;;
        esac
        ;;
      book)
        case "$second" in
          create|update)
            flags="$flags --name"
            ;;
          delete)
            flags="$flags --yes"
            ;;
        esac
        ;;
      user)
        case "$second" in
          create)
            flags="$flags --username --password-stdin --display-name"
            ;;
          deactivate)
            flags="$flags --yes"
            ;;
        esac
        ;;
      recipe)
        case "$second" in
          list)
            flags="$flags --q --book --book-id --tag --tag-id --servings --include-deleted --limit --cursor --all --with-counts"
            ;;
          create)
            flags="$flags --file --stdin --interactive --allow-duplicate"
            ;;
          update)
            flags="$flags --file --stdin"
            ;;
          import)
            flags="$flags --file --stdin --allow-duplicate"
            ;;
          edit)
            flags="$flags --editor"
            ;;
          clone)
            flags="$flags --title --allow-duplicate"
            ;;
          tag)
            flags="$flags --replace --create-missing --no-create-missing"
            ;;
          delete|restore)
            flags="$flags --yes"
            ;;
        esac
        ;;
      meal-plan)
        case "$second" in
          list)
            flags="$flags --start --end"
            ;;
          create)
            flags="$flags --date --recipe-id"
            ;;
          delete)
            flags="$flags --date --recipe-id --yes"
            ;;
        esac
        ;;
      config)
        case "$second" in
          view)
            flags="$flags --config"
            ;;
          set)
            flags="$flags --config --api-url --output --timeout --debug"
            ;;
          unset)
            flags="$flags --config --api-url --output --timeout --debug"
            ;;
        esac
        ;;
    esac
    if [[ -n "$flags" ]]; then
      COMPREPLY=( $(compgen -W "$flags" -- "$cur") )
    fi
    return 0
  fi

  case "$first" in
    completion)
      COMPREPLY=( $(compgen -W "bash zsh fish" -- "$cur") )
      ;;
    help)
      COMPREPLY=( $(compgen -W "health version completion help auth token tag book user recipe meal-plan config" -- "$cur") )
      ;;
    auth)
      COMPREPLY=( $(compgen -W "login set status whoami logout" -- "$cur") )
      ;;
    token)
      COMPREPLY=( $(compgen -W "list create revoke" -- "$cur") )
      ;;
    tag)
      COMPREPLY=( $(compgen -W "list create update delete" -- "$cur") )
      ;;
    book)
      COMPREPLY=( $(compgen -W "list create update delete" -- "$cur") )
      ;;
    user)
      COMPREPLY=( $(compgen -W "list create deactivate" -- "$cur") )
      ;;
    recipe)
      COMPREPLY=( $(compgen -W "list get create update init template export import tag clone edit delete restore" -- "$cur") )
      ;;
    meal-plan)
      COMPREPLY=( $(compgen -W "list create delete" -- "$cur") )
      ;;
    config)
      COMPREPLY=( $(compgen -W "view set unset path" -- "$cur") )
      ;;
  esac
}

complete -F _cookctl cookctl
`

// zshCompletion provides zsh completion for cookctl.
const zshCompletion = `#compdef cookctl

_cookctl() {
  local -a commands auth_cmds token_cmds tag_cmds book_cmds user_cmds recipe_cmds meal_plan_cmds config_cmds
  commands=(
    'health:Check API health'
    'version:Show version'
    'completion:Generate shell completions'
    'help:Show command help'
    'auth:Authentication commands'
    'token:Token management'
    'tag:Tag management'
    'book:Recipe book management'
    'user:User management'
    'recipe:Recipe management'
    'meal-plan:Meal plan commands'
    'config:Config commands'
  )
  auth_cmds=(login set status whoami logout)
  token_cmds=(list create revoke)
  tag_cmds=(list create update delete)
  book_cmds=(list create update delete)
  user_cmds=(list create deactivate)
  recipe_cmds=(list get create update init template export import tag clone edit delete restore)
  meal_plan_cmds=(list create delete)
  config_cmds=(view set unset path)
  help_topics=(health version completion help auth token tag book user recipe meal-plan config)

  _arguments -C \
    '1:command:->command' \
    '*::arg:->args'

  case $state in
    command)
      _describe 'command' commands
      ;;
    args)
      case $words[2] in
        completion)
          _values 'shell' bash zsh fish
          ;;
        help)
          _values 'help topic' $help_topics
          ;;
        auth)
          _values 'auth command' $auth_cmds
          ;;
        token)
          _values 'token command' $token_cmds
          ;;
        tag)
          _values 'tag command' $tag_cmds
          ;;
        book)
          _values 'book command' $book_cmds
          ;;
        user)
          _values 'user command' $user_cmds
          ;;
        recipe)
          _values 'recipe command' $recipe_cmds
          ;;
        meal-plan)
          _values 'meal plan command' $meal_plan_cmds
          ;;
        config)
          _values 'config command' $config_cmds
          ;;
      esac
      ;;
  esac
}

compdef _cookctl cookctl
`

// fishCompletion provides fish completion for cookctl.
const fishCompletion = `# fish completion for cookctl
complete -c cookctl -f -l api-url -d 'API base URL'
complete -c cookctl -f -l output -d 'Output format (table|json)'
complete -c cookctl -f -l timeout -d 'Request timeout'
complete -c cookctl -f -l debug -d 'Enable debug logging'
complete -c cookctl -f -l skip-health-check -d 'Skip API health preflight'
complete -c cookctl -f -l version -d 'Show version and exit'
complete -c cookctl -f -s h -l help -d 'Show help and exit'

complete -c cookctl -f -n '__fish_use_subcommand' -a 'health version completion help auth token tag book user recipe meal-plan config'
complete -c cookctl -f -n '__fish_seen_subcommand_from completion' -a 'bash zsh fish'
complete -c cookctl -f -n '__fish_seen_subcommand_from help' -a 'health version completion help auth token tag book user recipe meal-plan config'
complete -c cookctl -f -n '__fish_seen_subcommand_from auth' -a 'login set status whoami logout'
complete -c cookctl -f -n '__fish_seen_subcommand_from token' -a 'list create revoke'
complete -c cookctl -f -n '__fish_seen_subcommand_from tag' -a 'list create update delete'
complete -c cookctl -f -n '__fish_seen_subcommand_from book' -a 'list create update delete'
complete -c cookctl -f -n '__fish_seen_subcommand_from user' -a 'list create deactivate'
complete -c cookctl -f -n '__fish_seen_subcommand_from recipe' -a 'list get create update init template export import tag clone edit delete restore'
complete -c cookctl -f -n '__fish_seen_subcommand_from meal-plan' -a 'list create delete'
complete -c cookctl -f -n '__fish_seen_subcommand_from config' -a 'view set unset path'
`

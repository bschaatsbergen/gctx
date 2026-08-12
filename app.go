package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
)

// version is set via -ldflags at release.
var version = "dev"

type app struct {
	stdout io.Writer
	stderr io.Writer
	getenv func(string) string
	// runGcloud is injectable for tests.
	runGcloud func(configName string, args ...string) error
}

func (a *app) run(args []string) int {
	if a.runGcloud == nil {
		a.runGcloud = execGcloud
	}
	if len(args) == 0 {
		return a.runGetContexts(nil)
	}
	cmd, rest := args[0], args[1:]
	switch cmd {
	case "config":
		return a.runConfig(rest)
	case "env":
		return a.runEnv(rest)
	case "login":
		return a.runLogin(rest)
	case "adc":
		return a.runADC(rest)
	case "init":
		return a.runInit(rest)
	case "completion":
		return a.runCompletion(rest)
	case "version", "--version":
		fmt.Fprintf(a.stdout, "gctx %s\n", version)
		return 0
	case "help", "--help", "-h":
		io.WriteString(a.stdout, usage)
		return 0
	case "-o", "--output":
		return a.runGetContexts(args)
	default:
		return a.unknownCommand(cmd)
	}
}

func (a *app) unknownCommand(cmd string) int {
	if strings.HasPrefix(cmd, "-") {
		return a.errf("unknown flag or command %q, run `gctx help` for usage", cmd)
	}
	s := a.store()
	if _, ok := s.lookup(cmd); ok {
		return a.errf("%q is a context, but per-shell switching needs the shell wrapper.\n"+
			"Add it to your shell config, e.g. bash/zsh:\n"+
			"  eval \"$(gctx init bash)\"\n"+
			"or switch globally instead: gctx config use-context %s", cmd, cmd)
	}
	names := contextNames(s.list())
	if len(names) > 0 {
		return a.errf("unknown command or context %q (available contexts: %s), run `gctx help` for usage", cmd, strings.Join(names, ", "))
	}
	return a.errf("unknown command %q, run `gctx help` for usage", cmd)
}

func (a *app) errf(format string, args ...any) int {
	fmt.Fprintf(a.stderr, "gctx: %s\n", fmt.Sprintf(format, args...))
	return 1
}

func (a *app) unknownContext(s *store, name string) int {
	names := contextNames(s.list())
	if len(names) == 0 {
		return a.errf("context %q not found: no contexts exist in %s (create one with `gctx login <name>`)", name, s.dir)
	}
	return a.errf("context %q not found (available: %s)", name, strings.Join(names, ", "))
}

func contextNames(cs []context) []string {
	names := make([]string, len(cs))
	for i, c := range cs {
		names[i] = c.name
	}
	return names
}

// execGcloud runs gcloud with CLOUDSDK_ACTIVE_CONFIG_NAME pinned to
// configName and stdio inherited, so browser flows work and the global
// pointer is never disturbed.
func execGcloud(configName string, args ...string) error {
	cmd := exec.Command("gcloud", args...)
	env := make([]string, 0, len(os.Environ())+1)
	for _, kv := range os.Environ() {
		if !strings.HasPrefix(kv, envActiveConfig+"=") {
			env = append(env, kv)
		}
	}
	cmd.Env = append(env, envActiveConfig+"="+configName)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// parseArgs splits args into positionals and registered flags. Both
// "--flag value" and "--flag=value" are accepted; a lone "-" is positional.
func parseArgs(args []string, strFlags map[string]*string, boolFlags map[string]*bool) ([]string, error) {
	var pos []string
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if !strings.HasPrefix(arg, "-") || arg == "-" {
			pos = append(pos, arg)
			continue
		}
		name, val, hasVal := arg, "", false
		if j := strings.IndexByte(arg, '='); j >= 0 {
			name, val, hasVal = arg[:j], arg[j+1:], true
		}
		if dst, ok := strFlags[name]; ok {
			if !hasVal {
				i++
				if i >= len(args) {
					return nil, fmt.Errorf("flag %s needs a value", name)
				}
				val = args[i]
			}
			*dst = val
			continue
		}
		if dst, ok := boolFlags[name]; ok {
			if hasVal {
				return nil, fmt.Errorf("flag %s takes no value", name)
			}
			*dst = true
			continue
		}
		return nil, fmt.Errorf("unknown flag %s", name)
	}
	return pos, nil
}

const usage = `gctx: gcloud configuration + ADC context switcher

A context is a gcloud named configuration plus, optionally, a captured
Application Default Credentials file (adc-<name>.json) that gctx binds to
GOOGLE_APPLICATION_CREDENTIALS on per-shell switches.

Usage:
  gctx                    List contexts
  gctx <name>             Switch context in this shell only (needs the wrapper
                          from 'gctx init')
  gctx -                  Switch back to the previous per-shell context
  gctx reset              Drop per-shell overrides, follow the global context

Config commands:
  gctx config get-contexts [-o table|json|name]
                          List contexts. '*' marks the context effective for
                          this process; '(global)' marks the global pointer
                          when it differs.
  gctx config current-context
                          Print the effective context for this process.
  gctx config use-context <name>
                          Switch the global context (writes active_config);
                          affects every shell without a per-shell override.
  gctx config set-context <name> [--account <email>] [--project <id>]
                          Create or modify a context. --account only binds an
                          account reference; run 'gctx login <name>' to
                          authenticate it.
  gctx config delete-context <name>
                          Delete a context and its captured ADC. Refuses to
                          delete the global current context.
  gctx config rename-context <old> <new>
                          Rename a context, its ADC file, and the global
                          pointer if it pointed at <old>.
  gctx config view        Show the effective context's resolved settings.

Auth:
  gctx login <name> [--project <id>]
                          Create <name> if missing, then 'gcloud auth login
                          --no-update-adc' pinned to it.
  gctx adc login <name>   Run 'gcloud auth application-default login' pinned to
                          <name> and capture the result as adc-<name>.json,
                          preserving any existing well-known ADC file.
  gctx adc capture <name> Adopt the existing well-known ADC file by moving it
                          to adc-<name>.json.
  gctx adc list           List captured ADC files.

Shell plumbing:
  gctx env <name> [--shell posix|fish|powershell|nu]
                          Emit env mutations for a per-shell switch.
  gctx env --reset [--shell ...]
                          Emit the unsets to fall back to global behavior.
  gctx init <posix|bash|zsh|fish|powershell|nu>
                          Print the shell wrapper.
  gctx completion <bash|zsh|fish|powershell|nu>
                          Print shell completions.

Misc:
  gctx version            Print the version.
  gctx help               Show this help.
`

package main

import (
	"encoding/json"
	"fmt"
	"strings"
)

// envOp is one environment mutation: set key=value or unset key.
type envOp struct {
	key   string
	value string
	set   bool
}

func (a *app) runEnv(args []string) int {
	shell := "posix"
	reset := false
	pos, err := parseArgs(args, map[string]*string{"--shell": &shell}, map[string]*bool{"--reset": &reset})
	if err != nil {
		return a.errf("%v", err)
	}
	dialect, ok := normalizeShell(shell)
	if !ok {
		return a.errf("unknown shell %q (want posix, fish, powershell or nu)", shell)
	}

	var ops []envOp
	switch {
	case reset:
		if len(pos) != 0 {
			return a.errf("usage: gctx env --reset [--shell <shell>] (no context name)")
		}
		ops = []envOp{{key: envActiveConfig}, {key: envADC}}
	case len(pos) == 1:
		s := a.store()
		name := pos[0]
		c, found := s.lookup(name)
		if !found {
			return a.unknownContext(s, name)
		}
		ops = append(ops, envOp{key: envActiveConfig, value: name, set: true})
		if c.adc != "" {
			ops = append(ops, envOp{key: envADC, value: c.adc, set: true})
		} else {
			ops = append(ops, envOp{key: envADC})
			if fileExists(s.wellKnownADC()) {
				fmt.Fprintf(a.stderr,
					"gctx: warning: context %q has no captured ADC, but %s exists.\n"+
						"Client libraries (Terraform, SDKs) will silently fall back to that file, which may\n"+
						"belong to a different identity. Capture ADC for this context with `gctx adc login %s`.\n",
					name, s.wellKnownADC(), name)
			}
		}
	default:
		return a.errf("usage: gctx env <name> [--shell <shell>] | gctx env --reset [--shell <shell>]")
	}

	a.emitEnv(dialect, ops)
	return 0
}

func normalizeShell(shell string) (string, bool) {
	switch shell {
	case "posix", "bash", "zsh", "sh":
		return "posix", true
	case "fish":
		return "fish", true
	case "powershell", "pwsh":
		return "powershell", true
	case "nu", "nushell":
		return "nu", true
	}
	return "", false
}

// emitEnv renders ops on stdout. Adding a shell means adding a case here,
// never new logic in the wrappers.
func (a *app) emitEnv(dialect string, ops []envOp) {
	switch dialect {
	case "posix":
		for _, op := range ops {
			if op.set {
				fmt.Fprintf(a.stdout, "export %s=%s\n", op.key, quotePosix(op.value))
			} else {
				fmt.Fprintf(a.stdout, "unset %s\n", op.key)
			}
		}
	case "fish":
		for _, op := range ops {
			if op.set {
				fmt.Fprintf(a.stdout, "set -gx %s %s\n", op.key, quoteFish(op.value))
			} else {
				fmt.Fprintf(a.stdout, "set -e %s\n", op.key)
			}
		}
	case "powershell":
		for _, op := range ops {
			if op.set {
				fmt.Fprintf(a.stdout, "$env:%s = %s\n", op.key, quotePowershell(op.value))
			} else {
				fmt.Fprintf(a.stdout, "Remove-Item Env:%s -ErrorAction Ignore\n", op.key)
			}
		}
	case "nu":
		// A JSON map applied by the nu wrapper; null means unset. Built by
		// hand for deterministic key order.
		var b strings.Builder
		b.WriteByte('{')
		for i, op := range ops {
			if i > 0 {
				b.WriteByte(',')
			}
			kj, _ := json.Marshal(op.key)
			b.Write(kj)
			b.WriteByte(':')
			if op.set {
				vj, _ := json.Marshal(op.value)
				b.Write(vj)
			} else {
				b.WriteString("null")
			}
		}
		b.WriteByte('}')
		fmt.Fprintln(a.stdout, b.String())
	}
}

func quotePosix(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

func quoteFish(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `'`, `\'`)
	return "'" + s + "'"
}

func quotePowershell(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "''") + "'"
}

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"text/tabwriter"
)

func (a *app) runConfig(args []string) int {
	if len(args) == 0 {
		fmt.Fprint(a.stdout, usage)
		return 0
	}
	cmd, rest := args[0], args[1:]
	switch cmd {
	case "get-contexts":
		return a.runGetContexts(rest)
	case "current-context":
		return a.runCurrentContext(rest)
	case "use-context":
		return a.runUseContext(rest)
	case "set-context":
		return a.runSetContext(rest)
	case "delete-context":
		return a.runDeleteContext(rest)
	case "rename-context":
		return a.runRenameContext(rest)
	case "protect":
		return a.runProtect(rest)
	case "unprotect":
		return a.runUnprotect(rest)
	case "view":
		return a.runView(rest)
	default:
		return a.errf("unknown config command %q, run `gctx help` for usage", cmd)
	}
}

func (a *app) runGetContexts(args []string) int {
	output := "table"
	pos, err := parseArgs(args, map[string]*string{"-o": &output, "--output": &output}, nil)
	if err != nil {
		return a.errf("%v", err)
	}
	if len(pos) != 0 {
		return a.errf("get-contexts takes no positional arguments")
	}
	s := a.store()
	cs := s.list()
	effective, _ := a.effectiveName(s)
	global := s.globalName()

	switch output {
	case "name":
		for _, c := range cs {
			fmt.Fprintln(a.stdout, c.name)
		}
	case "json":
		type record struct {
			Name    string `json:"name"`
			Current bool   `json:"current"`
			Global  bool   `json:"global"`
			Account string `json:"account"`
			Project string `json:"project"`
			ADC     string `json:"adc"`
		}
		records := make([]record, 0, len(cs))
		for _, c := range cs {
			records = append(records, record{c.name, c.name == effective, c.name == global, c.account, c.project, c.adc})
		}
		enc := json.NewEncoder(a.stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(records); err != nil {
			return a.errf("%v", err)
		}
	case "table":
		if len(cs) == 0 {
			fmt.Fprintf(a.stderr, "gctx: no contexts found in %s (create one with `gctx login <name>`)\n", s.dir)
			return 0
		}
		tw := tabwriter.NewWriter(a.stdout, 0, 0, 3, ' ', 0)
		fmt.Fprintln(tw, "CURRENT\tNAME\tACCOUNT\tPROJECT\tADC")
		for _, c := range cs {
			marker := ""
			switch {
			case c.name == effective:
				marker = "*"
			case c.name == global:
				marker = "(global)"
			}
			fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n", marker, c.name, c.account, c.project, c.adc)
		}
		tw.Flush()
	default:
		return a.errf("unknown output format %q (want table, json or name)", output)
	}
	return 0
}

func (a *app) runCurrentContext(args []string) int {
	if len(args) != 0 {
		return a.errf("current-context takes no arguments")
	}
	s := a.store()
	name, _ := a.effectiveName(s)
	if _, ok := s.lookup(name); !ok {
		return a.unknownContext(s, name)
	}
	fmt.Fprintln(a.stdout, name)
	return 0
}

func (a *app) runUseContext(args []string) int {
	if len(args) != 1 {
		return a.errf("usage: gctx config use-context <name>")
	}
	name := args[0]
	s := a.store()
	if _, ok := s.lookup(name); !ok {
		return a.unknownContext(s, name)
	}
	if err := s.setGlobalName(name); err != nil {
		return a.errf("%v", err)
	}
	fmt.Fprintf(a.stderr, "Switched global context to %q.\n", name)
	a.syncWellKnownADC(s, name)
	return 0
}

func (a *app) runSetContext(args []string) int {
	var account, project string
	pos, err := parseArgs(args, map[string]*string{"--account": &account, "--project": &project}, nil)
	if err != nil {
		return a.errf("%v", err)
	}
	if len(pos) != 1 {
		return a.errf("usage: gctx config set-context <name> [--account <email>] [--project <id>]")
	}
	name := pos[0]
	if !contextNameRE.MatchString(name) {
		return a.errf("invalid context name %q: use lowercase letters, digits and hyphens, starting with a letter", name)
	}
	s := a.store()
	path := s.configPath(name)
	content := ""
	created := true
	if data, err := os.ReadFile(path); err == nil {
		content = string(data)
		created = false
	}
	if account != "" {
		content = setINI(content, "core", "account", account)
	}
	if project != "" {
		content = setINI(content, "core", "project", project)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return a.errf("%v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return a.errf("%v", err)
	}
	if created {
		fmt.Fprintf(a.stderr, "Created context %q.\n", name)
	} else {
		fmt.Fprintf(a.stderr, "Updated context %q.\n", name)
	}
	return 0
}

func (a *app) runDeleteContext(args []string) int {
	args, force := takeForce(args)
	if len(args) != 1 {
		return a.errf("usage: gctx config delete-context <name> [--force]")
	}
	name := args[0]
	s := a.store()
	if _, ok := s.lookup(name); !ok {
		return a.unknownContext(s, name)
	}
	if a.blockedByProtection(s, name, "delete", force) {
		return 1
	}
	if name == s.globalName() {
		return a.errf("refusing to delete %q: it is the global current context; switch away first with `gctx config use-context <other>`", name)
	}
	if err := os.Remove(s.configPath(name)); err != nil {
		return a.errf("%v", err)
	}
	if fileExists(s.adcPath(name)) {
		if err := os.Remove(s.adcPath(name)); err != nil {
			return a.errf("%v", err)
		}
	}
	// Never leave the well-known link dangling at the deleted ADC file.
	wk := s.wellKnownADC()
	if isLink(wk) {
		if got, err := os.Readlink(wk); err == nil && got == "adc-"+name+".json" {
			os.Remove(wk)
		}
	}
	// Leave no orphan mark behind for a future context that reuses the name.
	if s.isProtected(name) {
		os.Remove(s.protectPath(name))
	}
	fmt.Fprintf(a.stderr, "Deleted context %q.\n", name)
	return 0
}

func (a *app) runRenameContext(args []string) int {
	args, force := takeForce(args)
	if len(args) != 2 {
		return a.errf("usage: gctx config rename-context <old> <new> [--force]")
	}
	oldName, newName := args[0], args[1]
	s := a.store()
	if _, ok := s.lookup(oldName); !ok {
		return a.unknownContext(s, oldName)
	}
	if a.blockedByProtection(s, oldName, "rename", force) {
		return 1
	}
	if !contextNameRE.MatchString(newName) {
		return a.errf("invalid context name %q: use lowercase letters, digits and hyphens, starting with a letter", newName)
	}
	if _, ok := s.lookup(newName); ok {
		return a.errf("context %q already exists", newName)
	}
	if err := os.Rename(s.configPath(oldName), s.configPath(newName)); err != nil {
		return a.errf("%v", err)
	}
	if fileExists(s.adcPath(oldName)) {
		if err := os.Rename(s.adcPath(oldName), s.adcPath(newName)); err != nil {
			return a.errf("%v", err)
		}
	}
	if s.globalName() == oldName {
		if err := s.setGlobalName(newName); err != nil {
			return a.errf("%v", err)
		}
	}
	wk := s.wellKnownADC()
	if isLink(wk) {
		if got, err := os.Readlink(wk); err == nil && got == "adc-"+oldName+".json" {
			a.syncWellKnownADC(s, newName)
		}
	}
	// A rename must not quietly drop protection.
	if s.isProtected(oldName) {
		if err := os.Rename(s.protectPath(oldName), s.protectPath(newName)); err != nil {
			return a.errf("%v", err)
		}
	}

	fmt.Fprintf(a.stderr, "Renamed context %q to %q.\n", oldName, newName)
	return 0
}

func (a *app) runView(args []string) int {
	if len(args) != 0 {
		return a.errf("view takes no arguments")
	}
	s := a.store()
	name, fromEnv := a.effectiveName(s)
	c, ok := s.lookup(name)
	if !ok {
		return a.unknownContext(s, name)
	}
	source := fmt.Sprintf("global pointer (%s)", filepath.Join(s.dir, activeConfigFile))
	if fromEnv {
		source = fmt.Sprintf("environment (%s)", envActiveConfig)
	}
	adc := "(none)"
	switch {
	case c.adc != "":
		adc = c.adc
	case fileExists(s.wellKnownADC()):
		adc = fmt.Sprintf("well-known fallback (%s), not captured for this context; run `gctx adc login %s`", s.wellKnownADC(), name)
	}
	tw := tabwriter.NewWriter(a.stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintf(tw, "name:\t%s\n", c.name)
	fmt.Fprintf(tw, "source:\t%s\n", source)
	fmt.Fprintf(tw, "account:\t%s\n", orUnset(c.account))
	fmt.Fprintf(tw, "project:\t%s\n", orUnset(c.project))
	fmt.Fprintf(tw, "adc:\t%s\n", adc)
	fmt.Fprintf(tw, "config dir:\t%s\n", s.dir)
	tw.Flush()
	return 0
}

func orUnset(s string) string {
	if s == "" {
		return "(unset)"
	}
	return s
}

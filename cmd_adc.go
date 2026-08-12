package main

import (
	"fmt"
	"os"
	"text/tabwriter"
)

func (a *app) runLogin(args []string) int {
	var project string
	pos, err := parseArgs(args, map[string]*string{"--project": &project}, nil)
	if err != nil {
		return a.errf("%v", err)
	}
	if len(pos) != 1 {
		return a.errf("usage: gctx login <name> [--project <id>]")
	}
	name := pos[0]
	s := a.store()
	if _, ok := s.lookup(name); !ok {
		if !contextNameRE.MatchString(name) {
			return a.errf("invalid context name %q: use lowercase letters, digits and hyphens, starting with a letter", name)
		}
		if code := a.runSetContext([]string{name}); code != 0 {
			return code
		}
	}
	// --no-update-adc: user auth must never rewrite the ADC file.
	if err := a.runGcloud(name, "auth", "login", "--no-update-adc"); err != nil {
		return a.errf("gcloud auth login failed: %v", err)
	}
	if project != "" {
		if code := a.runSetContext([]string{name, "--project", project}); code != 0 {
			return code
		}
	}
	c, _ := s.lookup(name)
	fmt.Fprintf(a.stderr, "Logged in context %q.\nNext steps:\n  gctx %s\t\tswitch this shell to it\n", name, name)
	if c.adc == "" {
		fmt.Fprintf(a.stderr, "  gctx adc login %s\tcapture Application Default Credentials for it\n", name)
	}
	return 0
}

func (a *app) runADC(args []string) int {
	if len(args) == 0 {
		return a.errf("usage: gctx adc <login|capture|list>")
	}
	cmd, rest := args[0], args[1:]
	switch cmd {
	case "login":
		return a.runADCLogin(rest)
	case "capture":
		return a.runADCCapture(rest)
	case "list":
		return a.runADCList(rest)
	default:
		return a.errf("unknown adc command %q (want login, capture or list)", cmd)
	}
}

// runADCLogin captures ADC for a context. Any existing well-known ADC file is
// set aside first and restored afterwards, on failure and success alike.
func (a *app) runADCLogin(args []string) int {
	if len(args) != 1 {
		return a.errf("usage: gctx adc login <name>")
	}
	name := args[0]
	s := a.store()
	c, ok := s.lookup(name)
	if !ok {
		return a.unknownContext(s, name)
	}

	wk := s.wellKnownADC()
	backup := wk + ".gctx-backup"
	haveBackup := false
	// Lstat-based: a dangling gctx link must be set aside too, or gcloud
	// would write the fresh credential through it into the wrong file.
	if pathExists(wk) {
		if err := os.Rename(wk, backup); err != nil {
			return a.errf("could not set aside existing ADC file: %v", err)
		}
		haveBackup = true
	}
	restore := func() {
		if haveBackup {
			if err := os.Rename(backup, wk); err != nil {
				fmt.Fprintf(a.stderr, "gctx: warning: could not restore %s from %s: %v\n", wk, backup, err)
			}
		}
	}

	if err := a.runGcloud(name, "auth", "application-default", "login"); err != nil {
		restore()
		return a.errf("gcloud auth application-default login failed: %v", err)
	}
	if !fileExists(wk) {
		restore()
		return a.errf("gcloud reported success but %s does not exist", wk)
	}
	if c.project != "" {
		// Warn only: the credentials are still good without a quota project.
		if err := a.runGcloud(name, "auth", "application-default", "set-quota-project", c.project); err != nil {
			fmt.Fprintf(a.stderr, "gctx: warning: could not set quota project %q: %v\n", c.project, err)
		}
	}
	if err := os.Rename(wk, s.adcPath(name)); err != nil {
		restore()
		return a.errf("could not capture ADC file: %v", err)
	}
	restore()
	fmt.Fprintf(a.stderr, "Captured ADC for context %q at %s.\n", name, s.adcPath(name))
	if name == s.globalName() {
		a.syncWellKnownADC(s, name)
	}
	return 0
}

// runADCCapture moves (not copies) the well-known ADC file to adc-<name>.json
// so no stale fallback identity remains.
func (a *app) runADCCapture(args []string) int {
	if len(args) != 1 {
		return a.errf("usage: gctx adc capture <name>")
	}
	name := args[0]
	s := a.store()
	if _, ok := s.lookup(name); !ok {
		return a.unknownContext(s, name)
	}
	wk := s.wellKnownADC()
	if isLink(wk) {
		return a.errf("%s is a gctx-managed link, nothing to capture (renew with `gctx adc login %s` instead)", wk, name)
	}
	if !fileExists(wk) {
		return a.errf("no well-known ADC file at %s to capture (run `gctx adc login %s` instead)", wk, name)
	}
	if err := os.Rename(wk, s.adcPath(name)); err != nil {
		return a.errf("%v", err)
	}
	fmt.Fprintf(a.stderr, "Captured ADC for context %q at %s.\n", name, s.adcPath(name))
	if name == s.globalName() {
		a.syncWellKnownADC(s, name)
	}
	return 0
}

func (a *app) runADCList(args []string) int {
	if len(args) != 0 {
		return a.errf("adc list takes no arguments")
	}
	s := a.store()
	var captured []context
	for _, c := range s.list() {
		if c.adc != "" {
			captured = append(captured, c)
		}
	}
	if len(captured) == 0 {
		fmt.Fprintf(a.stderr, "gctx: no captured ADC files in %s (run `gctx adc login <name>`)\n", s.dir)
		return 0
	}
	tw := tabwriter.NewWriter(a.stdout, 0, 0, 3, ' ', 0)
	fmt.Fprintln(tw, "NAME\tADC")
	for _, c := range captured {
		fmt.Fprintf(tw, "%s\t%s\n", c.name, c.adc)
	}
	tw.Flush()
	return 0
}

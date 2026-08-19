package main

import (
	"fmt"
	"os"
)

// A protected context is one you would rather not lose to a typo. Marking it
// makes gctx refuse the two operations it cannot undo - delete-context and
// rename-context - unless --force is passed as well.
//
// The mark is a marker file next to the context's captured ADC, following the
// adc-<name>.json convention. It deliberately does not live inside the gcloud
// config file: that file belongs to gcloud, and an unknown section there is one
// gcloud rewrite away from disappearing.
//
// This guards gctx's own destructive commands. It is not a permission boundary
// and does not restrain gcloud itself; IAM is the place for that.

func (s *store) protectPath(name string) string {
	return joinConfig(s.dir, "protected-"+name)
}

func (s *store) isProtected(name string) bool {
	return fileExists(s.protectPath(name))
}

func (a *app) runProtect(args []string) int {
	if len(args) != 1 {
		return a.errf("usage: gctx config protect <name>")
	}
	name := args[0]
	s := a.store()
	if _, ok := s.lookup(name); !ok {
		return a.unknownContext(s, name)
	}
	if s.isProtected(name) {
		fmt.Fprintf(a.stderr, "Context %q is already protected.\n", name)
		return 0
	}
	if err := os.WriteFile(s.protectPath(name), nil, 0o600); err != nil {
		return a.errf("%v", err)
	}
	fmt.Fprintf(a.stderr, "Protected context %q. delete-context and rename-context now need --force.\n", name)
	return 0
}

func (a *app) runUnprotect(args []string) int {
	if len(args) != 1 {
		return a.errf("usage: gctx config unprotect <name>")
	}
	name := args[0]
	s := a.store()
	if _, ok := s.lookup(name); !ok {
		return a.unknownContext(s, name)
	}
	if !s.isProtected(name) {
		fmt.Fprintf(a.stderr, "Context %q is not protected.\n", name)
		return 0
	}
	if err := os.Remove(s.protectPath(name)); err != nil {
		return a.errf("%v", err)
	}
	fmt.Fprintf(a.stderr, "Unprotected context %q.\n", name)
	return 0
}

// blockedByProtection reports whether verb should stop on name. It also strips
// --force from args so callers keep their existing argument handling.
func (a *app) blockedByProtection(s *store, name, verb string, force bool) bool {
	if !s.isProtected(name) || force {
		return false
	}
	a.errf("refusing to %s %q: it is protected; pass --force, or run `gctx config unprotect %s` first", verb, name, name)
	return true
}

// takeForce removes a --force flag from args and reports whether it was there.
func takeForce(args []string) ([]string, bool) {
	out := make([]string, 0, len(args))
	force := false
	for _, arg := range args {
		if arg == "--force" {
			force = true
			continue
		}
		out = append(out, arg)
	}
	return out, force
}

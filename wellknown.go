package main

import (
	"fmt"
	"os"
)

// syncWellKnownADC keeps the well-known ADC path in step with the global
// context: a symlink to the context's captured ADC file, so shells without
// a per-shell override resolve the right identity and renewals propagate.
// A symlink at the well-known path is gctx-managed and freely replaced; a
// regular file is foreign and is set aside, never overwritten. Failures
// warn instead of erroring: the switch itself has already happened.
func (a *app) syncWellKnownADC(s *store, name string) {
	wk := s.wellKnownADC()
	c, ok := s.lookup(name)
	target := "adc-" + name + ".json" // relative: same directory as wk

	if !ok || c.adc == "" {
		if isLink(wk) {
			if err := os.Remove(wk); err != nil {
				a.warnf("could not remove ADC link %s: %v", wk, err)
				return
			}
			a.warnf("context %q has no captured ADC; removed the well-known ADC link. Capture one with `gctx adc login %s`.", name, name)
		}
		return
	}

	switch {
	case isLink(wk):
		if got, err := os.Readlink(wk); err == nil && got == target {
			return
		}
		if err := os.Remove(wk); err != nil {
			a.warnf("could not replace ADC link %s: %v", wk, err)
			return
		}
	case pathExists(wk):
		saved := freePath(wk + ".gctx-saved")
		if err := os.Rename(wk, saved); err != nil {
			a.warnf("could not set aside %s: %v", wk, err)
			return
		}
		a.warnf("set aside the existing ADC file %s to %s", wk, saved)
	}
	if err := os.Symlink(target, wk); err != nil {
		a.warnf("could not link %s to %s: %v", wk, target, err)
	}
}

func (a *app) warnf(format string, args ...any) {
	fmt.Fprintf(a.stderr, "gctx: warning: %s\n", fmt.Sprintf(format, args...))
}

// isLink reports whether path is a symlink, without following it.
func isLink(path string) bool {
	fi, err := os.Lstat(path)
	return err == nil && fi.Mode()&os.ModeSymlink != 0
}

// pathExists reports whether path exists at all, dangling symlinks included.
func pathExists(path string) bool {
	_, err := os.Lstat(path)
	return err == nil
}

// freePath returns base if unoccupied, else base.2, base.3, ...
func freePath(base string) string {
	p := base
	for i := 2; pathExists(p); i++ {
		p = fmt.Sprintf("%s.%d", base, i)
	}
	return p
}

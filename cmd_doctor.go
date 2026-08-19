package main

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// gcloud writes its own debug logs under <config-dir>/logs/<date>/*.log, and an
// auth flow leaves the resulting tokens in them in clear text. `gcloud auth
// revoke` invalidates the credential server-side but does not touch the logs, so
// a refresh token can outlive the account it belonged to. Refresh tokens stay
// valid until they are revoked or go unused for months, which makes an old log
// directory a quiet credential store nobody remembers.
//
// gctx does not create these files and cannot stop gcloud writing them. It can
// point at them.
//
// Everything here reports paths and match counts only. The matched text is never
// printed, logged or returned - a tool that reads credentials to warn you about
// credentials has not helped.

type finding struct {
	path   string
	counts map[string]int
	// longLived is true when at least one match on this file outlives the hour
	// an access token gets.
	longLived bool
}

// Patterns match the shape of the value, not the field name around it. Field
// names show up in ordinary API traffic; these prefixes do not.
var credentialPatterns = []struct {
	label string
	re    *regexp.Regexp
	// longLived marks the ones still worth acting on. An access token expires
	// about an hour after it is written, so a log full of them is usually noise;
	// a refresh token or a private key stays useful until somebody revokes it.
	longLived bool
}{
	{"oauth refresh token", regexp.MustCompile(`1//[A-Za-z0-9._\-]{20,}`), true},
	{"service account private key", regexp.MustCompile(`-----BEGIN[ A-Z]*PRIVATE KEY-----`), true},
	{"oauth access token", regexp.MustCompile(`ya29\.[A-Za-z0-9._\-]{20,}`), false},
}

func (a *app) runDoctor(args []string) int {
	var fix bool
	for _, arg := range args {
		switch arg {
		case "--fix":
			fix = true
		case "-h", "--help":
			fmt.Fprint(a.stdout, doctorUsage)
			return 0
		default:
			return a.errf("unknown flag %q, run `gctx doctor --help` for usage", arg)
		}
	}

	logDir := filepath.Join(a.configDir(), "logs")
	findings, err := scanForCredentials(logDir)
	if err != nil {
		return a.errf("%v", err)
	}

	if len(findings) == 0 {
		fmt.Fprintf(a.stdout, "No credential material found in %s\n", logDir)
		return 0
	}

	total, live := 0, 0
	for _, f := range findings {
		for _, n := range f.counts {
			total += n
		}
		if f.longLived {
			live++
		}
	}
	fmt.Fprintf(a.stdout, "Found credential material in %d log file(s) under %s\n\n", len(findings), logDir)
	for _, f := range findings {
		labels := make([]string, 0, len(f.counts))
		for label, n := range f.counts {
			labels = append(labels, fmt.Sprintf("%s x%d", label, n))
		}
		sort.Strings(labels)
		mark := " "
		if f.longLived {
			mark = "!"
		}
		fmt.Fprintf(a.stdout, "%s %s\n      %s\n", mark, f.path, strings.Join(labels, ", "))
	}

	if !fix {
		fmt.Fprintf(a.stdout, "\n%d match(es) in %d file(s).\n", total, len(findings))
		if live > 0 {
			fmt.Fprintf(a.stdout, "%d file(s) marked ! hold a refresh token or private key. Those stay valid until\n", live)
			fmt.Fprint(a.stdout, "revoked, so start there - deleting the log does not un-issue what was written.\n")
		}
		if live < len(findings) {
			fmt.Fprintf(a.stdout, "The other %d hold access tokens only, which expire about an hour after they are\n", len(findings)-live)
			fmt.Fprint(a.stdout, "written; those are almost certainly dead already.\n")
		}
		fmt.Fprint(a.stdout, "Run `gctx doctor --fix` to delete the files listed above.\n")
		return 1
	}

	removed := 0
	for _, f := range findings {
		if err := os.Remove(f.path); err != nil {
			fmt.Fprintf(a.stderr, "could not remove %s: %v\n", f.path, err)
			continue
		}
		removed++
	}
	fmt.Fprintf(a.stdout, "\nRemoved %d of %d file(s).\n", removed, len(findings))
	if removed != len(findings) {
		return 1
	}
	return 0
}

// scanForCredentials walks dir and returns one entry per file that contains at
// least one match. A missing directory is not an error: plenty of installs have
// never produced a log.
func scanForCredentials(dir string) ([]finding, error) {
	var out []finding
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			// A missing logs dir is the normal case on a fresh install, and one
			// unreadable file should not abandon the rest of the scan.
			return nil
		}
		if d.IsDir() || !strings.HasSuffix(path, ".log") {
			return nil
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		counts := map[string]int{}
		longLived := false
		for _, p := range credentialPatterns {
			if n := len(p.re.FindAll(b, -1)); n > 0 {
				counts[p.label] = n
				longLived = longLived || p.longLived
			}
		}
		if len(counts) > 0 {
			out = append(out, finding{path: path, counts: counts, longLived: longLived})
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	// Long-lived first: that is the part of the list worth reading.
	sort.Slice(out, func(i, j int) bool {
		if out[i].longLived != out[j].longLived {
			return out[i].longLived
		}
		return out[i].path < out[j].path
	})
	return out, nil
}

const doctorUsage = `gctx doctor: look for credential material left in gcloud's own logs

gcloud records the tokens from an auth flow in its debug logs under
<config-dir>/logs. 'gcloud auth revoke' does not remove them, so a refresh token
can stay readable on disk long after the account is gone.

Usage:
  gctx doctor         Report affected files. Exits 1 when there is something to report.
  gctx doctor --fix   Delete the affected log files.

Files marked ! contain a refresh token or a private key, which stay valid until
revoked. The rest hold access tokens, which expire about an hour after they are
written.

Paths and match counts are printed. The matched text never is.
`

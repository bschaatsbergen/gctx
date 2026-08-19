package main

import (
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
)

const (
	defaultContextName = "default"
	wellKnownADCFile   = "application_default_credentials.json"
	activeConfigFile   = "active_config"
	envActiveConfig    = "CLOUDSDK_ACTIVE_CONFIG_NAME"
	envADC             = "GOOGLE_APPLICATION_CREDENTIALS"
)

// context is a gcloud named configuration plus an optional captured ADC file.
type context struct {
	name    string
	account string
	project string
	adc     string
}

// store reads and writes the gcloud config dir directly; gcloud is never
// invoked on read paths.
type store struct {
	dir string
}

func (a *app) store() *store {
	return &store{dir: a.configDir()}
}

func (a *app) configDir() string {
	if v := a.getenv("CLOUDSDK_CONFIG"); v != "" {
		return v
	}
	if runtime.GOOS == "windows" {
		return filepath.Join(a.getenv("APPDATA"), "gcloud")
	}
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		home = a.getenv("HOME")
	}
	return filepath.Join(home, ".config", "gcloud")
}

func (s *store) configPath(name string) string {
	return filepath.Join(s.dir, "configurations", "config_"+name)
}

func (s *store) adcPath(name string) string {
	return filepath.Join(s.dir, "adc-"+name+".json")
}

func (s *store) wellKnownADC() string {
	return filepath.Join(s.dir, wellKnownADCFile)
}

func (s *store) list() []context {
	entries, err := os.ReadDir(filepath.Join(s.dir, "configurations"))
	if err != nil {
		return nil
	}
	var out []context
	for _, e := range entries {
		name, ok := strings.CutPrefix(e.Name(), "config_")
		if !ok || e.IsDir() || name == "" {
			continue
		}
		c := context{name: name}
		if data, err := os.ReadFile(s.configPath(name)); err == nil {
			ini := parseINI(string(data))
			c.account = ini["core"]["account"]
			c.project = ini["core"]["project"]
		}
		if fileExists(s.adcPath(name)) {
			c.adc = s.adcPath(name)
		}
		out = append(out, c)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].name < out[j].name })
	return out
}

func (s *store) lookup(name string) (context, bool) {
	for _, c := range s.list() {
		if c.name == name {
			return c, true
		}
	}
	return context{}, false
}

func (s *store) globalName() string {
	data, err := os.ReadFile(filepath.Join(s.dir, activeConfigFile))
	if err != nil || strings.TrimSpace(string(data)) == "" {
		return defaultContextName
	}
	return strings.TrimSpace(string(data))
}

func (s *store) setGlobalName(name string) error {
	return os.WriteFile(filepath.Join(s.dir, activeConfigFile), []byte(name), 0o644)
}

// effectiveName is the env override if set, else the global pointer.
func (a *app) effectiveName(s *store) (name string, fromEnv bool) {
	if v := a.getenv(envActiveConfig); v != "" {
		return v, true
	}
	return s.globalName(), false
}

// contextNameRE mirrors gcloud's configuration name rules and keeps names
// safe to splice into paths.
var contextNameRE = regexp.MustCompile(`^[a-z][-a-z0-9]*$`)

func fileExists(path string) bool {
	fi, err := os.Stat(path)
	return err == nil && !fi.IsDir()
}

// joinConfig keeps path building in one place for files gctx owns.
func joinConfig(dir, name string) string {
	return filepath.Join(dir, name)
}

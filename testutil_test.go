package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func newTestApp(dir string, env map[string]string) (*app, *bytes.Buffer, *bytes.Buffer) {
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	if env == nil {
		env = map[string]string{}
	}
	if _, ok := env["CLOUDSDK_CONFIG"]; !ok {
		env["CLOUDSDK_CONFIG"] = dir
	}
	a := &app{
		stdout: stdout,
		stderr: stderr,
		getenv: func(k string) string { return env[k] },
	}
	return a, stdout, stderr
}

const configA = `[core]
account = a@example.com
project = proj-a
disable_usage_reporting = True

[compute]
region = europe-west4
`

const configB = `[core]
account = b@example.com
project = proj-b
`

func writeFixture(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "configurations", "config_a"), configA)
	mustWrite(t, filepath.Join(dir, "configurations", "config_b"), configB)
	mustWrite(t, filepath.Join(dir, "active_config"), "a")
	mustWrite(t, filepath.Join(dir, "adc-b.json"), `{"type":"authorized_user","fake":"b"}`)
	return dir
}

func mustWrite(t *testing.T, path, content string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))
}

// mustWriteRaw is for fake-gcloud closures with no *testing.T in scope.
func mustWriteRaw(path, content string) {
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		panic(err)
	}
}

func mustRead(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	require.NoError(t, err)
	return string(b)
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

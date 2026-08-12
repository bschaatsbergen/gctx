package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func tableRow(t *testing.T, out, name string) string {
	t.Helper()
	for _, line := range strings.Split(out, "\n")[1:] {
		for _, f := range strings.Fields(line) {
			if f == name {
				return line
			}
		}
	}
	require.Failf(t, "row not found", "no row for %q in:\n%s", name, out)
	return ""
}

func TestGetContextsTable(t *testing.T) {
	dir := writeFixture(t)
	a, stdout, _ := newTestApp(dir, nil)
	require.Equal(t, 0, a.run([]string{"config", "get-contexts"}))
	out := stdout.String()

	header := strings.Split(out, "\n")[0]
	for _, col := range []string{"CURRENT", "NAME", "ACCOUNT", "PROJECT", "ADC"} {
		assert.Contains(t, header, col)
	}

	rowA := tableRow(t, out, "a")
	assert.True(t, strings.HasPrefix(rowA, "*"), "effective context not marked: %q", rowA)
	assert.Contains(t, rowA, "a@example.com")
	assert.Contains(t, rowA, "proj-a")

	rowB := tableRow(t, out, "b")
	assert.False(t, strings.HasPrefix(rowB, "*"))
	assert.Contains(t, rowB, filepath.Join(dir, "adc-b.json"))
}

func TestGetContextsTableEnvOverride(t *testing.T) {
	dir := writeFixture(t)
	a, stdout, _ := newTestApp(dir, map[string]string{"CLOUDSDK_ACTIVE_CONFIG_NAME": "b"})
	require.Equal(t, 0, a.run([]string{"config", "get-contexts"}))
	out := stdout.String()

	assert.True(t, strings.HasPrefix(tableRow(t, out, "b"), "*"))
	rowA := tableRow(t, out, "a")
	assert.False(t, strings.HasPrefix(rowA, "*"))
	assert.Contains(t, rowA, "global", "global pointer not annotated when it differs from effective")
}

func TestGetContextsNames(t *testing.T) {
	dir := writeFixture(t)
	a, stdout, _ := newTestApp(dir, nil)
	require.Equal(t, 0, a.run([]string{"config", "get-contexts", "-o", "name"}))
	assert.Equal(t, "a\nb\n", stdout.String())
}

func TestGetContextsJSON(t *testing.T) {
	dir := writeFixture(t)
	a, stdout, _ := newTestApp(dir, map[string]string{"CLOUDSDK_ACTIVE_CONFIG_NAME": "b"})
	require.Equal(t, 0, a.run([]string{"config", "get-contexts", "-o", "json"}))

	var got []struct {
		Name    string `json:"name"`
		Current bool   `json:"current"`
		Global  bool   `json:"global"`
		Account string `json:"account"`
		Project string `json:"project"`
		ADC     string `json:"adc"`
	}
	require.NoError(t, json.Unmarshal(stdout.Bytes(), &got))
	require.Len(t, got, 2)

	ra, rb := got[0], got[1]
	assert.Equal(t, "a", ra.Name)
	assert.False(t, ra.Current)
	assert.True(t, ra.Global)
	assert.Equal(t, "a@example.com", ra.Account)
	assert.Equal(t, "proj-a", ra.Project)
	assert.Empty(t, ra.ADC)

	assert.Equal(t, "b", rb.Name)
	assert.True(t, rb.Current)
	assert.False(t, rb.Global)
	assert.Equal(t, "b@example.com", rb.Account)
	assert.Equal(t, "proj-b", rb.Project)
	assert.Equal(t, filepath.Join(dir, "adc-b.json"), rb.ADC)
}

func TestCurrentContextGlobal(t *testing.T) {
	dir := writeFixture(t)
	a, stdout, _ := newTestApp(dir, nil)
	require.Equal(t, 0, a.run([]string{"config", "current-context"}))
	assert.Equal(t, "a\n", stdout.String())
}

func TestCurrentContextEnvOverride(t *testing.T) {
	dir := writeFixture(t)
	a, stdout, _ := newTestApp(dir, map[string]string{"CLOUDSDK_ACTIVE_CONFIG_NAME": "b"})
	require.Equal(t, 0, a.run([]string{"config", "current-context"}))
	assert.Equal(t, "b\n", stdout.String())
}

func TestCurrentContextUnknownErrors(t *testing.T) {
	dir := writeFixture(t)
	a, stdout, stderr := newTestApp(dir, map[string]string{"CLOUDSDK_ACTIVE_CONFIG_NAME": "nosuch"})
	require.NotEqual(t, 0, a.run([]string{"config", "current-context"}))
	assert.Empty(t, stdout.String())
	assert.Contains(t, stderr.String(), "nosuch")
	assert.Contains(t, stderr.String(), "a")
}

func TestUseContext(t *testing.T) {
	dir := writeFixture(t)
	a, stdout, _ := newTestApp(dir, nil)
	require.Equal(t, 0, a.run([]string{"config", "use-context", "b"}))
	assert.Equal(t, "b", strings.TrimSpace(mustRead(t, filepath.Join(dir, "active_config"))))
	assert.Empty(t, stdout.String())
}

func TestUseContextUnknown(t *testing.T) {
	dir := writeFixture(t)
	a, _, stderr := newTestApp(dir, nil)
	require.NotEqual(t, 0, a.run([]string{"config", "use-context", "nosuch"}))
	assert.Equal(t, "a", strings.TrimSpace(mustRead(t, filepath.Join(dir, "active_config"))))
	assert.Contains(t, stderr.String(), "nosuch")
	assert.Contains(t, stderr.String(), "b")
}

func TestUseContextLinksCapturedADC(t *testing.T) {
	dir := writeFixture(t)
	wk := filepath.Join(dir, "application_default_credentials.json")
	a, _, _ := newTestApp(dir, nil)
	require.Equal(t, 0, a.run([]string{"config", "use-context", "b"}))
	requireLink(t, wk, "adc-b.json")
	assert.Equal(t, mustRead(t, filepath.Join(dir, "adc-b.json")), mustRead(t, wk))
}

func TestUseContextSavesForeignWellKnownFile(t *testing.T) {
	dir := writeFixture(t)
	wk := filepath.Join(dir, "application_default_credentials.json")
	mustWrite(t, wk, "FOREIGN")
	a, _, stderr := newTestApp(dir, nil)
	require.Equal(t, 0, a.run([]string{"config", "use-context", "b"}))
	requireLink(t, wk, "adc-b.json")
	assert.Equal(t, "FOREIGN", mustRead(t, wk+".gctx-saved"))
	assert.Contains(t, stderr.String(), ".gctx-saved")
}

func TestUseContextWithoutCapturedADCRemovesManagedLink(t *testing.T) {
	dir := writeFixture(t)
	wk := filepath.Join(dir, "application_default_credentials.json")
	require.NoError(t, os.Symlink("adc-b.json", wk))
	a, _, stderr := newTestApp(dir, nil)
	require.Equal(t, 0, a.run([]string{"config", "use-context", "a"}))
	assert.False(t, exists(wk))
	assert.False(t, isSymlink(wk))
	assert.Contains(t, stderr.String(), "gctx adc login a")
}

func TestUseContextWithoutCapturedADCLeavesForeignFile(t *testing.T) {
	dir := writeFixture(t)
	wk := filepath.Join(dir, "application_default_credentials.json")
	mustWrite(t, wk, "FOREIGN")
	a, _, _ := newTestApp(dir, nil)
	require.Equal(t, 0, a.run([]string{"config", "use-context", "a"}))
	assert.False(t, isSymlink(wk))
	assert.Equal(t, "FOREIGN", mustRead(t, wk))
}

func TestRenameContextRepointsWellKnownLink(t *testing.T) {
	dir := writeFixture(t)
	wk := filepath.Join(dir, "application_default_credentials.json")
	require.NoError(t, os.Symlink("adc-b.json", wk))
	a, _, _ := newTestApp(dir, nil)
	require.Equal(t, 0, a.run([]string{"config", "rename-context", "b", "c"}))
	requireLink(t, wk, "adc-c.json")
}

func TestDeleteContextRemovesItsWellKnownLink(t *testing.T) {
	dir := writeFixture(t)
	wk := filepath.Join(dir, "application_default_credentials.json")
	require.NoError(t, os.Symlink("adc-b.json", wk))
	a, _, _ := newTestApp(dir, nil)
	require.Equal(t, 0, a.run([]string{"config", "delete-context", "b"}))
	assert.False(t, exists(filepath.Join(dir, "adc-b.json")))
	assert.False(t, isSymlink(wk))
}

func TestSetContextCreates(t *testing.T) {
	dir := writeFixture(t)
	a, _, _ := newTestApp(dir, nil)
	require.Equal(t, 0, a.run([]string{"config", "set-context", "c", "--account", "c@example.com", "--project", "proj-c"}))
	ini := parseINI(mustRead(t, filepath.Join(dir, "configurations", "config_c")))
	assert.Equal(t, "c@example.com", ini["core"]["account"])
	assert.Equal(t, "proj-c", ini["core"]["project"])
	assert.Equal(t, "a", strings.TrimSpace(mustRead(t, filepath.Join(dir, "active_config"))))
}

func TestSetContextEditsPreservingOtherKeys(t *testing.T) {
	dir := writeFixture(t)
	a, _, _ := newTestApp(dir, nil)
	require.Equal(t, 0, a.run([]string{"config", "set-context", "a", "--project", "proj-a2"}))
	raw := mustRead(t, filepath.Join(dir, "configurations", "config_a"))
	ini := parseINI(raw)
	assert.Equal(t, "proj-a2", ini["core"]["project"])
	assert.Equal(t, "a@example.com", ini["core"]["account"])
	assert.Contains(t, raw, "disable_usage_reporting = True")
	assert.Contains(t, raw, "region = europe-west4")
}

func TestSetContextBareCreate(t *testing.T) {
	dir := writeFixture(t)
	a, _, _ := newTestApp(dir, nil)
	require.Equal(t, 0, a.run([]string{"config", "set-context", "d"}))
	assert.True(t, exists(filepath.Join(dir, "configurations", "config_d")))
}

func TestSetContextRejectsInvalidName(t *testing.T) {
	dir := writeFixture(t)
	a, _, stderr := newTestApp(dir, nil)
	for _, bad := range []string{"Bad", "has_underscore", "-leading", "sneaky/../path"} {
		assert.NotEqual(t, 0, a.run([]string{"config", "set-context", bad}), "accepted %q", bad)
	}
	assert.NotEmpty(t, stderr.String())
}

func TestDeleteContext(t *testing.T) {
	dir := writeFixture(t)
	a, _, _ := newTestApp(dir, nil)
	require.Equal(t, 0, a.run([]string{"config", "delete-context", "b"}))
	assert.False(t, exists(filepath.Join(dir, "configurations", "config_b")))
	assert.False(t, exists(filepath.Join(dir, "adc-b.json")))
}

func TestDeleteContextRefusesGlobalCurrent(t *testing.T) {
	dir := writeFixture(t)
	a, _, stderr := newTestApp(dir, map[string]string{"CLOUDSDK_ACTIVE_CONFIG_NAME": "b"})
	require.NotEqual(t, 0, a.run([]string{"config", "delete-context", "a"}))
	assert.True(t, exists(filepath.Join(dir, "configurations", "config_a")))
	assert.Contains(t, stderr.String(), "current")
}

func TestDeleteContextUnknown(t *testing.T) {
	dir := writeFixture(t)
	a, _, stderr := newTestApp(dir, nil)
	require.NotEqual(t, 0, a.run([]string{"config", "delete-context", "nosuch"}))
	assert.Contains(t, stderr.String(), "nosuch")
}

func TestRenameContextUpdatesActiveConfig(t *testing.T) {
	dir := writeFixture(t)
	a, _, _ := newTestApp(dir, nil)
	require.Equal(t, 0, a.run([]string{"config", "rename-context", "a", "c"}))
	assert.False(t, exists(filepath.Join(dir, "configurations", "config_a")))
	ini := parseINI(mustRead(t, filepath.Join(dir, "configurations", "config_c")))
	assert.Equal(t, "a@example.com", ini["core"]["account"])
	assert.Equal(t, "c", strings.TrimSpace(mustRead(t, filepath.Join(dir, "active_config"))))
}

func TestRenameContextMovesADC(t *testing.T) {
	dir := writeFixture(t)
	a, _, _ := newTestApp(dir, nil)
	require.Equal(t, 0, a.run([]string{"config", "rename-context", "b", "d"}))
	assert.False(t, exists(filepath.Join(dir, "adc-b.json")))
	assert.True(t, exists(filepath.Join(dir, "adc-d.json")))
	assert.Equal(t, "a", strings.TrimSpace(mustRead(t, filepath.Join(dir, "active_config"))))
}

func TestRenameContextRefusesExistingTarget(t *testing.T) {
	dir := writeFixture(t)
	a, _, stderr := newTestApp(dir, nil)
	require.NotEqual(t, 0, a.run([]string{"config", "rename-context", "b", "a"}))
	assert.Contains(t, stderr.String(), "a")
}

func TestRenameContextUnknownSource(t *testing.T) {
	dir := writeFixture(t)
	a, _, _ := newTestApp(dir, nil)
	assert.NotEqual(t, 0, a.run([]string{"config", "rename-context", "nosuch", "x"}))
}

func TestViewGlobalSource(t *testing.T) {
	dir := writeFixture(t)
	mustWrite(t, filepath.Join(dir, "application_default_credentials.json"), "{}")
	a, stdout, _ := newTestApp(dir, nil)
	require.Equal(t, 0, a.run([]string{"config", "view"}))
	for _, want := range []string{"a", "a@example.com", "proj-a", "global", "well-known", dir} {
		assert.Contains(t, stdout.String(), want)
	}
}

func TestViewEnvSource(t *testing.T) {
	dir := writeFixture(t)
	a, stdout, _ := newTestApp(dir, map[string]string{"CLOUDSDK_ACTIVE_CONFIG_NAME": "b"})
	require.Equal(t, 0, a.run([]string{"config", "view"}))
	for _, want := range []string{"b@example.com", "proj-b", "CLOUDSDK_ACTIVE_CONFIG_NAME", filepath.Join(dir, "adc-b.json")} {
		assert.Contains(t, stdout.String(), want)
	}
}

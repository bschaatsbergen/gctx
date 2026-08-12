package main

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeGcloud records invocations; handler scripts per-call behavior.
type fakeGcloud struct {
	calls   []gcloudCall
	handler func(configName string, args ...string) error
}

type gcloudCall struct {
	configName string
	args       []string
}

func (f *fakeGcloud) run(configName string, args ...string) error {
	f.calls = append(f.calls, gcloudCall{configName, args})
	if f.handler != nil {
		return f.handler(configName, args...)
	}
	return nil
}

func isADCLogin(args []string) bool {
	return slices.Equal(args, []string{"auth", "application-default", "login"})
}

func TestLoginExistingContext(t *testing.T) {
	dir := writeFixture(t)
	fake := &fakeGcloud{}
	a, stdout, stderr := newTestApp(dir, nil)
	a.runGcloud = fake.run
	require.Equal(t, 0, a.run([]string{"login", "a"}))

	require.Len(t, fake.calls, 1)
	assert.Equal(t, "a", fake.calls[0].configName)
	assert.Equal(t, []string{"auth", "login", "--no-update-adc"}, fake.calls[0].args)
	assert.Empty(t, stdout.String())
	assert.Contains(t, stderr.String(), "gctx adc login a")
	assert.Contains(t, stderr.String(), "gctx a")
}

func TestLoginCreatesMissingContextAndSetsProject(t *testing.T) {
	dir := writeFixture(t)
	fake := &fakeGcloud{}
	a, _, _ := newTestApp(dir, nil)
	a.runGcloud = fake.run
	require.Equal(t, 0, a.run([]string{"login", "c", "--project", "proj-c"}))

	ini := parseINI(mustRead(t, filepath.Join(dir, "configurations", "config_c")))
	assert.Equal(t, "proj-c", ini["core"]["project"])
	assert.Equal(t, "a", strings.TrimSpace(mustRead(t, filepath.Join(dir, "active_config"))))
	require.Len(t, fake.calls, 1)
	assert.Equal(t, []string{"auth", "login", "--no-update-adc"}, fake.calls[0].args)
}

func TestLoginGcloudFailure(t *testing.T) {
	dir := writeFixture(t)
	fake := &fakeGcloud{handler: func(string, ...string) error { return errors.New("auth failed") }}
	a, _, stderr := newTestApp(dir, nil)
	a.runGcloud = fake.run
	require.NotEqual(t, 0, a.run([]string{"login", "a"}))
	assert.Contains(t, stderr.String(), "auth failed")
}

func TestADCLoginGlobalContextSavesWellKnownAndLinks(t *testing.T) {
	dir := writeFixture(t)
	wk := filepath.Join(dir, "application_default_credentials.json")
	mustWrite(t, wk, "OLD-IDENTITY")
	fake := &fakeGcloud{handler: func(configName string, args ...string) error {
		if isADCLogin(args) {
			mustWriteRaw(wk, "NEW-"+configName)
		}
		return nil
	}}
	a, stdout, _ := newTestApp(dir, nil)
	a.runGcloud = fake.run
	require.Equal(t, 0, a.run([]string{"adc", "login", "a"}))

	assert.Equal(t, "NEW-a", mustRead(t, filepath.Join(dir, "adc-a.json")))
	requireLink(t, wk, "adc-a.json")
	assert.Equal(t, "NEW-a", mustRead(t, wk))
	assert.Equal(t, "OLD-IDENTITY", mustRead(t, wk+".gctx-saved"))
	assert.False(t, exists(wk+".gctx-backup"))
	require.Len(t, fake.calls, 2)
	assert.Equal(t, "a", fake.calls[1].configName)
	assert.Equal(t, []string{"auth", "application-default", "set-quota-project", "proj-a"}, fake.calls[1].args)
	assert.Empty(t, stdout.String())
}

func TestADCLoginNonGlobalRestoresWellKnown(t *testing.T) {
	dir := writeFixture(t)
	wk := filepath.Join(dir, "application_default_credentials.json")
	mustWrite(t, wk, "OLD-IDENTITY")
	fake := &fakeGcloud{handler: func(configName string, args ...string) error {
		if isADCLogin(args) {
			mustWriteRaw(wk, "NEW-"+configName)
		}
		return nil
	}}
	a, _, _ := newTestApp(dir, nil)
	a.runGcloud = fake.run
	require.Equal(t, 0, a.run([]string{"adc", "login", "b"}))

	assert.Equal(t, "NEW-b", mustRead(t, filepath.Join(dir, "adc-b.json")))
	assert.False(t, isSymlink(wk))
	assert.Equal(t, "OLD-IDENTITY", mustRead(t, wk))
	assert.False(t, exists(wk+".gctx-backup"))
	assert.False(t, exists(wk+".gctx-saved"))
}

func TestADCLoginSetsAsideDanglingLink(t *testing.T) {
	dir := writeFixture(t)
	wk := filepath.Join(dir, "application_default_credentials.json")
	require.NoError(t, os.Symlink("adc-x.json", wk))
	fake := &fakeGcloud{handler: func(configName string, args ...string) error {
		if isADCLogin(args) {
			mustWriteRaw(wk, "NEW-"+configName)
		}
		return nil
	}}
	a, _, _ := newTestApp(dir, nil)
	a.runGcloud = fake.run
	require.Equal(t, 0, a.run([]string{"adc", "login", "b"}))

	assert.Equal(t, "NEW-b", mustRead(t, filepath.Join(dir, "adc-b.json")))
	assert.False(t, exists(filepath.Join(dir, "adc-x.json")),
		"gcloud must never write through a dangling well-known link")
	requireLink(t, wk, "adc-x.json")
}

func TestADCLoginRestoresWellKnownOnFailure(t *testing.T) {
	dir := writeFixture(t)
	wk := filepath.Join(dir, "application_default_credentials.json")
	mustWrite(t, wk, "OLD-IDENTITY")
	fake := &fakeGcloud{handler: func(string, ...string) error { return errors.New("cancelled") }}
	a, _, _ := newTestApp(dir, nil)
	a.runGcloud = fake.run
	require.NotEqual(t, 0, a.run([]string{"adc", "login", "a"}))

	assert.Equal(t, "OLD-IDENTITY", mustRead(t, wk))
	assert.False(t, exists(filepath.Join(dir, "adc-a.json")))
	assert.False(t, exists(wk+".gctx-backup"))
}

func TestADCLoginWithoutPreexistingWellKnown(t *testing.T) {
	dir := writeFixture(t)
	wk := filepath.Join(dir, "application_default_credentials.json")
	fake := &fakeGcloud{handler: func(configName string, args ...string) error {
		if isADCLogin(args) {
			mustWriteRaw(wk, "NEW-"+configName)
		}
		return nil
	}}
	a, _, _ := newTestApp(dir, nil)
	a.runGcloud = fake.run
	require.Equal(t, 0, a.run([]string{"adc", "login", "a"}))

	assert.Equal(t, "NEW-a", mustRead(t, filepath.Join(dir, "adc-a.json")))
	requireLink(t, wk, "adc-a.json")
}

func TestADCLoginQuotaProjectFailureIsWarning(t *testing.T) {
	dir := writeFixture(t)
	wk := filepath.Join(dir, "application_default_credentials.json")
	fake := &fakeGcloud{handler: func(configName string, args ...string) error {
		if isADCLogin(args) {
			mustWriteRaw(wk, "NEW")
			return nil
		}
		return errors.New("no permission on quota project")
	}}
	a, _, stderr := newTestApp(dir, nil)
	a.runGcloud = fake.run
	require.Equal(t, 0, a.run([]string{"adc", "login", "a"}))

	assert.Equal(t, "NEW", mustRead(t, filepath.Join(dir, "adc-a.json")))
	assert.Contains(t, stderr.String(), "warning")
}

func TestADCLoginContextWithoutProjectSkipsQuota(t *testing.T) {
	dir := writeFixture(t)
	mustWrite(t, filepath.Join(dir, "configurations", "config_d"), "")
	wk := filepath.Join(dir, "application_default_credentials.json")
	fake := &fakeGcloud{handler: func(configName string, args ...string) error {
		if isADCLogin(args) {
			mustWriteRaw(wk, "NEW")
		}
		return nil
	}}
	a, _, _ := newTestApp(dir, nil)
	a.runGcloud = fake.run
	require.Equal(t, 0, a.run([]string{"adc", "login", "d"}))
	assert.Len(t, fake.calls, 1)
}

func TestADCLoginUnknownContext(t *testing.T) {
	dir := writeFixture(t)
	fake := &fakeGcloud{}
	a, _, stderr := newTestApp(dir, nil)
	a.runGcloud = fake.run
	require.NotEqual(t, 0, a.run([]string{"adc", "login", "nosuch"}))
	assert.Empty(t, fake.calls)
	assert.Contains(t, stderr.String(), "nosuch")
}

func TestADCCaptureMovesWellKnownAndLinksGlobal(t *testing.T) {
	dir := writeFixture(t)
	wk := filepath.Join(dir, "application_default_credentials.json")
	mustWrite(t, wk, "ADOPT-ME")
	a, _, _ := newTestApp(dir, nil)
	require.Equal(t, 0, a.run([]string{"adc", "capture", "a"}))
	assert.Equal(t, "ADOPT-ME", mustRead(t, filepath.Join(dir, "adc-a.json")))
	requireLink(t, wk, "adc-a.json")
}

func TestADCCaptureNonGlobalLeavesNoWellKnown(t *testing.T) {
	dir := writeFixture(t)
	wk := filepath.Join(dir, "application_default_credentials.json")
	mustWrite(t, wk, "ADOPT-ME")
	a, _, _ := newTestApp(dir, nil)
	require.Equal(t, 0, a.run([]string{"adc", "capture", "b"}))
	assert.Equal(t, "ADOPT-ME", mustRead(t, filepath.Join(dir, "adc-b.json")))
	assert.False(t, exists(wk), "well-known file must be moved, not copied")
	assert.False(t, isSymlink(wk))
}

func TestADCCaptureRefusesManagedLink(t *testing.T) {
	dir := writeFixture(t)
	wk := filepath.Join(dir, "application_default_credentials.json")
	require.NoError(t, os.Symlink("adc-b.json", wk))
	a, _, stderr := newTestApp(dir, nil)
	require.NotEqual(t, 0, a.run([]string{"adc", "capture", "a"}))
	assert.False(t, exists(filepath.Join(dir, "adc-a.json")))
	requireLink(t, wk, "adc-b.json")
	assert.Contains(t, stderr.String(), "link")
}

func TestADCCaptureWithoutWellKnown(t *testing.T) {
	dir := writeFixture(t)
	a, _, stderr := newTestApp(dir, nil)
	require.NotEqual(t, 0, a.run([]string{"adc", "capture", "a"}))
	assert.NotEmpty(t, stderr.String())
}

func TestADCCaptureUnknownContext(t *testing.T) {
	dir := writeFixture(t)
	mustWrite(t, filepath.Join(dir, "application_default_credentials.json"), "X")
	a, _, _ := newTestApp(dir, nil)
	require.NotEqual(t, 0, a.run([]string{"adc", "capture", "nosuch"}))
	assert.True(t, exists(filepath.Join(dir, "application_default_credentials.json")))
}

func TestADCList(t *testing.T) {
	dir := writeFixture(t)
	a, stdout, _ := newTestApp(dir, nil)
	require.Equal(t, 0, a.run([]string{"adc", "list"}))
	assert.Contains(t, stdout.String(), "b")
	assert.Contains(t, stdout.String(), filepath.Join(dir, "adc-b.json"))
	assert.NotContains(t, stdout.String(), "adc-a.json")
}

package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// gctxBin is the freshly built binary used by wrapper integration tests.
var gctxBin string

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "gctx-test-bin")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	gctxBin = filepath.Join(dir, "gctx")
	cmd := exec.Command("go", "build", "-o", gctxBin, ".")
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "building gctx for integration tests:", err)
		os.Exit(1)
	}
	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}

func commandOutput(t *testing.T, args ...string) string {
	t.Helper()
	a, stdout, stderr := newTestApp(t.TempDir(), nil)
	require.Equal(t, 0, a.run(args), "gctx %v: %s", args, stderr.String())
	return stdout.String()
}

func checkSyntax(t *testing.T, shell string, preArgs []string, text string) {
	t.Helper()
	if _, err := exec.LookPath(shell); err != nil {
		t.Skipf("%s not installed", shell)
	}
	f := filepath.Join(t.TempDir(), "snippet")
	require.NoError(t, os.WriteFile(f, []byte(text), 0o600))
	cmd := exec.Command(shell, append(preArgs, f)...)
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "%s parse failed:\n%s\n--- snippet ---\n%s", shell, out, text)
}

func checkNuSyntax(t *testing.T, text string) {
	t.Helper()
	if _, err := exec.LookPath("nu"); err != nil {
		t.Skip("nu not installed")
	}
	f := filepath.Join(t.TempDir(), "snippet.nu")
	require.NoError(t, os.WriteFile(f, []byte(text), 0o600))
	cmd := exec.Command("nu", "--no-config-file", "-c",
		fmt.Sprintf("if (nu-check --debug '%s') { exit 0 } else { exit 1 }", f))
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "nu-check failed:\n%s\n--- snippet ---\n%s", out, text)
}

func TestInitPosixSyntax(t *testing.T) {
	text := commandOutput(t, "init", "posix")
	checkSyntax(t, "bash", []string{"-n"}, text)
	checkSyntax(t, "zsh", []string{"-f", "-n"}, text)
}

func TestInitBashZshAreAliasesForPosix(t *testing.T) {
	posix := commandOutput(t, "init", "posix")
	for _, alias := range []string{"bash", "zsh"} {
		assert.Equal(t, posix, commandOutput(t, "init", alias))
	}
}

func TestInitFishSyntax(t *testing.T) {
	checkSyntax(t, "fish", []string{"--no-execute"}, commandOutput(t, "init", "fish"))
}

func TestInitNuSyntax(t *testing.T) {
	checkNuSyntax(t, commandOutput(t, "init", "nu"))
}

func TestInitPowershellParses(t *testing.T) {
	text := commandOutput(t, "init", "powershell")
	if _, err := exec.LookPath("pwsh"); err != nil {
		t.Skip("pwsh not installed")
	}
	f := filepath.Join(t.TempDir(), "snippet.ps1")
	require.NoError(t, os.WriteFile(f, []byte(text), 0o600))
	cmd := exec.Command("pwsh", "-NoProfile", "-Command",
		fmt.Sprintf("$null = [scriptblock]::Create((Get-Content -Raw '%s'))", f))
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "pwsh parse failed:\n%s", out)
}

func TestInitUnknownShell(t *testing.T) {
	a, stdout, _ := newTestApp(t.TempDir(), nil)
	require.NotEqual(t, 0, a.run([]string{"init", "tcsh"}))
	assert.Empty(t, stdout.String())
}

func TestCompletionSyntax(t *testing.T) {
	t.Run("bash", func(t *testing.T) {
		checkSyntax(t, "bash", []string{"-n"}, commandOutput(t, "completion", "bash"))
	})
	t.Run("zsh", func(t *testing.T) {
		checkSyntax(t, "zsh", []string{"-f", "-n"}, commandOutput(t, "completion", "zsh"))
	})
	t.Run("fish", func(t *testing.T) {
		checkSyntax(t, "fish", []string{"--no-execute"}, commandOutput(t, "completion", "fish"))
	})
	t.Run("nu", func(t *testing.T) {
		checkNuSyntax(t, commandOutput(t, "completion", "nu"))
	})
}

const posixDance = `set -u
eval "$(command gctx init posix)"

gctx b >/dev/null
[ "$CLOUDSDK_ACTIVE_CONFIG_NAME" = "b" ] || { echo "fail: switch name"; exit 1; }
[ "$GOOGLE_APPLICATION_CREDENTIALS" = "$CLOUDSDK_CONFIG/adc-b.json" ] || { echo "fail: adc bound"; exit 1; }
[ "$GCTX_PREV" = "a" ] || { echo "fail: prev captured"; exit 1; }

if gctx nosuch >/dev/null 2>&1; then echo "fail: unknown context accepted"; exit 1; fi
[ "$CLOUDSDK_ACTIVE_CONFIG_NAME" = "b" ] || { echo "fail: env changed on failed switch"; exit 1; }
[ "$GCTX_PREV" = "a" ] || { echo "fail: prev changed on failed switch"; exit 1; }

gctx a >/dev/null
[ "$CLOUDSDK_ACTIVE_CONFIG_NAME" = "a" ] || { echo "fail: switch back"; exit 1; }
[ -z "${GOOGLE_APPLICATION_CREDENTIALS:-}" ] || { echo "fail: adc not unset"; exit 1; }
[ "$GCTX_PREV" = "b" ] || { echo "fail: prev after second switch"; exit 1; }

gctx - >/dev/null
[ "$CLOUDSDK_ACTIVE_CONFIG_NAME" = "b" ] || { echo "fail: dash toggle"; exit 1; }
[ "$GCTX_PREV" = "a" ] || { echo "fail: prev after toggle"; exit 1; }

gctx reset >/dev/null
[ -z "${CLOUDSDK_ACTIVE_CONFIG_NAME:-}" ] || { echo "fail: reset name"; exit 1; }
[ -z "${GCTX_PREV:-}" ] || { echo "fail: reset prev"; exit 1; }

if gctx - >/dev/null 2>&1; then echo "fail: dash with no prev accepted"; exit 1; fi
echo OK
`

const fishDance = `command gctx init fish | source

gctx b >/dev/null
test "$CLOUDSDK_ACTIVE_CONFIG_NAME" = b; or begin; echo "fail: switch name"; exit 1; end
test "$GOOGLE_APPLICATION_CREDENTIALS" = "$CLOUDSDK_CONFIG/adc-b.json"; or begin; echo "fail: adc bound"; exit 1; end
test "$GCTX_PREV" = a; or begin; echo "fail: prev captured"; exit 1; end

if gctx nosuch >/dev/null 2>/dev/null; echo "fail: unknown context accepted"; exit 1; end
test "$CLOUDSDK_ACTIVE_CONFIG_NAME" = b; or begin; echo "fail: env changed on failed switch"; exit 1; end
test "$GCTX_PREV" = a; or begin; echo "fail: prev changed on failed switch"; exit 1; end

gctx a >/dev/null
test "$CLOUDSDK_ACTIVE_CONFIG_NAME" = a; or begin; echo "fail: switch back"; exit 1; end
if set -q GOOGLE_APPLICATION_CREDENTIALS; echo "fail: adc not unset"; exit 1; end
test "$GCTX_PREV" = b; or begin; echo "fail: prev after second switch"; exit 1; end

gctx - >/dev/null
test "$CLOUDSDK_ACTIVE_CONFIG_NAME" = b; or begin; echo "fail: dash toggle"; exit 1; end

gctx reset >/dev/null
if set -q CLOUDSDK_ACTIVE_CONFIG_NAME; echo "fail: reset name"; exit 1; end
if set -q GCTX_PREV; echo "fail: reset prev"; exit 1; end

if gctx - >/dev/null 2>/dev/null; echo "fail: dash with no prev accepted"; exit 1; end
echo OK
`

const nuDance = `source "@INIT@"

gctx b
if ($env.CLOUDSDK_ACTIVE_CONFIG_NAME? != "b") { print -e "fail: switch name"; exit 1 }
if ($env.GOOGLE_APPLICATION_CREDENTIALS? != "@ADCB@") { print -e "fail: adc bound"; exit 1 }
if ($env.GCTX_PREV? != "a") { print -e "fail: prev captured"; exit 1 }

let failed = (try { gctx nosuch; false } catch { true })
if not $failed { print -e "fail: unknown context accepted"; exit 1 }
if ($env.CLOUDSDK_ACTIVE_CONFIG_NAME? != "b") { print -e "fail: env changed on failed switch"; exit 1 }
if ($env.GCTX_PREV? != "a") { print -e "fail: prev changed on failed switch"; exit 1 }

gctx a
if ($env.CLOUDSDK_ACTIVE_CONFIG_NAME? != "a") { print -e "fail: switch back"; exit 1 }
if ($env.GOOGLE_APPLICATION_CREDENTIALS? != null) { print -e "fail: adc not hidden"; exit 1 }
if ($env.GCTX_PREV? != "b") { print -e "fail: prev after second switch"; exit 1 }

gctx -
if ($env.CLOUDSDK_ACTIVE_CONFIG_NAME? != "b") { print -e "fail: dash toggle"; exit 1 }
if ($env.GCTX_PREV? != "a") { print -e "fail: prev after toggle"; exit 1 }

gctx reset
if ($env.CLOUDSDK_ACTIVE_CONFIG_NAME? != null) { print -e "fail: reset name"; exit 1 }
if ($env.GCTX_PREV? != null) { print -e "fail: reset prev"; exit 1 }

let failed2 = (try { gctx -; false } catch { true })
if not $failed2 { print -e "fail: dash with no prev accepted"; exit 1 }
print "OK"
`

func runDance(t *testing.T, shell string, preArgs []string, script string) {
	t.Helper()
	if _, err := exec.LookPath(shell); err != nil {
		t.Skipf("%s not installed", shell)
	}
	dir := writeFixture(t)
	f := filepath.Join(t.TempDir(), "dance")
	require.NoError(t, os.WriteFile(f, []byte(script), 0o600))
	cmd := exec.Command(shell, append(preArgs, f)...)
	cmd.Env = []string{
		"PATH=" + filepath.Dir(gctxBin) + string(os.PathListSeparator) + os.Getenv("PATH"),
		"HOME=" + t.TempDir(),
		"CLOUDSDK_CONFIG=" + dir,
		"TERM=dumb",
	}
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "%s wrapper dance:\n%s", shell, out)
	assert.Contains(t, string(out), "OK")
}

func TestBashWrapperIntegration(t *testing.T) {
	runDance(t, "bash", nil, posixDance)
}

func TestZshWrapperIntegration(t *testing.T) {
	runDance(t, "zsh", []string{"-f"}, posixDance)
}

func TestFishWrapperIntegration(t *testing.T) {
	runDance(t, "fish", nil, fishDance)
}

func TestNuWrapperIntegration(t *testing.T) {
	if _, err := exec.LookPath("nu"); err != nil {
		t.Skip("nu not installed")
	}
	initFile := filepath.Join(t.TempDir(), "gctx-init.nu")
	require.NoError(t, os.WriteFile(initFile, []byte(commandOutput(t, "init", "nu")), 0o600))
	dir := writeFixture(t)
	script := strings.ReplaceAll(nuDance, "@INIT@", initFile)
	script = strings.ReplaceAll(script, "@ADCB@", filepath.Join(dir, "adc-b.json"))
	f := filepath.Join(t.TempDir(), "dance.nu")
	require.NoError(t, os.WriteFile(f, []byte(script), 0o600))
	cmd := exec.Command("nu", "--no-config-file", f)
	cmd.Env = []string{
		"PATH=" + filepath.Dir(gctxBin) + string(os.PathListSeparator) + os.Getenv("PATH"),
		"HOME=" + t.TempDir(),
		"CLOUDSDK_CONFIG=" + dir,
		"TERM=dumb",
	}
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "nu wrapper dance:\n%s", out)
	assert.Contains(t, string(out), "OK")
}

func TestBareBinaryContextNameHint(t *testing.T) {
	dir := writeFixture(t)
	a, stdout, stderr := newTestApp(dir, nil)
	require.NotEqual(t, 0, a.run([]string{"b"}))
	assert.Empty(t, stdout.String())
	assert.Contains(t, stderr.String(), "gctx init")
}

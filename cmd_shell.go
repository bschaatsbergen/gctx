package main

import "io"

// runInit prints the wrapper for a shell. Wrappers only apply what
// `gctx env` emits; all logic lives in the binary.
func (a *app) runInit(args []string) int {
	if len(args) != 1 {
		return a.errf("usage: gctx init <posix|bash|zsh|fish|powershell|nu>")
	}
	dialect, ok := normalizeShell(args[0])
	if !ok {
		return a.errf("unknown shell %q (want posix, bash, zsh, fish, powershell or nu)", args[0])
	}
	switch dialect {
	case "posix":
		io.WriteString(a.stdout, posixInit)
	case "fish":
		io.WriteString(a.stdout, fishInit)
	case "powershell":
		io.WriteString(a.stdout, powershellInit)
	case "nu":
		io.WriteString(a.stdout, nuInit)
	}
	return 0
}

func (a *app) runCompletion(args []string) int {
	if len(args) != 1 {
		return a.errf("usage: gctx completion <bash|zsh|fish|powershell|nu>")
	}
	switch args[0] {
	case "bash":
		io.WriteString(a.stdout, bashCompletion)
	case "zsh":
		io.WriteString(a.stdout, zshCompletion)
	case "fish":
		io.WriteString(a.stdout, fishCompletion)
	case "powershell", "pwsh":
		io.WriteString(a.stdout, powershellCompletion)
	case "nu", "nushell":
		io.WriteString(a.stdout, nuCompletion)
	default:
		return a.errf("unknown shell %q (want bash, zsh, fish, powershell or nu)", args[0])
	}
	return 0
}

const posixInit = `# gctx shell integration (bash/zsh).
# Install: add to your shell rc:
#   eval "$(gctx init bash)"   # or: gctx init zsh

__gctx_switch() {
    __gctx_prev="$(command gctx config current-context 2>/dev/null)"
    if ! __gctx_env="$(command gctx env "$1" --shell posix)"; then
        unset __gctx_prev __gctx_env
        return 1
    fi
    eval "$__gctx_env"
    if [ -n "${__gctx_prev:-}" ]; then
        GCTX_PREV="$__gctx_prev"
        export GCTX_PREV
    fi
    unset __gctx_prev __gctx_env
    printf 'Switched to context "%s".\n' "$1"
}

gctx() {
    if [ "$#" -eq 0 ]; then
        command gctx config get-contexts
        return
    fi
    case "$1" in
        config|login|adc|env|init|completion|version|help)
            command gctx "$@"
            ;;
        -)
            if [ -z "${GCTX_PREV:-}" ]; then
                echo "gctx: no previous context" >&2
                return 1
            fi
            __gctx_switch "$GCTX_PREV"
            ;;
        reset)
            if __gctx_env="$(command gctx env --reset --shell posix)"; then
                eval "$__gctx_env"
                unset __gctx_env GCTX_PREV
                printf 'Reset to the global context.\n'
            else
                unset __gctx_env
                return 1
            fi
            ;;
        -*)
            command gctx "$@"
            ;;
        *)
            __gctx_switch "$1"
            ;;
    esac
}
`

const fishInit = `# gctx shell integration (fish).
# Install: add to ~/.config/fish/config.fish:
#   gctx init fish | source

function __gctx_switch
    set -l prev (command gctx config current-context 2>/dev/null)
    command gctx env $argv[1] --shell fish | source
    if test $pipestatus[1] -ne 0
        return 1
    end
    if test -n "$prev"
        set -gx GCTX_PREV $prev
    end
    echo "Switched to context \"$argv[1]\"."
end

function gctx
    if test (count $argv) -eq 0
        command gctx config get-contexts
        return
    end
    switch $argv[1]
        case config login adc env init completion version help
            command gctx $argv
        case '-'
            if not set -q GCTX_PREV
                echo "gctx: no previous context" >&2
                return 1
            end
            __gctx_switch $GCTX_PREV
        case reset
            command gctx env --reset --shell fish | source
            if test $pipestatus[1] -ne 0
                return 1
            end
            set -e GCTX_PREV
            echo "Reset to the global context."
        case '-*'
            command gctx $argv
        case '*'
            __gctx_switch $argv[1]
    end
end
`

const powershellInit = `# gctx shell integration (PowerShell).
# Install: add to your $PROFILE:
#   gctx init powershell | Out-String | Invoke-Expression

function __GctxBin {
    $app = Get-Command -Name gctx -CommandType Application -ErrorAction SilentlyContinue | Select-Object -First 1
    if (-not $app) {
        [Console]::Error.WriteLine('gctx: cannot find the gctx binary on PATH')
        return $null
    }
    $app.Source
}

function __GctxApply([object]$Lines) {
    $text = ($Lines | Out-String)
    if ($text.Trim()) { Invoke-Expression $text }
}

function __GctxSwitch([string]$Name) {
    $bin = __GctxBin
    if (-not $bin) { return }
    $prev = & $bin config current-context 2>$null
    $out = & $bin env $Name --shell powershell
    if ($LASTEXITCODE -ne 0) { return }
    __GctxApply $out
    if ($prev) { $env:GCTX_PREV = "$prev" }
    Write-Host "Switched to context ""$Name""."
}

function gctx {
    $bin = __GctxBin
    if (-not $bin) { return }
    if ($args.Count -eq 0) {
        & $bin config get-contexts
        return
    }
    $first = "$($args[0])"
    switch ($first) {
        { $_ -in 'config','login','adc','env','init','completion','version','help' } {
            & $bin @args
            return
        }
        '-' {
            if (-not $env:GCTX_PREV) {
                [Console]::Error.WriteLine('gctx: no previous context')
                return
            }
            __GctxSwitch $env:GCTX_PREV
            return
        }
        'reset' {
            $out = & $bin env --reset --shell powershell
            if ($LASTEXITCODE -ne 0) { return }
            __GctxApply $out
            Remove-Item Env:GCTX_PREV -ErrorAction Ignore
            Write-Host 'Reset to the global context.'
            return
        }
        default {
            if ($first.StartsWith('-')) {
                & $bin @args
                return
            }
            __GctxSwitch $first
            return
        }
    }
}
`

const nuInit = `# gctx shell integration (nushell).
# Install (nushell cannot eval command output; save once and source):
#   ^gctx init nu | save -f ($nu.default-config-dir | path join gctx-init.nu)
# then add to config.nu:
#   source gctx-init.nu

def "nu-complete gctx" [] {
    (do { ^gctx config get-contexts -o name } | complete | get stdout | lines)
    | append [config login adc env init completion version help reset -]
}

def --env __gctx_apply [envjson: string] {
    let m = ($envjson | from json)
    if ($m.CLOUDSDK_ACTIVE_CONFIG_NAME? == null) {
        hide-env --ignore-errors CLOUDSDK_ACTIVE_CONFIG_NAME
    } else {
        load-env {CLOUDSDK_ACTIVE_CONFIG_NAME: $m.CLOUDSDK_ACTIVE_CONFIG_NAME}
    }
    if ($m.GOOGLE_APPLICATION_CREDENTIALS? == null) {
        hide-env --ignore-errors GOOGLE_APPLICATION_CREDENTIALS
    } else {
        load-env {GOOGLE_APPLICATION_CREDENTIALS: $m.GOOGLE_APPLICATION_CREDENTIALS}
    }
}

def --env __gctx_switch [name: string] {
    let prev = (do { ^gctx config current-context } | complete | get stdout | str trim)
    let res = (do { ^gctx env $name --shell nu } | complete)
    if ($res.stderr | str trim | is-not-empty) {
        print -e ($res.stderr | str trim)
    }
    if $res.exit_code != 0 {
        error make --unspanned {msg: $"gctx: could not switch to '($name)'"}
    }
    __gctx_apply $res.stdout
    if ($prev | is-not-empty) {
        load-env {GCTX_PREV: $prev}
    }
    print $"Switched to context \"($name)\"."
}

def --env gctx [...args: string@"nu-complete gctx"] {
    if ($args | is-empty) {
        ^gctx config get-contexts
        return
    }
    let cmd = $args.0
    if $cmd in [config login adc env init completion version help] {
        ^gctx ...$args
    } else if $cmd == '-' {
        if ($env.GCTX_PREV? == null) {
            error make --unspanned {msg: "gctx: no previous context"}
        }
        __gctx_switch $env.GCTX_PREV
    } else if $cmd == 'reset' {
        let res = (do { ^gctx env --reset --shell nu } | complete)
        if $res.exit_code != 0 {
            print -e ($res.stderr | str trim)
            error make --unspanned {msg: "gctx: reset failed"}
        }
        # Hide before the nested call: hides made after a nested def --env
        # call do not propagate.
        hide-env --ignore-errors GCTX_PREV
        __gctx_apply $res.stdout
        print "Reset to the global context."
    } else if ($cmd | str starts-with '-') {
        ^gctx ...$args
    } else {
        __gctx_switch $cmd
    }
}
`

const bashCompletion = `# gctx bash completion.
# Install: add to ~/.bashrc:
#   source <(gctx completion bash)

_gctx_complete() {
    local cur prev ctxs
    cur="${COMP_WORDS[COMP_CWORD]}"
    prev="${COMP_WORDS[COMP_CWORD-1]}"
    ctxs="$(command gctx config get-contexts -o name 2>/dev/null)"
    if [ "$COMP_CWORD" -eq 1 ]; then
        COMPREPLY=($(compgen -W "config login adc env init completion version help reset $ctxs" -- "$cur"))
        return
    fi
    case "$prev" in
        env|login|use-context|delete-context|rename-context|set-context|capture)
            COMPREPLY=($(compgen -W "$ctxs" -- "$cur"))
            ;;
        config)
            COMPREPLY=($(compgen -W "get-contexts current-context use-context set-context delete-context rename-context view" -- "$cur"))
            ;;
        adc)
            COMPREPLY=($(compgen -W "login capture list" -- "$cur"))
            ;;
        init|completion)
            COMPREPLY=($(compgen -W "posix bash zsh fish powershell nu" -- "$cur"))
            ;;
        --shell)
            COMPREPLY=($(compgen -W "posix fish powershell nu" -- "$cur"))
            ;;
    esac
}
complete -F _gctx_complete gctx
`

const zshCompletion = `# gctx zsh completion.
# Install: add to ~/.zshrc (after compinit):
#   source <(gctx completion zsh)

_gctx() {
    local -a ctxs subs
    subs=(config login adc env init completion version help reset)
    ctxs=(${(f)"$(command gctx config get-contexts -o name 2>/dev/null)"})
    if (( CURRENT == 2 )); then
        _describe -t commands 'gctx command' subs
        (( ${#ctxs} )) && _describe -t contexts 'context' ctxs
        return
    fi
    case "${words[2]}" in
        env|login)
            (( ${#ctxs} )) && _describe -t contexts 'context' ctxs
            ;;
        config)
            local -a cfg
            cfg=(get-contexts current-context use-context set-context delete-context rename-context view)
            if (( CURRENT == 3 )); then
                _describe -t commands 'config command' cfg
            else
                (( ${#ctxs} )) && _describe -t contexts 'context' ctxs
            fi
            ;;
        adc)
            local -a sub
            sub=(login capture list)
            if (( CURRENT == 3 )); then
                _describe -t commands 'adc command' sub
            else
                (( ${#ctxs} )) && _describe -t contexts 'context' ctxs
            fi
            ;;
        init|completion)
            local -a shells
            shells=(posix bash zsh fish powershell nu)
            _describe -t shells 'shell' shells
            ;;
    esac
}
compdef _gctx gctx
`

const fishCompletion = `# gctx fish completion.
# Install: add to ~/.config/fish/config.fish:
#   gctx completion fish | source

complete -c gctx -f
complete -c gctx -n __fish_use_subcommand -a 'config login adc env init completion version help reset'
complete -c gctx -n __fish_use_subcommand -a '(command gctx config get-contexts -o name 2>/dev/null)' -d context
complete -c gctx -n '__fish_seen_subcommand_from env login' -a '(command gctx config get-contexts -o name 2>/dev/null)' -d context
complete -c gctx -n '__fish_seen_subcommand_from config' -a 'get-contexts current-context use-context set-context delete-context rename-context view'
complete -c gctx -n '__fish_seen_subcommand_from use-context set-context delete-context rename-context capture' -a '(command gctx config get-contexts -o name 2>/dev/null)' -d context
complete -c gctx -n '__fish_seen_subcommand_from adc' -a 'login capture list'
complete -c gctx -n '__fish_seen_subcommand_from init completion' -a 'posix bash zsh fish powershell nu'
`

const powershellCompletion = `# gctx PowerShell completion.
# Install: add to your $PROFILE:
#   gctx completion powershell | Out-String | Invoke-Expression

Register-ArgumentCompleter -Native -CommandName gctx -ScriptBlock {
    param($wordToComplete, $commandAst, $cursorPosition)
    $subs = @('config','login','adc','env','init','completion','version','help','reset')
    $ctxs = @(& gctx config get-contexts -o name 2>$null)
    ($subs + $ctxs) |
        Where-Object { $_ -like "$wordToComplete*" } |
        ForEach-Object { [System.Management.Automation.CompletionResult]::new($_, $_, 'ParameterValue', $_) }
}
`

const nuCompletion = `# Completions are bundled with the wrapper from 'gctx init nu'.
# Standalone completer:

def "nu-complete gctx" [] {
    (do { ^gctx config get-contexts -o name } | complete | get stdout | lines)
    | append [config login adc env init completion version help reset -]
}
`

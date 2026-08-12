# gctx

Context switcher for gcloud configurations and Application Default
Credentials (ADC). A context is a gcloud [named configuration](https://cloud.google.com/sdk/docs/configurations)
plus an optional captured ADC file. Switching changes who gcloud and your
client libraries act as, per shell.

```console
$ gctx
CURRENT   NAME    ACCOUNT          PROJECT      ADC
*         acme1   you@acme1.com                 ~/.config/gcloud/adc-acme1.json
          acme2   you@acme2.com                 ~/.config/gcloud/adc-acme2.json

$ gctx acme2
Switched to context "acme2".
```

## Install

```sh
go install github.com/bschaatsbergen/gctx@latest
```

Or grab a binary from the [releases page](https://github.com/bschaatsbergen/gctx/releases).

Add the shell wrapper and completion to your rc file (required for switching):

```sh
eval "$(gctx init bash)"        # or: zsh
source <(gctx completion bash)
```

fish, powershell, and nu are also supported; `gctx init <shell>` prints its
own install instructions.

## Setup

Once per org:

```sh
gctx login acme1       # create the context, authenticate you@acme1.com
gctx adc login acme1   # capture ADC for it
```

Repeat for `acme2`, then switch with `gctx <name>` as shown above.

Logging in never switches your shell. Switching stays an explicit step so
that authenticating or re-authenticating an org can not silently change the
identity your current shell is acting as.

## Switching

Switch the current shell. Other shells are unaffected:

```sh
gctx acme2
```

Jump back to the previous context:

```sh
gctx -
```

Switch all shells at once by moving the global default:

```sh
gctx config use-context acme1
```

Drop this shell's override so it follows the global default again:

```sh
gctx reset
```

## Managing contexts

Rename or delete a context:

```sh
gctx config rename-context acme2 acme3
gctx config delete-context acme2
```

Show the active context's resolved settings:

```sh
gctx config view
```

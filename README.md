# gctx

Context switcher for gcloud configurations and Application Default
Credentials (ADC).

gcloud [named configurations](https://cloud.google.com/sdk/docs/configurations)
switch who gcloud acts as, but ADC lives in one shared file: every org
switch means another `gcloud auth application-default login` browser
round-trip. gctx captures ADC once per context and re-points client
libraries when you switch, so changing orgs never repeats an auth flow.

```console
$ gctx
CURRENT   NAME    ACCOUNT          PROJECT      ADC
*         acme1   you@acme1.com                 ~/.config/gcloud/adc-acme1.json
          acme2   you@acme2.com                 ~/.config/gcloud/adc-acme2.json

$ gctx config use-context acme2
Switched global context to "acme2".
```

## Install

```sh
go install github.com/bschaatsbergen/gctx@latest
```

Or grab a binary from the [releases page](https://github.com/bschaatsbergen/gctx/releases).

## Setup

Once per org:

```sh
gctx login acme1       # create the context, authenticate you@acme1.com
gctx adc login acme1   # capture ADC for it
```

Repeat for `acme2`.

Logging in never switches the active context. Switching stays an explicit
step so that authenticating or re-authenticating an org can not silently
change the identity you are acting as.

## Switching

Move the global default; every shell follows:

```sh
gctx config use-context acme2
```

To pin a different org per terminal instead, see
[per-shell switching](#per-shell-switching) below.

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

## Per-shell switching

Pin a context to one terminal, so different orgs run side by side. This
needs a one-line shell hook in your rc file (plus completion):

```sh
eval "$(gctx init bash)"        # or: zsh
source <(gctx completion bash)
```

fish, powershell, and nu are also supported; `gctx init <shell>` prints its
own install instructions.

Switch the current shell. Other shells are unaffected:

```sh
gctx acme2
```

Jump back to the previous context:

```sh
gctx -
```

Drop this shell's override so it follows the global default again:

```sh
gctx reset
```

Per-shell switching also points `GOOGLE_APPLICATION_CREDENTIALS` at the
context's captured ADC file, so client libraries switch with the shell.

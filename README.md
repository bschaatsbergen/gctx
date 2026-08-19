# gctx

Context switcher for gcloud configurations and Application Default
Credentials (ADC).

gcloud [named configurations](https://cloud.google.com/sdk/docs/configurations)
switch who gcloud acts as, but ADC lives in one shared file: every org
switch means another `gcloud auth application-default login` browser
round-trip. gctx captures ADC once per context and re-points client
libraries when you switch, so changing orgs never repeats an auth flow.
Helpful if you work in many different Google Cloud Organizations.

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

This also links the well-known ADC file to the context's captured ADC, so
client libraries and new terminals follow along and pick up renewals
instantly. A foreign file at that path is set aside, never overwritten.

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

Mark a context you would rather not lose to a typo:

```sh
gctx config protect prod
gctx config delete-context prod     # refused
gctx config delete-context prod --force
gctx config unprotect prod
```

A protected context still switches, authenticates and renews ADC as usual. The
mark only applies to `delete-context` and `rename-context`, the two commands
that cannot be undone. It travels with the context when you rename it, and is
removed when the context is.

`gctx config get-contexts` shows a PROTECTED column, and the JSON output carries
a `protected` field.

This guards gctx's own commands. It is not a permission boundary and has no
effect on what gcloud will let you do; IAM is the place for that.

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

## Checking for leftover credentials

gcloud writes its own debug logs under `<config-dir>/logs`, and an auth flow
leaves the resulting tokens in them in clear text. `gcloud auth revoke`
invalidates the credential with Google but does not remove the log, so a refresh
token can stay readable on disk after the account it belonged to is gone.
Refresh tokens remain valid until they are revoked or go unused for months.

```sh
gctx doctor          # list affected files, exit 1 if there are any
gctx doctor --fix    # delete them
```

The report gives file paths and match counts. It never prints the matched text.

gctx does not create these logs and cannot stop gcloud writing them. Deleting
them does not un-issue a token that was already exposed, so rotate anything you
have reason to worry about.

## Security

gctx never reads, parses or transmits credentials. All authentication is
performed by gcloud itself; gctx only moves the ADC files gcloud writes,
maintains a symlink at the well-known ADC path, and points environment
variables at the captured files. The only files it reads are gcloud's
plain text configuration files, which hold an account name and a project id.

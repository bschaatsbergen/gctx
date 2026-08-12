# gctx

Context switcher for gcloud configurations and Application Default
Credentials (ADC). A context is a gcloud [named configuration](https://cloud.google.com/sdk/docs/configurations)
plus an optional captured ADC file. Switching changes who gcloud and your
client libraries act as, per shell.

```console
$ gctx
CURRENT   NAME    ACCOUNT            PROJECT      ADC
*         org1    me@company.com     example      ~/.config/gcloud/adc-org1.json
          org2    me@company.com     example      ~/.config/gcloud/adc-org2.json

$ gctx org1
✓ org1
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

```sh
gctx login work --project work-proj   # create context, authenticate
gctx adc login work                   # capture ADC for it
```

Already have an ADC file? `gctx adc capture work` adopts it.
`gctx adc list` shows captured files.

## Usage

```console
$ gctx                    # list contexts
$ gctx work               # switch this shell
$ gctx -                  # previous context
$ gctx reset              # drop per-shell overrides
$ gctx config use-context work    # switch all shells
```

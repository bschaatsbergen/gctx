# gctx

Context switcher for gcloud configurations and Application Default
Credentials (ADC). A context is a gcloud [named configuration](https://cloud.google.com/sdk/docs/configurations)
plus an optional captured ADC file. Switching changes who gcloud and your
client libraries act as, per shell.

```console
$ gctx
CURRENT   NAME    ACCOUNT         PROJECT      ADC
*         acme    you@acme.com                 ~/.config/gcloud/adc-acme.json
          hooli   you@hooli.com                ~/.config/gcloud/adc-hooli.json

$ gctx hooli
✓ hooli
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

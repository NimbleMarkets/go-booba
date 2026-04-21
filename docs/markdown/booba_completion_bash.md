## booba completion bash

Generate the autocompletion script for bash

### Synopsis

Generate the autocompletion script for the bash shell.

This script depends on the 'bash-completion' package.
If it is not installed already, you can install it via your OS's package manager.

To load completions in your current shell session:

	source <(booba completion bash)

To load completions for every new session, execute once:

#### Linux:

	booba completion bash > /etc/bash_completion.d/booba

#### macOS:

	booba completion bash > $(brew --prefix)/etc/bash_completion.d/booba

You will need to start a new shell for this setup to take effect.


```
booba completion bash
```

### Options

```
  -h, --help              help for bash
      --no-descriptions   disable completion descriptions
```

### SEE ALSO

* [booba completion](booba_completion.md)	 - Generate the autocompletion script for the specified shell


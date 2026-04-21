## booba completion zsh

Generate the autocompletion script for zsh

### Synopsis

Generate the autocompletion script for the zsh shell.

If shell completion is not already enabled in your environment you will need
to enable it.  You can execute the following once:

	echo "autoload -U compinit; compinit" >> ~/.zshrc

To load completions in your current shell session:

	source <(booba completion zsh)

To load completions for every new session, execute once:

#### Linux:

	booba completion zsh > "${fpath[1]}/_booba"

#### macOS:

	booba completion zsh > $(brew --prefix)/share/zsh/site-functions/_booba

You will need to start a new shell for this setup to take effect.


```
booba completion zsh [flags]
```

### Options

```
  -h, --help              help for zsh
      --no-descriptions   disable completion descriptions
```

### SEE ALSO

* [booba completion](booba_completion.md)	 - Generate the autocompletion script for the specified shell


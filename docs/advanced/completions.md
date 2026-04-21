# Shell Completions & Man Pages

Cobra ships completion scripts for **bash, zsh, fish, and PowerShell**, and rpcduel includes a
`man` subcommand that generates `man(1)` pages.

## Quick install

```bash
# Bash (current shell only)
source <(rpcduel completion bash)

# Zsh, persistent
rpcduel completion zsh > "${fpath[1]}/_rpcduel"

# Fish
rpcduel completion fish > ~/.config/fish/completions/rpcduel.fish

# PowerShell
rpcduel completion powershell | Out-String | Invoke-Expression
```

## Bundled artifacts

Pre-built completion scripts and man pages are bundled in **every release archive** under:

```
rpcduel_<version>_<os>_<arch>/
├── rpcduel
├── completions/
│   ├── rpcduel.bash
│   ├── rpcduel.zsh
│   ├── rpcduel.fish
│   └── rpcduel.ps1
├── man/
│   ├── rpcduel.1
│   ├── rpcduel-call.1
│   ├── rpcduel-replay.1
│   └── …
└── README.md
```

## Man pages on the fly

```bash
mkdir -p ~/man
rpcduel man --dir ~/man
export MANPATH="$HOME/man:$MANPATH"
man rpcduel-replay
```

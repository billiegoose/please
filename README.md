# please

A natural language shell command translator. Type English, get CLI commands.

## Usage

**One-shot mode** — translate and run a command:
```
please <english description>
```
```
please list all docker containers including stopped ones
please find files modified in the last hour
please undo my last git commit but keep the changes
```

**TUI mode** — interactive shell with live translation:
```
please
```
Type English at the bottom prompt. The translated command appears above as you type. Hit Enter to run.

## Picker

In one-shot mode, a picker appears after translation:

```
  please copy my ssh key to the clipboard

▶ $ cat ~/.ssh/id_rsa.pub | pbcopy  (copy public key to clipboard)
  $ pbcopy < ~/.ssh/id_rsa.pub  (alternative pbcopy syntax)

↑↓ select · enter run · e edit · esc cancel
```

- `↑↓` — select between options (only shown when the request is genuinely ambiguous)
- `Enter` — run selected command
- `e` — edit the command before running (supports `←→`, `Ctrl+A`/`Ctrl+E`, backspace)
- `Esc` — cancel

## Shell integration

Add to your `.bashrc` or `.zshrc`:

```bash
export PATH="$PATH:/path/to/please"
```

## Requirements

- `ANTHROPIC_API_KEY` environment variable set
- `bash` available at `/bin/bash`

## Building

```
go build
```

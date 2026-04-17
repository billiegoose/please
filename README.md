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
  copy my ssh key to the clipboard

▶ $ cat ~/.ssh/id_rsa.pub | pbcopy  (copy public SSH key to clipboard (macOS))
  $ cat ~/.ssh/id_rsa.pub | xclip -selection clipboard  (copy public SSH key to clipboard (Linux))

↑↓ select · enter run · e edit · esc cancel
```

- `↑↓` — select between options (only shown when different interpretations would produce meaningfully different outcomes)
- `Enter` — run selected command
- `e` — edit the command before running
- `Esc` — cancel

## Shell integration

Add to your `.bashrc` or `.zshrc`:

```bash
export PATH="$PATH:/path/to/please"
```

## Configuration

On first run, `please` will prompt for your Anthropic API key and save it to `~/.config/please/config`. Run `please --setup` at any time to change or rotate the key.

Set `ANTHROPIC_API_KEY` as an environment variable to override the config file (useful for CI/servers).

## Requirements

- `bash` available at `/bin/bash`

## Install

```
go install github.com/billiegoose/please@latest
```

## Building from source

```
go build
```

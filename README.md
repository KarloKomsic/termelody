# Termelody

> [!NOTE]
> The vast majority of the code for Termelody is written by hand. However, I've also gotten help from agents (Opencode, and Pi).
> All the help that I've received from said agents is **thoroughly reviewed, modified, and tested.**
> The architecture, tech-stack, and design choices + tradeoffs are entirely made by myself. Agents only help me write the code faster
> once I've gone over the architecture and design decisions, not do the thinking for me. This code and design is my own, no matter the goods or bads.

A keyboard-driven terminal music player written in Go.

## Status

Early development. Playback, playlist navigation, and seeking work through mpv IPC,
with playback state and position read from the event stream mpv pushes rather
than assumed from the commands that were sent.

## Controls

| Key | Action |
|---|---|
| `j` / `k`, `↓` / `↑` | move selection |
| `←` / `→` | seek back / forward (default 5s) |
| `enter` | play selected track |
| `space` | play / pause |
| `n` / `p`, `>` / `<` | next / previous track |
| `s` | stop |
| `q` | quit |

## Usage

```sh
termelody [-seek duration] [-hidden] [-icons set] [music directory]
```

- `-seek` — how far `←` / `→` jump, e.g. `-seek 30s` (default `5s`)
- `-hidden` — start the folder picker with dot-directories shown
- `-icons` — glyphs before folder and track rows: `unicode` (`🗀` / `♪`;
  standard Unicode, no patched font needed), `nerd` (folder and music glyphs
  from Nerd Fonts, which need a patched terminal font), or `none` (default
  `unicode`)
- With no music directory, a folder picker opens at `$HOME`; only folders are
  listed. Dot-directories are hidden by default; `.` toggles them in the picker.
  Symlinked folders are listed and marked with `@`; broken links and links to
  files are ignored. `s` scans the highlighted folder, `space` the one being
  browsed. Choosing a folder with no supported audio reopens the picker with a
  notice, so another folder can be picked without relaunching.

### Configuration

Termelody reads `termelody/config` from the OS config directory
(`os.UserConfigDir`); on Linux that is `$XDG_CONFIG_HOME/termelody/config`,
usually `~/.config/termelody/config`. The file is optional: without it the
defaults below apply. Command-line flags override values from the file.

The format is one `key = value` per line. `#` starts a comment, either on its
own line or after a value.

```ini
# ~/.config/termelody/config
seek = 5s
icons = unicode
show_hidden = false
volume_step = 1
```

| Key | Default | Meaning |
|---|---|---|
| `seek` | `5s` | how far `←` / `→` jump; uses Go duration syntax such as `10s` or `1m30s` |
| `icons` | `unicode` | row glyphs: `unicode`, `nerd`, or `none` |
| `show_hidden` | `false` | start the picker with dot-directories visible |
| `volume_step` | `1` | percent added or removed by one `+` / `-` press |

Unknown or duplicate keys, and invalid values, are reported with the file and
line at startup rather than silently ignored. `volume_step` is currently
config-file only.

### Picker controls

| Key | Action |
|---|---|
| `j` / `k`, `↓` / `↑` | move selection |
| `enter`, `→`, `l` | open selected folder |
| `h`, `←`, `backspace` | go to parent folder |
| `g` / `G` | top / bottom |
| `s` | scan the highlighted folder |
| `space` | scan the current folder |
| `.` | show / hide dot-directories |
| `q`, `esc` | cancel |

## Goals

- Learn Go through building a real application.
- Create a fast terminal music player.
- Practice concurrency, IPC, and TUI development.

## Stack

- Go
- Bubble Tea
- Lip Gloss
- mpv

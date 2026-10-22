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
termelody [-seek duration] [music directory]
```

- `-seek` — how far `←` / `→` jump, e.g. `-seek 30s` (default `5s`)
- With no music directory, a folder picker opens at `$HOME`; only folders are
  listed. `s` scans the highlighted folder, `space` the one being browsed.

### Picker controls

| Key | Action |
|---|---|
| `j` / `k`, `↓` / `↑` | move selection |
| `enter` | open selected folder |
| `h`, `←`, `backspace` | go to parent folder |
| `g` / `G` | top / bottom |
| `s` | scan the highlighted folder |
| `space` | scan the current folder |
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

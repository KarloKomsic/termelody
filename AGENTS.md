# AGENTS.md

## Build & Verify

```sh
go build ./...
go vet ./...
```

Run both after every change to ensure correctness.

## Project Structure

- `playlist/` — core data model (`Track` struct)
- `scanner/` — discovers audio files by extension
- `metadata/` — reads audio tags via `dhowden/tag`
- `loader/` — orchestrates scanner + metadata
- `ipc/` — mpv IPC client (Unix socket, JSON protocol)
- `player/` — wraps ipc, tracks playback state
- `cmd/termelody/` — entry point

## Dependency Graph

```
cmd/termelody
  → player → ipc
  → loader → scanner → playlist
            → metadata → playlist
```

## External Dependencies

| Package | Purpose |
|---|---|
| `github.com/dhowden/tag` | Audio metadata extraction (ID3, FLAC Vorbis, MP4) |
| `bubbletea` (planned) | TUI framework |
| `lipgloss` (planned) | TUI styling |

## Runtime Dependency

- **mpv** - must be installed and available in `$PATH`
- The `ipc` package communicates with mpv over a Unix socket at `/tmp/termelody-mpv.sock`

## Conventions

- Keep packages small and focused on one thing
- Errors should be returned, not logged or swallowed silently
- Exported functions need a doc comment
- No comments on obvious code (e.g. `// open the file` before `os.Open`)
- Prefer `fmt.Errorf` with `%w` for wrapping errors
- The `music/` directory is for testing only and is gitignored

# Architecture

## Overview

Termelody is divided into independent packages.

Current packages:

- **playlist** — core data model (`Track` struct)
- **scanner** — discovers audio files in a directory
- **metadata** — reads audio tags (ID3, Vorbis, etc.) from files
- **loader** — orchestrates scan + metadata enrichment
- **ipc** — low-level mpv IPC client (Unix socket, JSON protocol)
- **player** — wraps ipc, tracks playback state, exposes Play/Pause/Stop
- **cmd/termelody** — entry point and CLI

## Dependency graph

```
cmd/termelody
  -> player -> ipc
  -> loader -> scanner -> playlist
            -> metadata -> playlist
```

## Planned packages

- **ui** — Bubble Tea TUI for interactive browsing and control

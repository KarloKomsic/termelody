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
- **ui** — Bubble Tea TUI for browsing and control, the preference schema, and
  the config file reader and writer
- **cmd/termelody** — entry point, CLI flags, config path resolution, and
  precedence

## Dependency graph

```
cmd/termelody
  -> player -> ipc
  -> ui -> player -> ipc
        -> playlist
  -> loader -> scanner -> playlist
            -> metadata -> playlist
```

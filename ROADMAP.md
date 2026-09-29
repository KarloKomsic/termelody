# Roadmap

## Phase 1 - Foundation

- [x] Create repository
- [x] Initialize Go module
- [x] Setup Git

## Phase 2 - Scanner

- [x] Learn basic Go
- [x] Scan music directory
- [x] Detect supported formats
- [x] Create Track structure

## Phase 3 - Playback

- [x] IPC client for mpv (Unix socket)
- [x] Launch and control mpv from Go
- [x] Player package with state tracking
- [x] Playlist navigation (next/prev)
- [x] Seek forward/back with configurable step
- [x] Bubble Tea TUI
- [x] Subscribe to mpv events; state derived from reality
- [x] Own the mpv process (reap it, surface crashes, clean up socket)

## Phase 4 - Polish

- [x] Centered layout
- [x] Keybindings
- [x] Playback position indicator and progress bar
- [x] Auto-advance to next track on end
- [ ] Volume control
- [ ] Configurable seek step via flag or config file
- [ ] Recursive directory scanning
- [ ] Scrolling for long playlists
- [ ] Command-line argument for music directory
- [ ] If launched with no arguement, TUI menu starts from the $HOME directory so the user can navigate to the directory, once they are done, they press a certain key to put that as the directory in which scanning starts
- [ ] Shuffle and repeat
- [ ] Allow configuration from either the config file (manually), OR from the TUI itself
- [ ] Error recovery and reconnection

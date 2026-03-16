# Nest

SSH client with a terminal UI for collecting connection details and a Fyne-based GUI terminal for the session.

Supports direct connections and one level of nested (tunneled) connections for reaching hosts behind a jump server.

## Requirements

- Go 1.25+
- Fyne system dependencies (OpenGL, C compiler). See https://docs.fyne.io/started/

On Arch Linux:

```
sudo pacman -S gcc pkg-config mesa xorg-server-devel libxcursor libxrandr libxinerama libxi
```

On Ubuntu/Debian:

```
sudo apt install gcc pkg-config libgl1-mesa-dev xorg-dev
```

## Build

```
go build -o nest .
```

## Run

```
./nest
```

Or directly:

```
go run main.go
```

## Usage

### Main menu

Use arrow keys to navigate and enter to select.

- **Connect to Server** -- Enter credentials for a new connection.
- **Past Connections** -- Pick a previously used connection to reconnect.
- **Configuration** -- Not yet implemented.

### New connection

You are prompted for username, IP address, port (defaults to 22), and password. Press enter to advance through each field. Press esc to go back one field.

After entering the source host credentials, you are asked whether to add a nested connection. Press `y` to tunnel through the source host to a second host (e.g. a VM). Press `n` to connect directly. Only one level of nesting is supported.

### Past connections

Connections are saved automatically to `~/.config/nest/connections.json` after each session. Selecting a past connection reconnects with the stored credentials.

### Terminal

After connecting, a Fyne window opens with an interactive terminal session. The terminal uses a Catppuccin Mocha color scheme and supports ANSI escape sequences (cursor movement, 256-color, truecolor).

Keyboard input is forwarded to the remote shell. Arrow keys, tab, backspace, delete, home, end, page up, and page down are supported.

Close the window or type `exit` in the shell to disconnect.

## Project structure

```
main.go      -- Entry point. Runs the TUI, builds the SSH chain, opens the GUI.
tui/tui.go   -- BubbleTea TUI for collecting connection details and managing past connections.
ssh/ssh.go   -- SSH connection logic including tunneled/nested connections.
gui/gui.go   -- Fyne GUI terminal emulator with ANSI parsing.
```

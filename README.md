# Hyprtime

A minimal screen time tracker for hyprland.

## Features

- Track application usage in real-time
- View daily screen time statistics
- Persistent data storage with SQLite
- Clean and minimal UI

## Installation

```bash
yay -S hyprtime-bin
```

## Usage

1. Add the daemon to your Hyprland Lua config. Autostart apps on `hyprland.start`:

```lua
hl.on("hyprland.start", function ()
	hl.exec_cmd("hyprtimed")
end)
```

2. Restart Hyprland

3. Launch the application from your application menu or run:

```bash
hyprtime
```

The daemon will automatically track your screen time in the background, and you can open the application to view your daily usage statistics.

## License

See [LICENSE](LICENSE) file for details.

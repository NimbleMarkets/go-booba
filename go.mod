module github.com/NimbleMarkets/booba

go 1.25.3

// development version using local bubbletea module
replace github.com/charmbracelet/bubbletea/v2 => ../bubbletea

require (
	github.com/charmbracelet/bubbletea/v2 v2.0.0-beta.5
	github.com/charmbracelet/lipgloss/v2 v2.0.0-beta1
	github.com/fogleman/ease v0.0.0-20170301025033-8da417bf1776
	github.com/gorilla/websocket v1.5.3
	github.com/lucasb-eyer/go-colorful v1.3.0
)

require (
	github.com/charmbracelet/colorprofile v0.3.2 // indirect
	github.com/charmbracelet/ultraviolet v0.0.0-20251017140847-d4ace4d6e731 // indirect
	github.com/charmbracelet/x/ansi v0.10.2 // indirect
	github.com/charmbracelet/x/cellbuf v0.0.13 // indirect
	github.com/charmbracelet/x/term v0.2.1 // indirect
	github.com/charmbracelet/x/termios v0.1.1 // indirect
	github.com/charmbracelet/x/windows v0.2.2 // indirect
	github.com/mattn/go-runewidth v0.0.17 // indirect
	github.com/muesli/cancelreader v0.2.2 // indirect
	github.com/rivo/uniseg v0.4.7 // indirect
	github.com/xo/terminfo v0.0.0-20220910002029-abceb7e1c41e // indirect
	golang.org/x/sync v0.17.0 // indirect
	golang.org/x/sys v0.37.0 // indirect
)

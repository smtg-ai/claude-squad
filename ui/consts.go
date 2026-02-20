package ui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

var beeBannerRaw = `       ██  ██
        ████
     ██████████
    ██  ████  ██
     ██████████
 ░░░████████████░░░
░░░░████████████░░░░
 ░░░████████████░░░
   ▓▓▓▓▓▓▓▓▓▓▓▓▓▓
   ██████████████
   ▓▓▓▓▓▓▓▓▓▓▓▓▓▓
   ██████████████
    ████████████
      ████████
        ██`

var fallbackBannerRaw = `██╗░░██╗██╗██╗░░░██╗███████╗███╗░░░███╗██╗███╗░░██╗██████╗░
██║░░██║██║██║░░░██║██╔════╝████╗░████║██║████╗░██║██╔══██╗
███████║██║╚██╗░██╔╝█████╗░░██╔████╔██║██║██╔██╗██║██║░░██║
██╔══██║██║░╚████╔╝░██╔══╝░░██║╚██╔╝██║██║██║╚████║██║░░██║
██║░░██║██║░░╚██╔╝░░███████╗██║░╚═╝░██║██║██║░╚███║██████╔╝
╚═╝░░╚═╝╚═╝░░░╚═╝░░░╚══════╝╚═╝░░░░░╚═╝╚═╝╚═╝░░╚══╝╚═════╝`

// padLines pads all lines to the widest line's width so lipgloss
// center-alignment shifts every line by the same amount.
func padLines(s string) string {
	lines := strings.Split(s, "\n")
	maxW := 0
	for _, l := range lines {
		if w := len([]rune(l)); w > maxW {
			maxW = w
		}
	}
	for i, l := range lines {
		if w := len([]rune(l)); w < maxW {
			lines[i] = l + strings.Repeat(" ", maxW-w)
		}
	}
	return strings.Join(lines, "\n")
}

var FallBackText = lipgloss.JoinVertical(lipgloss.Center,
	GradientText(padLines(beeBannerRaw), "#F0A868", "#E0C070"),
	"",
	GradientText(fallbackBannerRaw, "#F0A868", "#7EC8D8"))

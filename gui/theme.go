package gui

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
)

// Color constants (Catppuccin Mocha palette)
var (
	colorBase     = color.NRGBA{R: 0x1e, G: 0x1e, B: 0x2e, A: 0xff}
	colorMantle   = color.NRGBA{R: 0x18, G: 0x18, B: 0x25, A: 0xff}
	colorSurface0 = color.NRGBA{R: 0x31, G: 0x32, B: 0x44, A: 0xff}
	colorText     = color.NRGBA{R: 0xcd, G: 0xd6, B: 0xf4, A: 0xff}
	colorSubtext  = color.NRGBA{R: 0xa6, G: 0xad, B: 0xc8, A: 0xff}
	colorOverlay  = color.NRGBA{R: 0x6c, G: 0x70, B: 0x86, A: 0xff}
	colorMauve    = color.NRGBA{R: 0xcb, G: 0xa6, B: 0xf7, A: 0xff}
	colorGreen    = color.NRGBA{R: 0xa6, G: 0xe3, B: 0xa1, A: 0xff}
	colorYellow   = color.NRGBA{R: 0xf9, G: 0xe2, B: 0xaf, A: 0xff}
	colorRed      = color.NRGBA{R: 0xf3, G: 0x8b, B: 0xa8, A: 0xff}
	colorBlue     = color.NRGBA{R: 0x89, G: 0xb4, B: 0xfa, A: 0xff}
)

type squadTheme struct{}

var _ fyne.Theme = (*squadTheme)(nil)

func (s *squadTheme) Color(name fyne.ThemeColorName, variant fyne.ThemeVariant) color.Color {
	switch name {
	case theme.ColorNameBackground:
		return colorBase
	case theme.ColorNameForeground:
		return colorText
	case theme.ColorNamePrimary:
		return colorMauve
	case theme.ColorNameFocus:
		return colorMauve
	case theme.ColorNameSelection:
		return colorSurface0
	case theme.ColorNameSeparator:
		return colorSurface0
	case theme.ColorNameInputBackground:
		return colorMantle
	case theme.ColorNameMenuBackground:
		return colorMantle
	case theme.ColorNameOverlayBackground:
		return colorMantle
	case theme.ColorNameHeaderBackground:
		return colorMantle
	case theme.ColorNameButton:
		return colorSurface0
	case theme.ColorNameScrollBar:
		return colorOverlay
	case theme.ColorNameHover:
		return colorSurface0
	case theme.ColorNameDisabled:
		return colorOverlay
	case theme.ColorNameError:
		return colorRed
	default:
		return theme.DefaultTheme().Color(name, theme.VariantDark)
	}
}

func (s *squadTheme) Font(style fyne.TextStyle) fyne.Resource {
	if style.Monospace {
		return theme.DefaultTheme().Font(style)
	}
	return theme.DefaultTheme().Font(style)
}

func (s *squadTheme) Icon(name fyne.ThemeIconName) fyne.Resource {
	return theme.DefaultTheme().Icon(name)
}

func (s *squadTheme) Size(name fyne.ThemeSizeName) float32 {
	return theme.DefaultTheme().Size(name)
}

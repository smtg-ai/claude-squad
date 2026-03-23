package sidebar

import (
	"claude-squad/session"
	"fmt"
	"image/color"
	"sort"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

var (
	colorGreen   = color.NRGBA{R: 0xa6, G: 0xe3, B: 0xa1, A: 0xff}
	colorYellow  = color.NRGBA{R: 0xf9, G: 0xe2, B: 0xaf, A: 0xff}
	colorOverlay = color.NRGBA{R: 0x6c, G: 0x70, B: 0x86, A: 0xff}
	colorText    = color.NRGBA{R: 0xcd, G: 0xd6, B: 0xf4, A: 0xff}
	colorSubtext = color.NRGBA{R: 0xa6, G: 0xad, B: 0xc8, A: 0xff}
)

// listEntry represents a single row in the flattened list.
type listEntry struct {
	isHeader bool
	text     string
	instance *session.Instance
}

// SessionList is a grouped, sorted session list widget.
type SessionList struct {
	widget.BaseWidget
	list        *widget.List
	entries     []listEntry
	onSelect    func(*session.Instance)
	onActivate  func(*session.Instance) // double-click
	selectedIdx int
}

// NewSessionList creates a new session list widget.
func NewSessionList(onSelect func(*session.Instance), onActivate func(*session.Instance)) *SessionList {
	sl := &SessionList{
		onSelect:    onSelect,
		onActivate:  onActivate,
		selectedIdx: -1,
	}

	sl.list = widget.NewList(
		func() int { return len(sl.entries) },
		func() fyne.CanvasObject {
			icon := canvas.NewText("●", colorGreen)
			icon.TextSize = 12
			name := widget.NewLabel("Session Name")
			name.TextStyle = fyne.TextStyle{Bold: true}
			subtitle := widget.NewLabel("Status +0/-0")
			subtitle.TextStyle = fyne.TextStyle{Italic: true}

			header := widget.NewLabelWithStyle("SECTION", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})

			return container.NewStack(
				header,
				container.NewHBox(icon, container.NewVBox(name, subtitle)),
			)
		},
		func(id widget.ListItemID, obj fyne.CanvasObject) {
			if id >= len(sl.entries) {
				return
			}
			entry := sl.entries[id]
			stack := obj.(*fyne.Container)
			header := stack.Objects[0].(*widget.Label)
			itemBox := stack.Objects[1].(*fyne.Container)

			if entry.isHeader {
				header.SetText(entry.text)
				header.Show()
				itemBox.Hide()
				return
			}

			header.Hide()
			itemBox.Show()

			icon := itemBox.Objects[0].(*canvas.Text)
			vbox := itemBox.Objects[1].(*fyne.Container)
			name := vbox.Objects[0].(*widget.Label)
			subtitle := vbox.Objects[1].(*widget.Label)

			name.SetText(entry.instance.Title)
			sl.updateEntryStyle(entry.instance, icon, name, subtitle)
		},
	)

	sl.list.OnSelected = func(id widget.ListItemID) {
		if id >= len(sl.entries) || sl.entries[id].isHeader {
			sl.list.UnselectAll()
			return
		}
		sl.selectedIdx = id
		if sl.onSelect != nil {
			sl.onSelect(sl.entries[id].instance)
		}
	}

	sl.ExtendBaseWidget(sl)
	return sl
}

// Update rebuilds the list from the current instances.
func (sl *SessionList) Update(instances []*session.Instance) {
	// Remember selected instance title to preserve selection
	var selectedTitle string
	if sl.selectedIdx >= 0 && sl.selectedIdx < len(sl.entries) && !sl.entries[sl.selectedIdx].isHeader {
		selectedTitle = sl.entries[sl.selectedIdx].instance.Title
	}

	var active, paused []*session.Instance
	for _, inst := range instances {
		if inst.Status == session.Paused {
			paused = append(paused, inst)
		} else {
			active = append(active, inst)
		}
	}

	sort.Slice(active, func(i, j int) bool { return active[i].Title < active[j].Title })
	sort.Slice(paused, func(i, j int) bool { return paused[i].Title < paused[j].Title })

	sl.entries = nil
	if len(active) > 0 {
		sl.entries = append(sl.entries, listEntry{isHeader: true, text: "ACTIVE"})
		for _, inst := range active {
			sl.entries = append(sl.entries, listEntry{instance: inst})
		}
	}
	if len(paused) > 0 {
		sl.entries = append(sl.entries, listEntry{isHeader: true, text: "PAUSED"})
		for _, inst := range paused {
			sl.entries = append(sl.entries, listEntry{instance: inst})
		}
	}

	// Restore selection by title, or reset if not found
	sl.selectedIdx = -1
	for i, e := range sl.entries {
		if !e.isHeader && e.instance.Title == selectedTitle {
			sl.selectedIdx = i
			break
		}
	}

	sl.list.Refresh()
}

func (sl *SessionList) updateEntryStyle(inst *session.Instance, icon *canvas.Text, name *widget.Label, subtitle *widget.Label) {
	var statusText string
	switch inst.Status {
	case session.Running:
		icon.Text = "●"
		icon.Color = colorGreen
		statusText = "Running..."
	case session.Ready:
		icon.Text = "▲"
		icon.Color = colorYellow
		statusText = "Needs input"
		name.TextStyle = fyne.TextStyle{Bold: true}
	case session.Loading:
		icon.Text = "◌"
		icon.Color = colorOverlay
		statusText = "Loading..."
	case session.Paused:
		icon.Text = "⏸"
		icon.Color = colorOverlay
		statusText = "Paused"
	}
	icon.Refresh()

	diffStats := inst.GetDiffStats()
	if diffStats != nil {
		subtitle.SetText(fmt.Sprintf("%s +%d/-%d", statusText, diffStats.Added, diffStats.Removed))
	} else {
		subtitle.SetText(statusText)
	}
}

// SelectedInstance returns the currently selected instance.
func (sl *SessionList) SelectedInstance() *session.Instance {
	if sl.selectedIdx < 0 || sl.selectedIdx >= len(sl.entries) {
		return nil
	}
	return sl.entries[sl.selectedIdx].instance
}

// SelectUp moves selection up one non-header item.
func (sl *SessionList) SelectUp() {
	for i := sl.selectedIdx - 1; i >= 0; i-- {
		if !sl.entries[i].isHeader {
			sl.list.Select(i)
			return
		}
	}
}

// SelectDown moves selection down one non-header item.
func (sl *SessionList) SelectDown() {
	for i := sl.selectedIdx + 1; i < len(sl.entries); i++ {
		if !sl.entries[i].isHeader {
			sl.list.Select(i)
			return
		}
	}
}

// CreateRenderer returns the list widget as the renderer.
func (sl *SessionList) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(sl.list)
}

// Suppress unused variable warnings for declared colors.
var _ = colorText
var _ = colorSubtext

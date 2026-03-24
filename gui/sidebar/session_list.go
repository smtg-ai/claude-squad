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

// ContextActions holds callbacks for right-click context menu actions.
type ContextActions struct {
	OnOpen   func(*session.Instance)
	OnPause  func(*session.Instance)
	OnDelete func(*session.Instance)
}

var (
	colorGreen    = color.NRGBA{R: 0xa6, G: 0xe3, B: 0xa1, A: 0xff}
	colorYellow   = color.NRGBA{R: 0xf9, G: 0xe2, B: 0xaf, A: 0xff}
	colorOverlay  = color.NRGBA{R: 0x6c, G: 0x70, B: 0x86, A: 0xff}
	colorText     = color.NRGBA{R: 0xcd, G: 0xd6, B: 0xf4, A: 0xff}
	colorSubtext  = color.NRGBA{R: 0xa6, G: 0xad, B: 0xc8, A: 0xff}
	colorSelected = color.NRGBA{R: 0x45, G: 0x47, B: 0x5a, A: 0xff}
)

// sessionRow is a single clickable row in the session list.
type sessionRow struct {
	widget.BaseWidget
	instance *session.Instance
	onTap    func(*session.Instance)
	actions  *ContextActions
	canvas   fyne.Canvas
	selected bool

	bg       *canvas.Rectangle
	icon     *canvas.Text
	name     *canvas.Text
	subtitle *canvas.Text
}

func newSessionRow(c fyne.Canvas, actions *ContextActions, onTap func(*session.Instance)) *sessionRow {
	r := &sessionRow{
		canvas:   c,
		actions:  actions,
		onTap:    onTap,
		bg:       canvas.NewRectangle(color.Transparent),
		icon:     canvas.NewText("●", colorGreen),
		name:     canvas.NewText("", colorText),
		subtitle: canvas.NewText("", colorSubtext),
	}
	r.icon.TextSize = 12
	r.name.TextSize = 14
	r.name.TextStyle = fyne.TextStyle{Bold: true}
	r.subtitle.TextSize = 12
	r.subtitle.TextStyle = fyne.TextStyle{Italic: true}
	r.ExtendBaseWidget(r)
	return r
}

func (r *sessionRow) SetInstance(inst *session.Instance) {
	r.instance = inst
	if inst == nil {
		return
	}
	r.name.Text = inst.Title
	r.name.Refresh()
	r.updateStyle()
}

func (r *sessionRow) SetSelected(selected bool) {
	r.selected = selected
	if selected {
		r.bg.FillColor = colorSelected
	} else {
		r.bg.FillColor = color.Transparent
	}
	r.bg.Refresh()
}

func (r *sessionRow) Tapped(_ *fyne.PointEvent) {
	if r.instance != nil && r.onTap != nil {
		r.onTap(r.instance)
	}
}

func (r *sessionRow) TappedSecondary(ev *fyne.PointEvent) {
	if r.instance == nil || r.actions == nil {
		return
	}
	showContextMenu(r.instance, r.actions, r.canvas, ev.AbsolutePosition)
}

func (r *sessionRow) updateStyle() {
	inst := r.instance
	if inst == nil {
		return
	}
	var statusText string
	switch inst.Status {
	case session.Running:
		r.icon.Text = "●"
		r.icon.Color = colorGreen
		statusText = "Running..."
	case session.Ready:
		r.icon.Text = "▲"
		r.icon.Color = colorYellow
		statusText = "Needs input"
	case session.Loading:
		r.icon.Text = "◌"
		r.icon.Color = colorOverlay
		statusText = "Loading..."
	case session.Paused:
		r.icon.Text = "⏸"
		r.icon.Color = colorOverlay
		statusText = "Paused"
	}
	r.icon.Refresh()

	diffStats := inst.GetDiffStats()
	if diffStats != nil {
		r.subtitle.Text = fmt.Sprintf("%s +%d/-%d", statusText, diffStats.Added, diffStats.Removed)
	} else {
		r.subtitle.Text = statusText
	}
	r.subtitle.Refresh()
}

func (r *sessionRow) CreateRenderer() fyne.WidgetRenderer {
	content := container.NewStack(
		r.bg,
		container.NewPadded(
			container.NewHBox(r.icon, container.NewVBox(r.name, r.subtitle)),
		),
	)
	return widget.NewSimpleRenderer(content)
}

// SessionList is a grouped, sorted session list widget.
type SessionList struct {
	widget.BaseWidget
	vbox        *fyne.Container
	scroll      *container.Scroll
	entries     []listEntry
	rows        []*sessionRow
	onSelect    func(*session.Instance)
	onActivate  func(*session.Instance)
	actions     *ContextActions
	canvas      fyne.Canvas
	selectedIdx int
}

// listEntry represents a single row in the flattened list.
type listEntry struct {
	isHeader bool
	text     string
	instance *session.Instance
}

// NewSessionList creates a new session list widget.
func NewSessionList(onSelect func(*session.Instance), onActivate func(*session.Instance), c fyne.Canvas, actions *ContextActions) *SessionList {
	sl := &SessionList{
		onSelect:    onSelect,
		onActivate:  onActivate,
		actions:     actions,
		canvas:      c,
		selectedIdx: -1,
		vbox:        container.NewVBox(),
	}
	sl.scroll = container.NewVScroll(sl.vbox)
	sl.ExtendBaseWidget(sl)
	return sl
}

// Update rebuilds the list from the current instances.
func (sl *SessionList) Update(instances []*session.Instance) {
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

	// Rebuild the VBox
	sl.vbox.Objects = nil
	sl.rows = nil
	sl.selectedIdx = -1

	for i, entry := range sl.entries {
		if entry.isHeader {
			header := widget.NewLabelWithStyle(entry.text, fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
			sl.vbox.Add(header)
			sl.rows = append(sl.rows, nil) // placeholder — no row for headers
		} else {
			idx := i
			row := newSessionRow(sl.canvas, sl.actions, func(inst *session.Instance) {
				sl.selectIndex(idx)
			})
			row.SetInstance(entry.instance)
			sl.vbox.Add(row)
			sl.rows = append(sl.rows, row)

			if entry.instance.Title == selectedTitle {
				sl.selectedIdx = idx
				row.SetSelected(true)
			}
		}
	}

	sl.vbox.Refresh()
}

func (sl *SessionList) selectIndex(idx int) {
	if idx < 0 || idx >= len(sl.entries) || sl.entries[idx].isHeader {
		return
	}
	// Deselect previous
	if sl.selectedIdx >= 0 && sl.selectedIdx < len(sl.rows) && sl.rows[sl.selectedIdx] != nil {
		sl.rows[sl.selectedIdx].SetSelected(false)
	}
	sl.selectedIdx = idx
	if sl.rows[idx] != nil {
		sl.rows[idx].SetSelected(true)
	}
	if sl.onSelect != nil {
		sl.onSelect(sl.entries[idx].instance)
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
			sl.selectIndex(i)
			return
		}
	}
}

// SelectDown moves selection down one non-header item.
func (sl *SessionList) SelectDown() {
	for i := sl.selectedIdx + 1; i < len(sl.entries); i++ {
		if !sl.entries[i].isHeader {
			sl.selectIndex(i)
			return
		}
	}
}

// CreateRenderer returns the scrollable list as the renderer.
func (sl *SessionList) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(sl.scroll)
}

// showContextMenu displays a popup context menu for the given session instance.
func showContextMenu(inst *session.Instance, actions *ContextActions, c fyne.Canvas, pos fyne.Position) {
	items := []*fyne.MenuItem{
		fyne.NewMenuItem("Open", func() {
			if actions.OnOpen != nil {
				actions.OnOpen(inst)
			}
		}),
	}
	if inst.Status == session.Paused {
		items = append(items, fyne.NewMenuItem("Resume", func() {
			if actions.OnOpen != nil {
				actions.OnOpen(inst)
			}
		}))
	} else {
		items = append(items, fyne.NewMenuItem("Pause", func() {
			if actions.OnPause != nil {
				actions.OnPause(inst)
			}
		}))
	}
	items = append(items, fyne.NewMenuItemSeparator())
	items = append(items, fyne.NewMenuItem("Delete", func() {
		if actions.OnDelete != nil {
			actions.OnDelete(inst)
		}
	}))

	menu := fyne.NewMenu("", items...)
	popup := widget.NewPopUpMenu(menu, c)
	popup.ShowAtPosition(pos)
}

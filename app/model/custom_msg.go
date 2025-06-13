package model

import (
	"claude-squad/keys"
	"claude-squad/log"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// keyupMsg implements tea.Msg and clears the menu option highlighting after 500ms.
type keyupMsg struct{}

// keydownCallback clears the menu option highlighting after 500ms.
func (m *Model) keydownCallback(name keys.KeyName) tea.Cmd {
	m.menu.Keydown(name)
	return func() tea.Msg {
		select {
		case <-m.ctx.Done():
		case <-time.After(500 * time.Millisecond):
		}

		return keyupMsg{}
	}
}

// hideErrMsg implements tea.Msg and clears the error text from the screen.
type hideErrMsg struct{}

// handleError handles all errors which get bubbled up to the app. sets the error message. We return a callback tea.Cmd that returns a hideErrMsg message
// which clears the error message after 3 seconds.
func (m *Model) handleError(err error) tea.Cmd {
	log.ErrorLog.Printf("%v", err)
	m.errBox.SetError(err)
	return func() tea.Msg {
		select {
		case <-m.ctx.Done():
		case <-time.After(3 * time.Second):
		}

		return hideErrMsg{}
	}
}

// tickUpdateMetadataMessage implements tea.Msg and triggers an update of the metadata of the instances
type tickUpdateMetadataMessage struct{}

// tickUpdateMetadataCmd is the callback to update the metadata of the instances every 500ms. Note that we iterate
// overall the instances and capture their output. It's a pretty expensive operation. Let's do it 2x a second only.
var tickUpdateMetadataCmd = func() tea.Msg {
	time.Sleep(500 * time.Millisecond)
	return tickUpdateMetadataMessage{}
}

// previewTickMsg implements tea.Msg and triggers a preview update
type previewTickMsg struct{}

// instanceChangedMsg implements tea.Msg and triggers an update of the instance
// list
type instanceChangedMsg struct{}

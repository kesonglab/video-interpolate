package tui

import tea "charm.land/bubbletea/v2"

// keymap holds the keys every page can rely on. Pages build their own
// behaviour on top, but navigation/quit stay consistent.
type keymap struct {
	Up       tea.Key
	Down     tea.Key
	Left     tea.Key
	Right    tea.Key
	Enter    tea.Key
	Esc      tea.Key
	Back     tea.Key
	Quit     tea.Key
	Tab      tea.Key
	ShiftTab tea.Key
	Paste    tea.Key // 'p'
	Add      tea.Key // 'a'
	Remove   tea.Key
	Help     tea.Key // '?'
}

func defaultKeymap() keymap {
	return keymap{
		Up:       tea.Key{Code: tea.KeyUp},
		Down:     tea.Key{Code: tea.KeyDown},
		Left:     tea.Key{Code: tea.KeyLeft},
		Right:    tea.Key{Code: tea.KeyRight},
		Enter:    tea.Key{Code: tea.KeyEnter},
		Esc:      tea.Key{Code: tea.KeyEsc},
		Back:     tea.Key{Code: tea.KeyBackspace},
		Quit:     tea.Key{Code: 'q', Text: "q"},
		Tab:      tea.Key{Code: tea.KeyTab},
		ShiftTab: tea.Key{Code: tea.KeyTab, Mod: tea.ModShift},
		Paste:    tea.Key{Code: 'p', Text: "p"},
		Add:      tea.Key{Code: 'a', Text: "a"},
		Remove:   tea.Key{Code: tea.KeyBackspace},
		Help:     tea.Key{Code: '?', Text: "?"},
	}
}

// matches reports whether a key press equals want. Enter also matches the
// numpad enter and the terminal's alternate return code.
func (k keymap) matches(msg tea.KeyMsg, want tea.Key) bool {
	got := msg.Key()
	if want.Mod != 0 && got.Mod != want.Mod {
		return false
	}
	if want.Text != "" {
		return got.Text == want.Text
	}
	switch want.Code {
	case tea.KeyEnter:
		return got.Code == tea.KeyEnter || got.Code == tea.KeyReturn || got.Code == tea.KeyKpEnter
	default:
		return got.Code == want.Code
	}
}

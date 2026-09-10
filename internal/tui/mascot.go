package tui

import "charm.land/lipgloss/v2"

// reelyFrames is the 4-frame mascot bounce. Frame order loops for a loop.
var reelyFrames = []string{
	`  ┌─────────┐
  │■      ■│
  │  ◕  ◕  │
  │   ▽   │
  │■      ■│
  └────┬────┘
       │
      ╱ ╲`,
	`  ┌─────────┐
  │■      ■│
  │  ◕  ◕  │
  │   ▽   │
  │■      ■│
  └───┬───┬┘
      │  │
     ╱  ╲`,
	`  ┌─────────┐
  │■      ■│
  │  ◕  ◕  │
  │   ▽   │
  │■      ■│
  └───┬─┬──┘
      │ │
     ╱ ╲`,
	`  ┌─────────┐
  │■      ■│
  │  ◕  ◕  │
  │   ▽   │
  │■      ■│
  └──┬──┬──┘
     │  │
    ╱  ╲`,
}

// mascot renders frame n (modulo frame count), tinted with color.
func mascot(frame int, color lipgloss.Style) string {
	return color.Render(reelyFrames[frame%len(reelyFrames)])
}

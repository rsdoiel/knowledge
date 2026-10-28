package cli

import tea "charm.land/bubbletea/v2"

// keyRunes returns the characters a key press typed: empty for a special key
// such as Enter, Esc or an arrow, one rune for a letter, several for a fast
// typist or a test.
func keyRunes(k tea.KeyPressMsg) []rune { return []rune(k.Text) }

// hasAlt reports whether Alt was held with the key.
func hasAlt(k tea.KeyPressMsg) bool { return k.Mod.Contains(tea.ModAlt) }

// isEsc reports whether the key is Esc, with or without a modifier a terminal
// glued to it.
func isEsc(k tea.KeyPressMsg) bool { return k.Code == tea.KeyEscape }

// isCtrlC reports whether the key is Ctrl-C.
func isCtrlC(k tea.KeyPressMsg) bool { return k.Code == 'c' && k.Mod.Contains(tea.ModCtrl) }

package cli

import (
	"image/color"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/colorprofile"
)

func TestInit_AsksForTheBackgroundAndDoesNotWait(t *testing.T) {
	m, _ := writeModel(t)
	if m.Init() == nil {
		t.Fatal("Init returned no command: the background colour is never asked for")
	}
	if !m.isDark {
		t.Error("the interface should assume a dark terminal until it hears otherwise")
	}
}

func TestBackgroundColour_ALightTerminalRestylesTheListsWithoutHarm(t *testing.T) {
	m, _ := writeModel(t)
	m, _ = press(t, m, tea.WindowSizeMsg{Width: 80, Height: 24})
	m, _ = press(t, m, tea.BackgroundColorMsg{Color: color.White})
	if m.isDark {
		t.Error("a white background was taken for a dark one")
	}
	m, _ = press(t, m, tea.BackgroundColorMsg{Color: color.Black})
	if !m.isDark {
		t.Error("a black background was taken for a light one")
	}
}

func TestColourProfile_ColourIsOnOnlyWhenTheTerminalShowsItAndNoColorIsUnset(t *testing.T) {
	t.Setenv("NO_COLOR", "")
	m, _ := writeModel(t)
	if m.colourOn() {
		t.Error("colour is on before the terminal has said what it shows")
	}
	m, _ = press(t, m, tea.ColorProfileMsg{Profile: colorprofile.ASCII})
	if m.colourOn() {
		t.Error("colour is on for a terminal with no colour")
	}
	m, _ = press(t, m, tea.ColorProfileMsg{Profile: colorprofile.TrueColor})
	if !m.colourOn() {
		t.Error("colour is off for a true-colour terminal")
	}
	t.Setenv("NO_COLOR", "1")
	if m.colourOn() {
		t.Error("NO_COLOR did not turn colour off")
	}
}

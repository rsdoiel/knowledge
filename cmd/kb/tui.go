package main

import (
	tea "github.com/charmbracelet/bubbletea"

	knowledge "github.com/rsdoiel/knowledge"
)

// runTUI launches the interactive browser against an already-open kb.
// Called from main when kb is invoked with no verb at all.
func runTUI(kb *knowledge.KnowledgeBase, dl *DebugLog) error {
	m, err := newTUIModel(kb, dl)
	if err != nil {
		return err
	}
	_, err = tea.NewProgram(m, tea.WithAltScreen()).Run()
	return err
}

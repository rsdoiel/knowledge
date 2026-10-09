package main

import (
	tea "github.com/charmbracelet/bubbletea"

	knowledge "github.com/rsdoiel/knowledge"
)

// runTUI launches the interactive browser against an already-open kb.
// Called from main when kb is invoked with no verb at all.
func runTUI(kb *knowledge.KnowledgeBase, dl *DebugLog, start tuiStart) error {
	m, err := newTUIModelAt(kb, dl, start)
	if err != nil {
		return err
	}
	_, err = tea.NewProgram(m, tea.WithAltScreen()).Run()
	return err
}

// launchTUI starts the interface at a place in the tree. It is a variable so tests
// can see where a command line would open it without running a terminal program.
var launchTUI = func(kb *knowledge.KnowledgeBase, dl *DebugLog, start tuiStart) error {
	return runTUI(kb, dl, start)
}

package main

import "testing"

func TestChangeExitWaitsForChange(t *testing.T) {
	m := newModel([]string{"unused"})
	m.chgExit = true
	for i, output := range []string{"first", "first", "second"} {
		_, exit := m.handleCmdCycle(cmdMsg{out: []byte(output)})
		if want := i == 2; exit != want {
			t.Fatalf("cycle %d: exit = %v, want %v", i, exit, want)
		}
	}
}

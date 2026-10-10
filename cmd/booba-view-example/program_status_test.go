package main

import (
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestProgramStatusFollowsScreens(t *testing.T) {
	cases := []struct {
		name     string
		m        model
		state    tea.ProgramState
		message  string
		progress int
		has      bool
	}{
		{"menu", model{}, tea.ProgramStateIdle, "Choose a task", 0, false},
		{"loading", model{Chosen: true, Progress: 0.42}, tea.ProgramStateWorking, "Loading", 42, true},
		{"loaded", model{Chosen: true, Loaded: true, Progress: 1}, tea.ProgramStateDone, "Loaded", 0, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.m.View().ProgramStatus
			if got == nil {
				t.Fatal("View().ProgramStatus is nil")
			}
			if got.App != "booba-views" || got.Title != "Views demo" {
				t.Errorf("App/Title = %q/%q; every report must repeat them", got.App, got.Title)
			}
			if got.State != tc.state || got.Message != tc.message {
				t.Errorf("state/message = %v/%q, want %v/%q", got.State, got.Message, tc.state, tc.message)
			}
			if got.HasProgress != tc.has || got.Progress != tc.progress {
				t.Errorf("progress = %d (has=%v), want %d (has=%v)", got.Progress, got.HasProgress, tc.progress, tc.has)
			}
		})
	}
	if got := (model{Quitting: true, Chosen: true, Loaded: true, Progress: 1}).View().ProgramStatus; got == nil || got.State != tea.ProgramStateDone {
		t.Errorf("farewell view must keep the last status (done), got %+v", got)
	}
	if got := (model{Quitting: true}).View().ProgramStatus; got == nil || got.State != tea.ProgramStateIdle {
		t.Errorf("quitting from the menu must keep idle, got %+v", got)
	}
}

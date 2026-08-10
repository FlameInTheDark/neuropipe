package app

import "testing"

type recordedTrayActions struct{ calls []string }

func (a *recordedTrayActions) Show()         { a.calls = append(a.calls, "show") }
func (a *recordedTrayActions) OpenSettings() { a.calls = append(a.calls, "settings") }
func (a *recordedTrayActions) Hide()         { a.calls = append(a.calls, "hide") }
func (a *recordedTrayActions) Quit()         { a.calls = append(a.calls, "quit") }

func TestTrayControllerRoutesActionsAndQuitsOnce(t *testing.T) {
	t.Parallel()
	actions := &recordedTrayActions{}
	controller := NewTrayController(actions)

	controller.Show()
	controller.OpenSettings()
	controller.Hide()
	controller.Quit()
	controller.Quit()

	want := []string{"show", "settings", "hide", "quit"}
	if len(actions.calls) != len(want) {
		t.Fatalf("actions = %v, want %v", actions.calls, want)
	}
	for index, call := range want {
		if actions.calls[index] != call {
			t.Fatalf("actions[%d] = %q, want %q", index, actions.calls[index], call)
		}
	}
	if !controller.Quitting() {
		t.Fatal("Quitting() = false, want true after an explicit tray quit")
	}
}

func TestNormalizeTrayMenuLabelsUsesLocalizedFallbacks(t *testing.T) {
	t.Parallel()
	labels := normalizeTrayMenuLabels(TrayMenuLabels{Show: "Custom show"}, "ru")
	if labels.Show != "Custom show" || labels.Settings != "Открыть настройки" || labels.Hide != "Скрыть Neuropipe" || labels.Close != "Закрыть Neuropipe" {
		t.Fatalf("normalized labels = %#v", labels)
	}
}

type trayState struct {
	ready    bool
	quitting bool
}

func (t trayState) Start()                   {}
func (t trayState) Stop()                    {}
func (t trayState) Ready() bool              { return t.ready }
func (t trayState) Quitting() bool           { return t.quitting }
func (t trayState) SetLabels(TrayMenuLabels) {}

func TestShouldHideOnClose(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name       string
		enabled    bool
		tray       SystemTray
		wantHidden bool
	}{
		{name: "disabled preference", tray: trayState{ready: true}},
		{name: "tray is still starting", enabled: true, tray: trayState{}},
		{name: "explicit quit", enabled: true, tray: trayState{ready: true, quitting: true}},
		{name: "ready tray and enabled preference", enabled: true, tray: trayState{ready: true}, wantHidden: true},
		{name: "no tray", enabled: true},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			if got := ShouldHideOnClose(test.enabled, test.tray); got != test.wantHidden {
				t.Fatalf("ShouldHideOnClose() = %t, want %t", got, test.wantHidden)
			}
		})
	}
}

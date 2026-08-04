package app

import (
	"context"
	"testing"
	"time"

	"github.com/FlameInTheDark/neuropipe/internal/updatecheck"
)

type staticUpdateChecker struct {
	release   updatecheck.Release
	available bool
	err       error
}

func (c staticUpdateChecker) Check(context.Context) (updatecheck.Release, bool, error) {
	return c.release, c.available, c.err
}

func TestDesktopPublishesAvailableUpdate(t *testing.T) {
	desktop := &Desktop{updates: staticUpdateChecker{release: updatecheck.Release{Version: "v1.2.3", URL: "https://github.com/FlameInTheDark/neuropipe/releases/tag/v1.2.3"}, available: true}}
	desktop.startUpdateChecks(context.Background())
	t.Cleanup(desktop.stopUpdateChecks)

	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		update := desktop.GetUpdateAvailability()
		if update.Available {
			if update.Version != "v1.2.3" || update.URL == "" {
				t.Fatalf("update = %#v", update)
			}
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("available update was not published")
}

func TestDesktopDoesNotPublishUnavailableUpdate(t *testing.T) {
	desktop := &Desktop{updates: staticUpdateChecker{available: false}}
	desktop.startUpdateChecks(context.Background())
	t.Cleanup(desktop.stopUpdateChecks)
	time.Sleep(10 * time.Millisecond)
	if update := desktop.GetUpdateAvailability(); update.Available {
		t.Fatalf("update = %#v, want unavailable", update)
	}
}

package app

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/FlameInTheDark/neuropipe/internal/domain"
	"github.com/FlameInTheDark/neuropipe/internal/updatecheck"
)

const (
	updateCheckInterval  = time.Hour
	updateCheckTimeout   = 12 * time.Second
	updateAvailableEvent = "app.update.available"
)

// UpdateChecker compares a running release with the latest published release.
// It allows the Wails façade to own lifecycle management independently of HTTP.
type UpdateChecker interface {
	Check(context.Context) (updatecheck.Release, bool, error)
}

func (d *Desktop) startUpdateChecks(parent context.Context) {
	if d.updates == nil {
		return
	}
	d.updateMu.Lock()
	if d.updateCancel != nil {
		d.updateMu.Unlock()
		return
	}
	ctx, cancel := context.WithCancel(parent)
	d.updateCancel = cancel
	d.updateWG.Add(1)
	d.updateMu.Unlock()
	go func() {
		defer d.updateWG.Done()
		d.runUpdateChecks(ctx)
	}()
}

func (d *Desktop) stopUpdateChecks() {
	d.updateMu.Lock()
	cancel := d.updateCancel
	d.updateCancel = nil
	d.updateMu.Unlock()
	if cancel != nil {
		cancel()
	}
	d.updateWG.Wait()
}

func (d *Desktop) runUpdateChecks(ctx context.Context) {
	d.checkForUpdate(ctx)
	ticker := time.NewTicker(updateCheckInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			d.checkForUpdate(ctx)
		}
	}
}

func (d *Desktop) checkForUpdate(ctx context.Context) {
	operationCtx, cancel := context.WithTimeout(ctx, updateCheckTimeout)
	release, available, err := d.updates.Check(operationCtx)
	cancel()
	if err != nil || !available {
		return
	}
	update := domain.UpdateAvailability{Available: true, Version: release.Version, URL: release.URL}
	d.updateMu.Lock()
	changed := d.update != update
	d.update = update
	d.updateMu.Unlock()
	if changed {
		d.emit(updateAvailableEvent, update)
	}
}

// GetUpdateAvailability returns a newer release when one was found in the
// background. Development builds intentionally return an unavailable status.
func (d *Desktop) GetUpdateAvailability() domain.UpdateAvailability {
	d.updateMu.RLock()
	defer d.updateMu.RUnlock()
	return d.update
}

// OpenUpdateRelease opens the verified GitHub release page for the discovered
// update without exposing arbitrary browser access to the renderer.
func (d *Desktop) OpenUpdateRelease() error {
	update := d.GetUpdateAvailability()
	if !update.Available || strings.TrimSpace(update.URL) == "" {
		return fmt.Errorf("no Neuropipe update is available")
	}
	target, err := url.Parse(update.URL)
	if err != nil || target.Scheme != "https" || !strings.EqualFold(target.Hostname(), "github.com") {
		return fmt.Errorf("available update has an invalid release URL")
	}
	if d.app == nil {
		return fmt.Errorf("application is not initialised")
	}
	return d.app.Browser.OpenURL(target.String())
}

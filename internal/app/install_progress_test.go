package app

import (
	"testing"

	"github.com/FlameInTheDark/neuropipe/internal/domain"
)

func TestInstallProgressSnapshotPreservesCompletedTransfer(t *testing.T) {
	t.Parallel()
	desktop := &Desktop{}
	desktop.reportInstallProgress(runtimeInstallProgressEvent, domain.InstallProgress{
		Kind: "runtime", Stage: "downloading", Label: "CPU runtime", DownloadedBytes: 24, TotalBytes: 80, BytesPerSecond: 12, Percentage: 30,
	})
	desktop.advanceInstallProgress(runtimeInstallProgressEvent, "runtime", "complete", "Runtime ready")

	progress, err := desktop.GetInstallProgress("runtime")
	if err != nil {
		t.Fatalf("GetInstallProgress() error = %v", err)
	}
	if progress.Stage != "complete" || progress.DownloadedBytes != 80 || progress.TotalBytes != 80 || progress.Percentage != 100 || progress.BytesPerSecond != 12 {
		t.Fatalf("progress = %#v, want completed runtime transfer with the last speed", progress)
	}
}

func TestGetInstallProgressRejectsUnknownKind(t *testing.T) {
	t.Parallel()
	if _, err := (&Desktop{}).GetInstallProgress("other"); err == nil {
		t.Fatal("GetInstallProgress() succeeded for an unknown kind")
	}
}

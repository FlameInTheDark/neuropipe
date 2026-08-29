package storages

import (
	"testing"
)

func TestFTPDebugDelete(t *testing.T) {
	fake := newFakeFTP(t)
	fake.seedFile("logs/app.log", "line")
	fake.seedFile("logs/old/1.log", "1")
	fake.seedFile("logs/old/2.log", "2")
	conn := newFTPConn(t, fake, "")
	result, err := conn.Delete(t.Context(), "logs", true)
	t.Logf("Delete = %#v err = %v", result, err)
	fake.mu.Lock()
	defer fake.mu.Unlock()
	for _, c := range fake.commands {
		t.Logf("CMD: %q", c)
	}
}

package catalog

import "testing"

func TestModuleOwnedTwitchPortsDoNotReceiveLegacyGenericPins(t *testing.T) {
	registry := New()
	trigger, found := registry.Get("twitch:event")
	if !found || !trigger.PortContractOwned {
		t.Fatal("missing module-owned Twitch trigger contract")
	}
	for _, output := range trigger.Outputs {
		if output.ID == "payload" {
			t.Fatal("legacy payload pin was appended to Twitch trigger")
		}
	}
	sender, found := registry.Get("twitch:send_chat_message")
	if !found || !sender.PortContractOwned {
		t.Fatal("missing module-owned Twitch sender contract")
	}
	for _, output := range sender.Outputs {
		if output.ID == "result" {
			t.Fatal("legacy result pin was appended to Twitch sender")
		}
	}
}

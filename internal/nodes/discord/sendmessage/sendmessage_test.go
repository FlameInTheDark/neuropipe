package sendmessage

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/FlameInTheDark/neuropipe/internal/domain"
	"github.com/FlameInTheDark/neuropipe/internal/nodes"
)

type runtimeStub struct{ sender nodes.DiscordSender }

func (r runtimeStub) DiscordSender() nodes.DiscordSender { return r.sender }

type senderStub struct {
	request domain.DiscordMessageRequest
	result  domain.DiscordMessageResult
}

func (s *senderStub) SendDiscordMessage(_ context.Context, request domain.DiscordMessageRequest) (domain.DiscordMessageResult, error) {
	s.request = request
	return s.result, nil
}
func (s *senderStub) SendDiscordDirectMessage(context.Context, domain.DiscordDMRequest) (domain.DiscordMessageResult, error) {
	panic("unused")
}
func (s *senderStub) AddDiscordReaction(context.Context, domain.DiscordReactionRequest) (domain.DiscordActionResult, error) {
	panic("unused")
}
func (s *senderStub) EditDiscordMessage(context.Context, domain.DiscordEditRequest) (domain.DiscordActionResult, error) {
	panic("unused")
}
func (s *senderStub) DeleteDiscordMessage(context.Context, domain.DiscordDeleteRequest) (domain.DiscordActionResult, error) {
	panic("unused")
}
func (s *senderStub) RespondDiscordInteraction(context.Context, domain.DiscordCommandReplyRequest) (domain.DiscordMessageResult, error) {
	panic("unused")
}
func (s *senderStub) SendDiscordFollowup(context.Context, domain.DiscordFollowupRequest) (domain.DiscordMessageResult, error) {
	panic("unused")
}
func (s *senderStub) EditDiscordInteractionMessage(context.Context, domain.DiscordCommandEditRequest) (domain.DiscordActionResult, error) {
	panic("unused")
}

func invocation(inputs map[string]any) nodes.Invocation {
	return nodes.Invocation{
		Node:            domain.FlowNode{Type: "action:discord_send_message", Data: map[string]any{"config": map[string]any{}}},
		Config:          map[string]any{},
		Inputs:          inputs,
		ConnectedInputs: map[string]bool{},
	}
}

func TestSendsMessageAndFollowsSentPort(t *testing.T) {
	sender := &senderStub{result: domain.DiscordMessageResult{MessageID: "msg-9", Sent: true}}
	module := nodes.Implementation{Metadata: definition(), Executor: execute}
	result, err := module.Execute(context.Background(), invocation(map[string]any{
		"channel": "55", "message": "hello", "replyToMessageId": "77", "identityId": "bot-1",
	}), runtimeStub{sender: sender})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Ports) != 1 || result.Ports[0] != "sent" || result.Outputs["messageId"] != "msg-9" {
		t.Fatalf("result = %#v", result)
	}
	if sender.request.ChannelID != "55" || sender.request.Message != "hello" || sender.request.ReplyToID != "77" || sender.request.IdentityID != "bot-1" {
		t.Fatalf("request = %#v", sender.request)
	}
}

func TestSoftRejectionFollowsRejectedPort(t *testing.T) {
	sender := &senderStub{result: domain.DiscordMessageResult{Reason: "Missing Permissions"}}
	module := nodes.Implementation{Metadata: definition(), Executor: execute}
	result, err := module.Execute(context.Background(), invocation(map[string]any{"channel": "55", "message": "hello"}), runtimeStub{sender: sender})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Ports) != 1 || result.Ports[0] != "rejected" || result.Outputs["reason"] != "Missing Permissions" {
		t.Fatalf("result = %#v", result)
	}
}

// Message and channel IDs frequently arrive as numbers (numeric constants,
// JSON extraction); they must be coerced to strings instead of silently
// dropping the value — a dropped reply reference would change the message
// from a reply into a plain send.
func TestNumericIDInputsAreCoerced(t *testing.T) {
	sender := &senderStub{result: domain.DiscordMessageResult{MessageID: "msg-1", Sent: true}}
	module := nodes.Implementation{Metadata: definition(), Executor: execute}
	_, err := module.Execute(context.Background(), invocation(map[string]any{
		"channel": json.Number("565062979255795712"), "message": "hello",
		"replyToMessageId": json.Number("79216925611139072"),
	}), runtimeStub{sender: sender})
	if err != nil {
		t.Fatal(err)
	}
	if sender.request.ChannelID != "565062979255795712" || sender.request.ReplyToID != "79216925611139072" {
		t.Fatalf("json.Number request = %#v", sender.request)
	}

	sender = &senderStub{result: domain.DiscordMessageResult{MessageID: "msg-2", Sent: true}}
	_, err = module.Execute(context.Background(), invocation(map[string]any{
		"channel": float64(55), "message": "hello", "replyToMessageId": float64(100000000000000000),
	}), runtimeStub{sender: sender})
	if err != nil {
		t.Fatal(err)
	}
	// 1e17 is exactly representable in float64; the coercion must render it
	// as a plain digit string, never scientific notation.
	if sender.request.ChannelID != "55" || sender.request.ReplyToID != "100000000000000000" {
		t.Fatalf("float64 request = %#v", sender.request)
	}
}

func TestLengthCapPreValidated(t *testing.T) {
	module := nodes.Implementation{Metadata: definition(), Executor: execute}
	result, err := module.Execute(context.Background(), invocation(map[string]any{"channel": "55", "message": strings.Repeat("a", 2001)}), runtimeStub{sender: &senderStub{result: domain.DiscordMessageResult{Sent: true}}})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Ports) != 1 || result.Ports[0] != "rejected" || !strings.Contains(result.Outputs["reason"].(string), "2,000") {
		t.Fatalf("cap result = %#v", result)
	}
}

func TestValidationErrors(t *testing.T) {
	module := nodes.Implementation{Metadata: definition(), Executor: execute}
	if _, err := module.Execute(context.Background(), invocation(map[string]any{"message": "x"}), runtimeStub{sender: &senderStub{}}); err == nil {
		t.Fatal("missing channel accepted")
	}
	if _, err := module.Execute(context.Background(), invocation(map[string]any{"channel": "1"}), runtimeStub{sender: &senderStub{}}); err == nil {
		t.Fatal("missing message accepted")
	}
	if _, err := module.Execute(context.Background(), invocation(map[string]any{"channel": "1", "message": "x"}), nil); err == nil {
		t.Fatal("missing runtime accepted")
	}
}

/* ------------------------------------------------------------------ */
/* embeds                                                              */
/* ------------------------------------------------------------------ */

func TestEmbedsFromDocumentWithTemplates(t *testing.T) {
	sender := &senderStub{result: domain.DiscordMessageResult{MessageID: "msg-1", Sent: true}}
	module := nodes.Implementation{Metadata: definition(), Executor: execute}
	config := map[string]any{"embeds": map[string]any{
		"pins": []any{map[string]any{"name": "city", "type": "text", "sample": "Berlin"}},
		"embeds": []any{map[string]any{
			"title": "Weather in {{city}}", "color": "#5865F2",
			"fields": []any{map[string]any{"name": "City", "value": "{{city}}"}},
		}},
	}}
	invocation := invocation(map[string]any{"channel": "55", "city": "Osaka"})
	invocation.Config = config
	result, err := module.Execute(context.Background(), invocation, runtimeStub{sender: sender})
	if err != nil {
		t.Fatal(err)
	}
	if result.Ports[0] != "sent" {
		t.Fatalf("ports = %v", result.Ports)
	}
	if len(sender.request.Embeds) != 1 {
		t.Fatalf("embeds = %#v", sender.request.Embeds)
	}
	embed := sender.request.Embeds[0]
	if embed.Title != "Weather in Osaka" || embed.Color != 0x5865F2 {
		t.Fatalf("embed = %#v", embed)
	}
	if len(embed.Fields) != 1 || embed.Fields[0].Value != "Osaka" {
		t.Fatalf("fields = %#v", embed.Fields)
	}
}

func TestEmbedsJsonPinOverridesDocument(t *testing.T) {
	sender := &senderStub{result: domain.DiscordMessageResult{MessageID: "msg-2", Sent: true}}
	module := nodes.Implementation{Metadata: definition(), Executor: execute}
	invocation := invocation(map[string]any{
		"channel": "55", "embedsJson": `{"title":"Live data","description":"from a pin"}`,
	})
	invocation.Config = map[string]any{"embeds": map[string]any{
		"embeds": []any{map[string]any{"title": "from editor"}},
	}}
	if _, err := module.Execute(context.Background(), invocation, runtimeStub{sender: sender}); err != nil {
		t.Fatal(err)
	}
	if len(sender.request.Embeds) != 1 || sender.request.Embeds[0].Title != "Live data" {
		t.Fatalf("embeds = %#v", sender.request.Embeds)
	}
}

func TestEmbedsJsonInvalidRejected(t *testing.T) {
	module := nodes.Implementation{Metadata: definition(), Executor: execute}
	result, err := module.Execute(context.Background(), invocation(map[string]any{
		"channel": "55", "embedsJson": `{"title":`,
	}), runtimeStub{sender: &senderStub{}})
	if err != nil {
		t.Fatal(err)
	}
	if result.Ports[0] != "rejected" || !strings.Contains(result.Outputs["reason"].(string), "not a valid embed") {
		t.Fatalf("result = %#v", result)
	}
}

func TestEmbedsOnlyMessageWithoutText(t *testing.T) {
	sender := &senderStub{result: domain.DiscordMessageResult{MessageID: "msg-3", Sent: true}}
	module := nodes.Implementation{Metadata: definition(), Executor: execute}
	invocation := invocation(map[string]any{"channel": "55"})
	invocation.Config = map[string]any{"embeds": map[string]any{
		"embeds": []any{map[string]any{"title": "No text needed"}},
	}}
	if _, err := module.Execute(context.Background(), invocation, runtimeStub{sender: sender}); err != nil {
		t.Fatal(err)
	}
	if sender.request.Message != "" || len(sender.request.Embeds) != 1 {
		t.Fatalf("request = %#v", sender.request)
	}
}

/* ------------------------------------------------------------------ */
/* attachments                                                         */
/* ------------------------------------------------------------------ */

func TestAttachmentsFromPathAndData(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "report.txt")
	if err := os.WriteFile(path, []byte("report body"), 0o600); err != nil {
		t.Fatal(err)
	}
	sender := &senderStub{result: domain.DiscordMessageResult{MessageID: "msg-4", Sent: true}}
	module := nodes.Implementation{Metadata: definition(), Executor: execute}
	inputs := map[string]any{
		"channel": "55", "message": "see attached",
		"filePath": path, "fileName": "weather.png",
		"fileData": base64.StdEncoding.EncodeToString([]byte("drawn image")),
	}
	invocation := invocation(inputs)
	if _, err := module.Execute(context.Background(), invocation, runtimeStub{sender: sender}); err != nil {
		t.Fatal(err)
	}
	if len(sender.request.Attachments) != 2 {
		t.Fatalf("attachments = %#v", sender.request.Attachments)
	}
	if sender.request.Attachments[0].Name != "report.txt" || string(sender.request.Attachments[0].Data) != "report body" {
		t.Fatalf("path attachment = %#v", sender.request.Attachments[0])
	}
	if sender.request.Attachments[1].Name != "weather.png" || string(sender.request.Attachments[1].Data) != "drawn image" {
		t.Fatalf("data attachment = %#v", sender.request.Attachments[1])
	}
}

func TestAttachmentsOnlyMessageWithoutText(t *testing.T) {
	sender := &senderStub{result: domain.DiscordMessageResult{MessageID: "msg-5", Sent: true}}
	module := nodes.Implementation{Metadata: definition(), Executor: execute}
	invocation := invocation(map[string]any{"channel": "55", "fileData": []byte("bytes"), "fileName": "out.bin"})
	if _, err := module.Execute(context.Background(), invocation, runtimeStub{sender: sender}); err != nil {
		t.Fatal(err)
	}
	if len(sender.request.Attachments) != 1 {
		t.Fatalf("attachments = %#v", sender.request.Attachments)
	}
	if sender.request.Attachments[0].Name != "out.bin" {
		t.Fatalf("attachment = %#v", sender.request.Attachments[0])
	}
}

// missingFileStatError mirrors the os.Stat call attachments.loadPath performs
// on the cleaned path, so the rejection reason can be asserted on any platform
// ("no such file or directory" on Unix, "cannot find the path specified" on
// Windows) instead of hardcoding one OS error text.
func missingFileStatError(t *testing.T, path string) string {
	t.Helper()
	_, err := os.Stat(filepath.Clean(path))
	if err == nil {
		t.Fatalf("expected %q to be missing on this machine", path)
	}
	return err.Error()
}

func TestAttachmentMissingFileRejected(t *testing.T) {
	module := nodes.Implementation{Metadata: definition(), Executor: execute}
	result, err := module.Execute(context.Background(), invocation(map[string]any{
		"channel": "55", "message": "x", "filePath": "/definitely/missing/file.bin",
	}), runtimeStub{sender: &senderStub{}})
	if err != nil {
		t.Fatal(err)
	}
	if result.Ports[0] != "rejected" || !strings.Contains(result.Outputs["reason"].(string), missingFileStatError(t, "/definitely/missing/file.bin")) {
		t.Fatalf("result = %#v", result)
	}
}

func TestEmptyEverythingRejected(t *testing.T) {
	module := nodes.Implementation{Metadata: definition(), Executor: execute}
	if _, err := module.Execute(context.Background(), invocation(map[string]any{"channel": "55"}), runtimeStub{sender: &senderStub{}}); err == nil {
		t.Fatal("empty message with no embeds or attachments accepted")
	}
}

func TestResolveAddsDocumentPinPorts(t *testing.T) {
	node := domain.FlowNode{Type: "action:discord_send_message", Data: map[string]any{"config": map[string]any{
		"embeds": map[string]any{
			"pins": []any{
				map[string]any{"name": "city", "type": "text"},
				map[string]any{"name": "temp", "type": "number"},
				map[string]any{"name": "hot", "type": "boolean"},
			},
			"embeds": []any{map[string]any{"title": "T"}},
		},
	}}}
	definition, err := resolve(node)
	if err != nil {
		t.Fatal(err)
	}
	ids := make([]string, 0, len(definition.Inputs))
	for _, port := range definition.Inputs {
		ids = append(ids, port.ID)
	}
	want := []string{"in", "message", "channel", "replyToMessageId", "embedsJson", "fileUrl", "filePath", "fileBase64", "fileData", "fileName", "identityId", "city", "temp", "hot"}
	if len(ids) != len(want) {
		t.Fatalf("pin ids = %v", ids)
	}
	for index := range want {
		if ids[index] != want[index] {
			t.Fatalf("pin ids = %v, want %v", ids, want)
		}
	}
	for _, port := range definition.Inputs[11:] {
		if port.Kind != domain.PinData || port.MaxConnections != 1 {
			t.Fatalf("dynamic port = %#v", port)
		}
	}
	if definition.Inputs[12].DataType != domain.DataNumber || definition.Inputs[13].DataType != domain.DataBoolean {
		t.Fatalf("typed pins = %#v %#v", definition.Inputs[11], definition.Inputs[12])
	}
}

func TestResolveWithoutEmbedsConfigKeepsBasePins(t *testing.T) {
	// graphs saved before embeds existed carry no embeds config at all
	node := domain.FlowNode{Type: "action:discord_send_message", Data: map[string]any{"config": map[string]any{}}}
	definition, err := resolve(node)
	if err != nil {
		t.Fatal(err)
	}
	if len(definition.Inputs) != 11 {
		t.Fatalf("pin count = %d", len(definition.Inputs))
	}
}

/* ------------------------------------------------------------------ */
/* image source selector                                               */
/* ------------------------------------------------------------------ */

func pinIDs(definition domain.NodeDefinition) []string {
	ids := make([]string, 0, len(definition.Inputs))
	for _, port := range definition.Inputs {
		ids = append(ids, port.ID)
	}
	return ids
}

func resolveWithConfig(t *testing.T, config map[string]any) domain.NodeDefinition {
	t.Helper()
	definition, err := resolve(domain.FlowNode{Type: "action:discord_send_message", Data: map[string]any{"config": config}})
	if err != nil {
		t.Fatal(err)
	}
	return definition
}

func containsPin(ids []string, pinID string) bool {
	for _, id := range ids {
		if id == pinID {
			return true
		}
	}
	return false
}

// An explicit source keeps only its own pin plus the shared file name pin;
// everything unrelated to attachments stays visible.
func TestResolveImageSourceFiltersPins(t *testing.T) {
	urlOnly := pinIDs(resolveWithConfig(t, map[string]any{"imageSource": "url"}))
	if !containsPin(urlOnly, "fileUrl") || containsPin(urlOnly, "filePath") || containsPin(urlOnly, "fileBase64") || containsPin(urlOnly, "fileData") || containsPin(urlOnly, "fileName") {
		t.Fatalf("url mode pins = %v", urlOnly)
	}
	bytesOnly := pinIDs(resolveWithConfig(t, map[string]any{"imageSource": "bytes"}))
	if !containsPin(bytesOnly, "fileData") || !containsPin(bytesOnly, "fileName") || containsPin(bytesOnly, "fileUrl") || containsPin(bytesOnly, "filePath") || containsPin(bytesOnly, "fileBase64") {
		t.Fatalf("bytes mode pins = %v", bytesOnly)
	}
	base64Only := pinIDs(resolveWithConfig(t, map[string]any{"imageSource": "base64"}))
	if !containsPin(base64Only, "fileBase64") || !containsPin(base64Only, "fileName") || containsPin(base64Only, "fileData") {
		t.Fatalf("base64 mode pins = %v", base64Only)
	}
	for _, always := range []string{"in", "message", "channel", "embedsJson", "identityId"} {
		if !containsPin(bytesOnly, always) {
			t.Fatalf("mode filtering dropped unrelated pin %q: %v", always, bytesOnly)
		}
	}
}

// Auto (and a graph saved before the selector existed) keeps every source
// pin so wired connections from older blueprints keep resolving.
func TestResolveImageSourceAutoKeepsAllPins(t *testing.T) {
	for _, config := range []map[string]any{{"imageSource": ""}, {}} {
		ids := pinIDs(resolveWithConfig(t, config))
		for _, pin := range []string{"fileUrl", "filePath", "fileBase64", "fileData", "fileName"} {
			if !containsPin(ids, pin) {
				t.Fatalf("config %v lost pin %q: %v", config, pin, ids)
			}
		}
	}
}

func TestAttachmentFromBase64Source(t *testing.T) {
	sender := &senderStub{result: domain.DiscordMessageResult{MessageID: "msg-6", Sent: true}}
	module := nodes.Implementation{Metadata: definition(), Executor: execute}
	invocation := invocation(map[string]any{
		"channel": "55", "fileBase64": "data:image/png;base64," + base64.StdEncoding.EncodeToString([]byte("pixel data")), "fileName": "chart.png",
	})
	invocation.Config = map[string]any{"imageSource": "base64"}
	if _, err := module.Execute(context.Background(), invocation, runtimeStub{sender: sender}); err != nil {
		t.Fatal(err)
	}
	if len(sender.request.Attachments) != 1 {
		t.Fatalf("attachments = %#v", sender.request.Attachments)
	}
	attachment := sender.request.Attachments[0]
	if attachment.Name != "chart.png" || string(attachment.Data) != "pixel data" {
		t.Fatalf("attachment = %#v", attachment)
	}
}

// A stale value left behind in a hidden field must never leak into the
// message: an explicit mode reads only its own source.
func TestImageSourceModeIgnoresOtherSources(t *testing.T) {
	sender := &senderStub{result: domain.DiscordMessageResult{MessageID: "msg-7", Sent: true}}
	module := nodes.Implementation{Metadata: definition(), Executor: execute}
	invocation := invocation(map[string]any{
		"channel": "55", "message": "hello",
		"fileUrl": "https://example.com/old.png", "filePath": "/definitely/missing.png", "fileBase64": "not-used",
	})
	invocation.Config = map[string]any{"imageSource": "bytes"}
	if _, err := module.Execute(context.Background(), invocation, runtimeStub{sender: sender}); err != nil {
		t.Fatal(err)
	}
	if len(sender.request.Attachments) != 0 {
		t.Fatalf("bytes mode must ignore other sources: %#v", sender.request.Attachments)
	}
}

func TestAttachmentFromBytesSource(t *testing.T) {
	sender := &senderStub{result: domain.DiscordMessageResult{MessageID: "msg-8", Sent: true}}
	module := nodes.Implementation{Metadata: definition(), Executor: execute}
	invocation := invocation(map[string]any{
		"channel": "55", "fileData": []byte("drawn image"), "fileName": "weather.png",
	})
	invocation.Config = map[string]any{"imageSource": "bytes"}
	if _, err := module.Execute(context.Background(), invocation, runtimeStub{sender: sender}); err != nil {
		t.Fatal(err)
	}
	if len(sender.request.Attachments) != 1 || sender.request.Attachments[0].Name != "weather.png" || string(sender.request.Attachments[0].Data) != "drawn image" {
		t.Fatalf("attachments = %#v", sender.request.Attachments)
	}
}

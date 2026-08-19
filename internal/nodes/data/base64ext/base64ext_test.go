package base64ext

import (
	"context"
	"encoding/base64"
	"testing"

	"github.com/FlameInTheDark/neuropipe/internal/nodes"
)

func TestBytesToBase64EncodesRawBytes(t *testing.T) {
	module := NewBytesToBase64()
	result, err := module.Execute(context.Background(), nodes.Invocation{
		Inputs: map[string]any{"value": []byte("hello")},
	}, nil)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if got, want := result.Outputs["result"], base64.StdEncoding.EncodeToString([]byte("hello")); got != want {
		t.Fatalf("encoded = %q, want %q", got, want)
	}
}

func TestBase64ToBytesDecodesText(t *testing.T) {
	module := NewBase64ToBytes()
	encoded := base64.StdEncoding.EncodeToString([]byte("hello"))
	result, err := module.Execute(context.Background(), nodes.Invocation{
		Inputs: map[string]any{"value": encoded},
	}, nil)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	data, ok := result.Outputs["result"].([]byte)
	if !ok {
		t.Fatalf("expected []byte, got %T", result.Outputs["result"])
	}
	if string(data) != "hello" {
		t.Fatalf("decoded = %q, want %q", string(data), "hello")
	}
}

func TestBase64ToBytesHandlesURLEncoded(t *testing.T) {
	module := NewBase64ToBytes()
	encoded := base64.URLEncoding.EncodeToString([]byte("hello"))
	result, err := module.Execute(context.Background(), nodes.Invocation{
		Inputs: map[string]any{"value": encoded},
	}, nil)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	data, ok := result.Outputs["result"].([]byte)
	if !ok {
		t.Fatalf("expected []byte, got %T", result.Outputs["result"])
	}
	if string(data) != "hello" {
		t.Fatalf("decoded URL-safe = %q, want %q", string(data), "hello")
	}
}

package runtime

import (
	"testing"

	"github.com/FlameInTheDark/neuropipe/internal/domain"
)

func TestServerArguments(t *testing.T) {
	tests := []struct {
		name    string
		mode    domain.RuntimeMode
		wantGPU bool
	}{
		{name: "cpu", mode: domain.RuntimeCPU, wantGPU: false},
		{name: "cuda", mode: domain.RuntimeCUDA, wantGPU: true},
		{name: "vulkan", mode: domain.RuntimeVulkan, wantGPU: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			args := serverArguments(domain.LlamaRuntimeSettings{ModelPath: "C:\\models\\test.gguf", ContextSize: 4096}, "9000", test.mode)
			if gotGPU := contains(args, "-ngl"); gotGPU != test.wantGPU {
				t.Fatalf("GPU argument = %v, want %v; args = %v", gotGPU, test.wantGPU, args)
			}
		})
	}
}

func contains(values []string, wanted string) bool {
	for _, value := range values {
		if value == wanted {
			return true
		}
	}
	return false
}

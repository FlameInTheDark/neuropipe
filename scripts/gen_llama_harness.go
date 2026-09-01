//go:build ignore

// gen_llama_harness extracts the pure managed-llama provider logic from
// internal/app/desktop.go into scripts/llama-harness/extracted.go. The app
// package cannot compile on the Linux dev sandbox (Wails GTK dependencies),
// so its CI tests are mirrored in scripts/llama-harness against a
// go/printer reprint of the real functions: the harness always tests the
// shipped code. Re-run after changing syncManagedLlamaModels or
// managedModelContextSizeFromProviders.
//
// Run: go run scripts/gen_llama_harness.go
package main

import (
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/printer"
	"go/token"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

const extractedPackage = "llamaharness"

var extractedNames = []string{"syncManagedLlamaModels", "managedModelContextSizeFromProviders"}

func findFunc(file *ast.File, name string) *ast.FuncDecl {
	for _, decl := range file.Decls {
		if fn, ok := decl.(*ast.FuncDecl); ok && fn.Name.Name == name {
			return fn
		}
	}
	return nil
}

// rendered returns the harness source that mirrors the given functions.
func rendered(source string) (string, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, source, nil, parser.SkipObjectResolution)
	if err != nil {
		return "", fmt.Errorf("parse %s: %w", source, err)
	}
	var out strings.Builder
	out.WriteString("package " + extractedPackage + "\n\nimport (\n\t\"slices\"\n\t\"strings\"\n\n\t\"github.com/FlameInTheDark/neuropipe/internal/domain\"\n)\n\n// managedLlamaProviderID mirrors the constant in internal/app.\nconst managedLlamaProviderID = \"llama-managed\"\n\n")
	for _, name := range extractedNames {
		fn := findFunc(file, name)
		if fn == nil {
			return "", fmt.Errorf("function %q not found in %s", name, source)
		}
		if fn.Recv != nil {
			return "", fmt.Errorf("function %q unexpectedly has a receiver; extract a free function instead", name)
		}
		if err := printer.Fprint(&out, fset, fn); err != nil {
			return "", fmt.Errorf("print %s: %w", name, err)
		}
		out.WriteString("\n\n")
	}
	canonical, err := format.Source([]byte(out.String()))
	if err != nil {
		return "", fmt.Errorf("format rendered harness: %w", err)
	}
	return string(canonical), nil
}

func main() {
	_, script, _, ok := runtime.Caller(0)
	if !ok {
		fmt.Fprintln(os.Stderr, "cannot locate this script")
		os.Exit(1)
	}
	root := filepath.Dir(filepath.Dir(script))
	source := filepath.Join(root, "internal", "app", "desktop.go")
	content, err := rendered(source)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	target := filepath.Join(root, "scripts", "llama-harness", "extracted.go")
	if err := os.WriteFile(target, []byte(content), 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "write %s: %v\n", target, err)
		os.Exit(1)
	}
	fmt.Printf("extracted %s\n    %s -> %s\n", strings.Join(extractedNames, ", "), source, target)
}

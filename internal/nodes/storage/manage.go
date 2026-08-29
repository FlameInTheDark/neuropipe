// Management nodes: delete remote entries, create folders, and move/rename.
package storage

import (
	"context"
	"fmt"

	"github.com/FlameInTheDark/neuropipe/internal/domain"
	"github.com/FlameInTheDark/neuropipe/internal/nodes"
)

func errRequired(label string) error { return fmt.Errorf("%s is required", label) }

/* ---------------- Delete File ---------------- */

func deleteDefinition() domain.NodeDefinition {
	return Definition("action:storage_delete_file", "Delete File", "Delete one remote file, or a folder with everything inside it.",
		[]domain.NodePort{
			Exec("in", "Exec", domain.PinInput),
			Text("path", "Path", domain.PinInput, true),
		},
		[]domain.NodePort{
			Exec("out", "Then", domain.PinOutput),
			Result("result", "Result", domain.PinOutput, []domain.DataField{
				{Path: "deleted", Label: "Deleted", DataType: domain.DataBoolean, Description: "Whether anything was removed."},
				{Path: "count", Label: "Count", DataType: domain.DataNumber, Description: "Number of removed entries (recursive folders count every child)."},
			}),
		},
		[]domain.ConfigField{
			{Name: "path", Label: "Path", Kind: "string", Placeholder: "reports/2026/old.csv", Required: true},
			{Name: "recursive", Label: "Delete folder contents recursively", Kind: "boolean"},
		},
		map[string]any{"path": "", "recursive": false},
	)
}

func executeDelete(ctx context.Context, invocation nodes.Invocation, runtime nodes.Runtime) (nodes.ExecutionResult, error) {
	if err := ContextCancelled(ctx); err != nil {
		return nodes.ExecutionResult{}, err
	}
	executor, id, err := Executor(invocation, runtime)
	if err != nil {
		return nodes.ExecutionResult{}, err
	}
	path, err := RequiredText(invocation, "path", "path")
	if err != nil {
		return nodes.ExecutionResult{}, err
	}
	result, err := executor.StorageDelete(ctx, domain.StorageDeleteRequest{
		StorageID: id, Path: path, Recursive: ConfigFlag(invocation, "recursive"),
	})
	if err != nil {
		return nodes.ExecutionResult{}, err
	}
	return nodes.ExecutionResult{
		Outputs: map[string]any{"result": map[string]any{"deleted": result.Deleted, "count": result.Count}},
		Ports:   []string{"out"},
	}, nil
}

/* ---------------- Create Folder ---------------- */

func makeDirDefinition() domain.NodeDefinition {
	return Definition("action:storage_create_folder", "Create Folder", "Create one folder in a registered storage (S3 folders are zero-byte markers).",
		[]domain.NodePort{
			Exec("in", "Exec", domain.PinInput),
			Text("path", "Path", domain.PinInput, true),
		},
		[]domain.NodePort{
			Exec("out", "Then", domain.PinOutput),
			Result("result", "Result", domain.PinOutput, []domain.DataField{
				{Path: "path", Label: "Path", DataType: domain.DataText, Description: "Folder path that was created."},
				{Path: "created", Label: "Created", DataType: domain.DataBoolean, Description: "Whether the folder was created."},
			}),
		},
		[]domain.ConfigField{
			{Name: "path", Label: "Path", Kind: "string", Placeholder: "reports/2026", Required: true},
		},
		map[string]any{"path": ""},
	)
}

func executeMakeDir(ctx context.Context, invocation nodes.Invocation, runtime nodes.Runtime) (nodes.ExecutionResult, error) {
	if err := ContextCancelled(ctx); err != nil {
		return nodes.ExecutionResult{}, err
	}
	executor, id, err := Executor(invocation, runtime)
	if err != nil {
		return nodes.ExecutionResult{}, err
	}
	path, err := RequiredText(invocation, "path", "path")
	if err != nil {
		return nodes.ExecutionResult{}, err
	}
	result, err := executor.StorageMakeDir(ctx, domain.StorageMakeDirRequest{StorageID: id, Path: path})
	if err != nil {
		return nodes.ExecutionResult{}, err
	}
	return nodes.ExecutionResult{
		Outputs: map[string]any{"result": map[string]any{"path": result.Path, "created": result.Created}},
		Ports:   []string{"out"},
	}, nil
}

/* ---------------- Move File ---------------- */

func moveDefinition() domain.NodeDefinition {
	return Definition("action:storage_move_file", "Move File", "Rename or move one remote file or folder inside a registered storage.",
		[]domain.NodePort{
			Exec("in", "Exec", domain.PinInput),
			Text("from", "From", domain.PinInput, true),
			Text("to", "To", domain.PinInput, true),
		},
		[]domain.NodePort{
			Exec("out", "Then", domain.PinOutput),
			Result("result", "Result", domain.PinOutput, []domain.DataField{
				{Path: "from", Label: "From", DataType: domain.DataText, Description: "Original remote path."},
				{Path: "to", Label: "To", DataType: domain.DataText, Description: "Destination remote path."},
				{Path: "moved", Label: "Moved", DataType: domain.DataBoolean, Description: "Whether the entry was moved."},
			}),
		},
		[]domain.ConfigField{
			{Name: "from", Label: "From", Kind: "string", Placeholder: "reports/2026/draft.csv", Required: true},
			{Name: "to", Label: "To", Kind: "string", Placeholder: "reports/2026/final.csv", Required: true},
		},
		map[string]any{"from": "", "to": ""},
	)
}

func executeMove(ctx context.Context, invocation nodes.Invocation, runtime nodes.Runtime) (nodes.ExecutionResult, error) {
	if err := ContextCancelled(ctx); err != nil {
		return nodes.ExecutionResult{}, err
	}
	executor, id, err := Executor(invocation, runtime)
	if err != nil {
		return nodes.ExecutionResult{}, err
	}
	from, err := RequiredText(invocation, "from", "from path")
	if err != nil {
		return nodes.ExecutionResult{}, err
	}
	to, err := RequiredText(invocation, "to", "to path")
	if err != nil {
		return nodes.ExecutionResult{}, err
	}
	result, err := executor.StorageMove(ctx, domain.StorageMoveRequest{StorageID: id, From: from, To: to})
	if err != nil {
		return nodes.ExecutionResult{}, err
	}
	return nodes.ExecutionResult{
		Outputs: map[string]any{"result": map[string]any{"from": result.From, "to": result.To, "moved": result.Moved}},
		Ports:   []string{"out"},
	}, nil
}

/* ---------------- registration ---------------- */

// Register registers every storage node.
func Register(registrar nodes.Registrar) error {
	nodesList := []nodes.Implementation{
		{Metadata: uploadFileDefinition(), Resolver: resolveUpload, Executor: executeUploadFile},
		{Metadata: downloadDefinition(), Executor: executeDownload},
		{Metadata: listDefinition(), Executor: executeList},
		{Metadata: deleteDefinition(), Executor: executeDelete},
		{Metadata: makeDirDefinition(), Executor: executeMakeDir},
		{Metadata: moveDefinition(), Executor: executeMove},
		{Metadata: presignDefinition(), Executor: executePresign},
		{Metadata: publicURLDefinition(), Executor: executePublicURL},
	}
	for _, node := range nodesList {
		if err := registrar.Register(node); err != nil {
			return err
		}
	}
	return nil
}

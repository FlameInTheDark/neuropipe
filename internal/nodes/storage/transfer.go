// Transfer nodes: upload files/data into a storage, download from it, and
// list directory contents.
package storage

import (
	"context"
	"fmt"
	"strings"

	"github.com/FlameInTheDark/neuropipe/internal/domain"
	"github.com/FlameInTheDark/neuropipe/internal/nodes"
)

/* ---------------- upload source selection ---------------- */

// Upload source modes. The values deliberately match the send-node attachment
// selector vocabulary (file / bytes / base64) so source-driven UI code stays
// uniform across node families. The empty mode is Auto: every source
// participates and the first one that is set wins, which is exactly how
// graphs saved before the dropdown behaved.
const (
	uploadSourceDisk   = "file"
	uploadSourceNode   = "bytes"
	uploadSourceBase64 = "base64"
)

// uploadSourceOptions lists the Source dropdown. Auto keeps legacy graphs
// fully functional because it is the value their config lacks.
func uploadSourceOptions() []domain.Option {
	return []domain.Option{
		{Value: "", Label: "Auto — use whatever is set"},
		{Value: uploadSourceDisk, Label: "From disk"},
		{Value: uploadSourceNode, Label: "From node"},
		{Value: uploadSourceBase64, Label: "From base64"},
	}
}

// uploadSourceMode normalises a selector value. Unknown values (including the
// send nodes' url mode) fall back to Auto so a typo can never hide every input.
func uploadSourceMode(value any) string {
	text, _ := value.(string)
	switch text {
	case uploadSourceDisk, uploadSourceNode, uploadSourceBase64:
		return text
	default:
		return ""
	}
}

// uploadSourceLabel renders one mode the way the dropdown shows it, for
// required-input error messages.
func uploadSourceLabel(mode string) string {
	switch mode {
	case uploadSourceDisk:
		return "From disk"
	case uploadSourceNode:
		return "From node"
	case uploadSourceBase64:
		return "From base64"
	default:
		return "Auto"
	}
}

/* ---------------- Upload File ---------------- */

func uploadFileDefinition() domain.NodeDefinition {
	return Definition("action:storage_upload_file", "Upload File", "Upload a file from disk, raw bytes from another node, or base64 text into a registered S3 or FTP storage.",
		[]domain.NodePort{
			Exec("in", "Exec", domain.PinInput),
			Text("localPath", "Local path", domain.PinInput, false),
			Bytes("data", "Data", domain.PinInput, false),
			Text("base64", "Base64", domain.PinInput, false),
			Text("remotePath", "Remote path", domain.PinInput, true),
			Text("contentType", "Content type", domain.PinInput, false),
		},
		[]domain.NodePort{
			Exec("out", "Then", domain.PinOutput),
			Result("result", "Result", domain.PinOutput, []domain.DataField{
				{Path: "key", Label: "Key", DataType: domain.DataText, Description: "Remote path the file was stored under."},
				{Path: "size", Label: "Size", DataType: domain.DataNumber, Description: "Number of bytes uploaded."},
				{Path: "driver", Label: "Driver", DataType: domain.DataText, Description: "Storage driver that handled the upload (s3 or ftp)."},
			}),
		},
		[]domain.ConfigField{
			{Name: "source", Label: "Source", Kind: "select", Options: uploadSourceOptions()},
			{Name: "localPath", Label: "Local path", Kind: "string", Placeholder: "/home/user/pictures/chart.png", VisibleWhen: "source=" + uploadSourceDisk + "|source="},
			{Name: "base64", Label: "Base64", Kind: "textarea", Placeholder: "aGVsbG8gd29ybGQ= or a data: URL", VisibleWhen: "source=" + uploadSourceBase64 + "|source="},
			{Name: "remotePath", Label: "Remote path", Kind: "string", Placeholder: "reports/2026/chart.png (trailing / keeps the file name)", Required: true},
			{Name: "contentType", Label: "Content type", Kind: "string", Placeholder: "image/png"},
		},
		map[string]any{"source": "", "localPath": "", "base64": "", "remotePath": "", "contentType": ""},
	)
}

// resolveUpload keeps only the source pins the selected upload mode uses.
// Auto keeps every pin so graphs saved before the selector keep their wires.
func resolveUpload(node domain.FlowNode) (domain.NodeDefinition, error) {
	definition := uploadFileDefinition()
	resolved := definition
	mode := uploadSourceMode(configValue(node, "source"))
	resolved.Inputs = filterUploadPins(definition.Inputs, mode)
	return resolved, nil
}

func filterUploadPins(inputs []domain.NodePort, mode string) []domain.NodePort {
	if mode == "" {
		return inputs
	}
	kept := make([]domain.NodePort, 0, len(inputs))
	for _, port := range inputs {
		if !uploadPinVisible(port.ID, mode) {
			continue
		}
		kept = append(kept, port)
	}
	return kept
}

func uploadPinVisible(pinID, mode string) bool {
	switch pinID {
	case "localPath":
		return mode == uploadSourceDisk
	case "data":
		return mode == uploadSourceNode
	case "base64":
		return mode == uploadSourceBase64
	default:
		return true
	}
}

func executeUploadFile(ctx context.Context, invocation nodes.Invocation, runtime nodes.Runtime) (nodes.ExecutionResult, error) {
	if err := ContextCancelled(ctx); err != nil {
		return nodes.ExecutionResult{}, err
	}
	executor, id, err := Executor(invocation, runtime)
	if err != nil {
		return nodes.ExecutionResult{}, err
	}
	remotePath, err := RequiredText(invocation, "remotePath", "remote path")
	if err != nil {
		return nodes.ExecutionResult{}, err
	}
	contentType := String(invocation, "contentType")
	result, err := uploadBySource(ctx, executor, id, invocation, remotePath, contentType)
	if err != nil {
		return nodes.ExecutionResult{}, err
	}
	return nodes.ExecutionResult{
		Outputs: map[string]any{"result": map[string]any{"key": result.Key, "size": result.Size, "driver": result.Driver}},
		Ports:   []string{"out"},
	}, nil
}

// uploadBySource performs the upload the Source dropdown selected. Auto
// prefers streaming a disk file, then bytes from another node, then base64
// text; an explicit mode reads only its own source so stale hidden values can
// never leak into the request.
func uploadBySource(ctx context.Context, executor nodes.StorageExecutor, id string, invocation nodes.Invocation, remotePath, contentType string) (domain.StorageUploadResult, error) {
	mode := uploadSourceMode(invocation.Config["source"])
	uploadFile := func() (domain.StorageUploadResult, error) {
		localPath := strings.TrimSpace(String(invocation, "localPath"))
		if localPath == "" {
			return domain.StorageUploadResult{}, fmt.Errorf("local path is required when the upload source is %s", uploadSourceLabel(mode))
		}
		return executor.StorageUploadFile(ctx, domain.StorageUploadFileRequest{
			StorageID: id, LocalPath: localPath, RemotePath: remotePath, ContentType: contentType,
		})
	}
	uploadData := func(data []byte, label string) (domain.StorageUploadResult, error) {
		if len(data) == 0 {
			return domain.StorageUploadResult{}, fmt.Errorf("%s is required when the upload source is %s", label, uploadSourceLabel(mode))
		}
		return executor.StorageUploadData(ctx, domain.StorageUploadDataRequest{
			StorageID: id, Data: data, RemotePath: remotePath, ContentType: contentType,
		})
	}

	switch mode {
	case uploadSourceDisk:
		return uploadFile()
	case uploadSourceNode:
		data, err := BytesValue(invocation, "data")
		if err != nil {
			return domain.StorageUploadResult{}, err
		}
		return uploadData(data, "data")
	case uploadSourceBase64:
		data, err := Base64Text(String(invocation, "base64"))
		if err != nil {
			return domain.StorageUploadResult{}, err
		}
		return uploadData(data, "base64")
	default: // Auto: disk file, then node bytes, then base64
		if strings.TrimSpace(String(invocation, "localPath")) != "" {
			return uploadFile()
		}
		if invocation.Inputs["data"] != nil {
			data, err := BytesValue(invocation, "data")
			if err != nil {
				return domain.StorageUploadResult{}, err
			}
			if len(data) > 0 {
				return uploadData(data, "data")
			}
		}
		if strings.TrimSpace(String(invocation, "base64")) != "" {
			data, err := Base64Text(String(invocation, "base64"))
			if err != nil {
				return domain.StorageUploadResult{}, err
			}
			return uploadData(data, "base64")
		}
		return domain.StorageUploadResult{}, fmt.Errorf("a local path, data, or base64 is required")
	}
}

/* ---------------- Download File ---------------- */

func downloadDefinition() domain.NodeDefinition {
	return Definition("action:storage_download_file", "Download File", "Stream one remote file from a registered storage to a local path.",
		[]domain.NodePort{
			Exec("in", "Exec", domain.PinInput),
			Text("remotePath", "Remote path", domain.PinInput, true),
			Text("localPath", "Local path", domain.PinInput, true),
		},
		[]domain.NodePort{
			Exec("out", "Then", domain.PinOutput),
			Result("result", "Result", domain.PinOutput, []domain.DataField{
				{Path: "path", Label: "Path", DataType: domain.DataText, Description: "Local path the file was written to."},
				{Path: "name", Label: "Name", DataType: domain.DataText, Description: "Remote file name."},
				{Path: "bytes", Label: "Bytes", DataType: domain.DataNumber, Description: "Number of bytes downloaded."},
			}),
		},
		[]domain.ConfigField{
			{Name: "remotePath", Label: "Remote path", Kind: "string", Placeholder: "reports/2026/chart.png", Required: true},
			{Name: "localPath", Label: "Local path", Kind: "string", Placeholder: "/home/user/downloads/chart.png (trailing / keeps the file name)", Required: true},
		},
		map[string]any{"remotePath": "", "localPath": ""},
	)
}

func executeDownload(ctx context.Context, invocation nodes.Invocation, runtime nodes.Runtime) (nodes.ExecutionResult, error) {
	if err := ContextCancelled(ctx); err != nil {
		return nodes.ExecutionResult{}, err
	}
	executor, id, err := Executor(invocation, runtime)
	if err != nil {
		return nodes.ExecutionResult{}, err
	}
	remotePath, err := RequiredText(invocation, "remotePath", "remote path")
	if err != nil {
		return nodes.ExecutionResult{}, err
	}
	localPath, err := RequiredText(invocation, "localPath", "local path")
	if err != nil {
		return nodes.ExecutionResult{}, err
	}
	result, err := executor.StorageDownloadFile(ctx, domain.StorageDownloadRequest{
		StorageID: id, RemotePath: remotePath, LocalPath: localPath,
	})
	if err != nil {
		return nodes.ExecutionResult{}, err
	}
	return nodes.ExecutionResult{
		Outputs: map[string]any{"result": map[string]any{"path": result.Path, "name": result.Name, "bytes": result.Bytes}},
		Ports:   []string{"out"},
	}, nil
}

/* ---------------- List Files ---------------- */

func listDefinition() domain.NodeDefinition {
	return Definition("action:storage_list_files", "List Files", "List the direct children of one folder in a registered storage.",
		[]domain.NodePort{
			Exec("in", "Exec", domain.PinInput),
			Text("path", "Path", domain.PinInput, false),
		},
		[]domain.NodePort{
			Exec("out", "Then", domain.PinOutput),
			Entries("entries", "Entries", domain.PinOutput),
			Number("count", "Count", domain.PinOutput, false),
		},
		[]domain.ConfigField{
			{Name: "path", Label: "Path", Kind: "string", Placeholder: "reports/2026 (empty lists the root)"},
		},
		map[string]any{"path": ""},
	)
}

func executeList(ctx context.Context, invocation nodes.Invocation, runtime nodes.Runtime) (nodes.ExecutionResult, error) {
	if err := ContextCancelled(ctx); err != nil {
		return nodes.ExecutionResult{}, err
	}
	executor, id, err := Executor(invocation, runtime)
	if err != nil {
		return nodes.ExecutionResult{}, err
	}
	result, err := executor.StorageListFiles(ctx, domain.StorageListRequest{StorageID: id, Path: String(invocation, "path")})
	if err != nil {
		return nodes.ExecutionResult{}, err
	}
	return nodes.ExecutionResult{
		Outputs: map[string]any{"entries": EntryList(result.Entries), "count": len(result.Entries)},
		Ports:   []string{"out"},
	}, nil
}

package remoteexec

import (
	"time"

	"github.com/FlameInTheDark/neuropipe/internal/domain"
	executorv1 "github.com/FlameInTheDark/neuropipe/internal/proto/executor/v1"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// ExecutionFromSnapshot converts an executor run snapshot into the desktop
// execution record shape. StartedAt falls back to the wall clock so records
// always carry a monotonic ordering field.
func ExecutionFromSnapshot(snapshot *executorv1.RunSnapshot, executorID string) domain.Execution {
	execution := domain.Execution{}
	if snapshot == nil {
		return execution
	}
	execution.ID = snapshot.GetExecutionId()
	execution.PipelineID = snapshot.GetPipelineId()
	execution.TriggerID = snapshot.GetTriggerBindingId()
	execution.Status = domain.RunStatus(snapshot.GetStatus())
	execution.Error = snapshot.GetError()
	execution.ExecutorID = executorID
	if stamp := snapshot.GetStartedAt(); stamp != nil {
		execution.StartedAt = stamp.AsTime()
	} else {
		execution.StartedAt = time.Now().UTC()
	}
	if stamp := snapshot.GetRunStartedAt(); stamp != nil {
		started := stamp.AsTime()
		execution.RunStartedAt = &started
	}
	if stamp := snapshot.GetFinishedAt(); stamp != nil {
		finished := stamp.AsTime()
		execution.FinishedAt = &finished
	}
	for _, nodeRun := range snapshot.GetNodeRuns() {
		execution.NodeRuns = append(execution.NodeRuns, NodeRunFromProto(nodeRun))
	}
	return execution
}

// NodeRunFromProto converts one node-run entry.
func NodeRunFromProto(nodeRun *executorv1.NodeRun) domain.NodeRun {
	result := domain.NodeRun{
		NodeID:           nodeRun.GetNodeId(),
		NodeType:         nodeRun.GetNodeType(),
		ParentNodeID:     nodeRun.GetParentNodeId(),
		FunctionID:       nodeRun.GetFunctionId(),
		FunctionRevision: int(nodeRun.GetFunctionRevision()),
		Status:           domain.RunStatus(nodeRun.GetStatus()),
		Error:            nodeRun.GetError(),
	}
	if stamp := nodeRun.GetStartedAt(); stamp != nil {
		result.StartedAt = stamp.AsTime()
	} else {
		result.StartedAt = time.Now().UTC()
	}
	if stamp := nodeRun.GetFinishedAt(); stamp != nil {
		result.FinishedAt = stamp.AsTime()
	}
	return result
}

// NodeRunToProto serialises a redacted desktop-side or executor-side node
// run. Packet input/output previews are intentionally not transported.
func NodeRunToProto(nodeRun domain.NodeRun) *executorv1.NodeRun {
	result := &executorv1.NodeRun{
		NodeId:           nodeRun.NodeID,
		NodeType:         nodeRun.NodeType,
		ParentNodeId:     nodeRun.ParentNodeID,
		FunctionId:       nodeRun.FunctionID,
		FunctionRevision: int32(nodeRun.FunctionRevision),
		Status:           string(nodeRun.Status),
		Error:            nodeRun.Error,
		StartedAt:        timestamppb.New(TimeOrNow(nodeRun.StartedAt)),
	}
	if !nodeRun.FinishedAt.IsZero() {
		result.FinishedAt = timestamppb.New(nodeRun.FinishedAt)
	}
	return result
}

// TimestampOrNull converts a time into a protobuf timestamp, tolerating zero.
func TimestampOrNull(value time.Time) *timestamppb.Timestamp {
	if value.IsZero() {
		return nil
	}
	return timestamppb.New(value)
}

// TimeOrNow normalises a possibly-zero stored timestamp for wire output.
func TimeOrNow(value time.Time) time.Time {
	if value.IsZero() {
		return time.Now().UTC()
	}
	return value
}

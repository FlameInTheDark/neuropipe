package app

import (
	"context"
	"strings"

	"github.com/FlameInTheDark/neuropipe/internal/chat"
	"github.com/FlameInTheDark/neuropipe/internal/domain"
)

// chatAuthoring adapts Desktop orchestration (validation, persistence,
// publish side-effects such as schedule reloads and executor deployment) to
// the assistant's authoring tools. Package chat defines the interface; this
// type keeps it satisfied without creating an import cycle.
type chatAuthoring struct {
	desktop *Desktop
}

func (a chatAuthoring) ValidatePipeline(def domain.FlowDefinition) error {
	return pipelineValidate(def, a.desktop.registry)
}

func (a chatAuthoring) CreatePipelineDraft(ctx context.Context, name, description string, def domain.FlowDefinition) (domain.Pipeline, error) {
	created, err := a.desktop.store.CreatePipeline(a.desktop.context(), name, "", def)
	if err != nil {
		return domain.Pipeline{}, err
	}
	created.Description = strings.TrimSpace(description)
	return a.desktop.store.SaveDraft(a.desktop.context(), created)
}

func (a chatAuthoring) SavePipelineDraft(ctx context.Context, id, name, description string, def domain.FlowDefinition) (domain.Pipeline, error) {
	current, err := a.desktop.store.GetPipeline(a.desktop.context(), id)
	if err != nil {
		return domain.Pipeline{}, err
	}
	if strings.TrimSpace(name) != "" {
		current.Name = strings.TrimSpace(name)
	}
	current.Description = strings.TrimSpace(description)
	current.DraftDefinition = def
	return a.desktop.store.SaveDraft(a.desktop.context(), current)
}

func (a chatAuthoring) GetPipelineFull(ctx context.Context, id string) (domain.Pipeline, error) {
	return a.desktop.store.GetPipeline(a.desktop.context(), id)
}

func (a chatAuthoring) PublishPipeline(ctx context.Context, id string) (domain.Pipeline, error) {
	current, err := a.desktop.store.GetPipeline(a.desktop.context(), id)
	if err != nil {
		return domain.Pipeline{}, err
	}
	return a.desktop.PublishPipeline(current)
}

func (a chatAuthoring) DeletePipeline(ctx context.Context, id string) error {
	return a.desktop.DeletePipeline(id)
}

func (a chatAuthoring) ValidateFunction(fn domain.CustomFunction) error {
	return validateFunction(fn, a.desktop.registry)
}

func (a chatAuthoring) SaveFunctionDraft(ctx context.Context, fn domain.CustomFunction) (domain.CustomFunction, error) {
	return a.desktop.SaveFunction(fn)
}

func (a chatAuthoring) GetFunction(ctx context.Context, id string) (domain.CustomFunction, error) {
	return a.desktop.GetFunction(id)
}

func (a chatAuthoring) ListFunctions(ctx context.Context) ([]domain.FunctionSummary, error) {
	return a.desktop.ListFunctions()
}

func (a chatAuthoring) PublishFunction(ctx context.Context, fn domain.CustomFunction) (domain.CustomFunction, error) {
	return a.desktop.PublishFunction(fn)
}

func (a chatAuthoring) DeleteFunction(ctx context.Context, id string) error {
	return a.desktop.DeleteFunction(id)
}

var _ chat.Authoring = chatAuthoring{}

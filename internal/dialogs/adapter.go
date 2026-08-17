// Package dialogs provides native window dialogs for Blueprint nodes that
// must interact with the user mid-execution. This file contains the adapter
// that exposes a Service through the focused contracts declared in
// internal/nodes, so node modules never import the dialog implementation.
package dialogs

import (
	"context"
	"errors"

	"github.com/FlameInTheDark/neuropipe/internal/nodes"
)

// OpenerAdapter exposes a Service as the focused DialogOpener contract
// consumed by Display Message and Display Question nodes.
type OpenerAdapter struct {
	service *Service
}

// NewOpenerAdapter wraps a Service so it satisfies nodes.DialogOpener.
func NewOpenerAdapter(service *Service) *OpenerAdapter {
	return &OpenerAdapter{service: service}
}

// ShowMessage opens a native Wails v3 information dialog with a single OK
// button.
func (adapter *OpenerAdapter) ShowMessage(ctx context.Context, title, message string) error {
	if adapter == nil || adapter.service == nil {
		return errors.New("dialog opener is unavailable")
	}
	return adapter.service.ShowMessage(ctx, title, message)
}

// ShowQuestion opens a native Yes/No dialog and reports the user's choice.
func (adapter *OpenerAdapter) ShowQuestion(ctx context.Context, title, message string) (nodes.DialogChoice, error) {
	if adapter == nil || adapter.service == nil {
		return nodes.DialogCancel, errors.New("dialog opener is unavailable")
	}
	result, err := adapter.service.ShowQuestion(ctx, title, message)
	if err != nil {
		return nodes.DialogCancel, err
	}
	switch result {
	case QuestionYes:
		return nodes.DialogYes, nil
	case QuestionNo:
		return nodes.DialogNo, nil
	default:
		return nodes.DialogCancel, nil
	}
}

// Compile-time contract assertions keep the adapter honest.
var (
	_ nodes.DialogOpener      = (*OpenerAdapter)(nil)
	_ nodes.InputDialogOpener = (*InputAdapter)(nil)
	_ nodes.FormDialogOpener  = (*FormAdapter)(nil)
)

// FormAdapter exposes a Service as the focused FormDialogOpener contract
// consumed by the Form node.
type FormAdapter struct {
	service *Service
}

// NewFormAdapter wraps a Service so it satisfies nodes.FormDialogOpener.
func NewFormAdapter(service *Service) *FormAdapter {
	return &FormAdapter{service: service}
}

// ShowForm emits a styled form dialog request to the React layer and waits
// for the user's response.
func (adapter *FormAdapter) ShowForm(ctx context.Context, request nodes.FormRequest) (nodes.FormResponse, error) {
	if adapter == nil || adapter.service == nil {
		return nodes.FormResponse{Canceled: true}, errors.New("form dialog opener is unavailable")
	}
	fields := make([]FormDialogField, 0, len(request.Items))
	for _, item := range request.Items {
		field := FormDialogField{
			ID: item.ID, Kind: item.Kind, Label: item.Label,
			Col: item.Col, Row: item.Row, Span: item.Span, RowSpan: item.RowSpan,
			InputType: item.InputType, Placeholder: item.Placeholder,
		}
		for _, opt := range item.Options {
			field.Options = append(field.Options, FormDialogOption{Value: opt.Value, Label: opt.Label})
		}
		fields = append(fields, field)
	}
	resp, err := adapter.service.ShowForm(ctx, FormDialogRequest{
		ID: request.ID, Title: request.Title, Message: request.Message,
		Continue: request.Continue, Cancel: request.Cancel, Items: fields,
	})
	if err != nil {
		return nodes.FormResponse{Canceled: true}, err
	}
	return nodes.FormResponse{Canceled: resp.Canceled, Values: resp.Values}, nil
}

// InputAdapter exposes a Service as the focused InputDialogOpener contract
// consumed by the Display Input Dialog node.
type InputAdapter struct {
	service *Service
}

// NewInputAdapter wraps a Service so it satisfies nodes.InputDialogOpener.
func NewInputAdapter(service *Service) *InputAdapter {
	return &InputAdapter{service: service}
}

// ShowInput emits a styled input dialog request to the React layer and waits
// for the user's response.
func (adapter *InputAdapter) ShowInput(ctx context.Context, request nodes.InputRequest) (nodes.InputResponse, error) {
	if adapter == nil || adapter.service == nil {
		return nodes.InputResponse{Canceled: true}, errors.New("input dialog opener is unavailable")
	}
	response, err := adapter.service.ShowInput(ctx, InputRequest{
		ID:          request.ID,
		Title:       request.Title,
		Message:     request.Message,
		Label:       request.Label,
		InputType:   request.InputType,
		Continue:    request.Continue,
		Cancel:      request.Cancel,
		Placeholder: request.Placeholder,
	})
	if err != nil {
		return nodes.InputResponse{Canceled: true}, err
	}
	return nodes.InputResponse{Canceled: response.Canceled, Value: response.Value}, nil
}

// Package httpapi hosts Neuropipe's optional local HTTP and webhook API.
package httpapi

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/FlameInTheDark/neuropipe/internal/domain"
	"github.com/FlameInTheDark/neuropipe/internal/llm"
	"github.com/gofiber/fiber/v3"
)

// SecretResolver keeps API-token resolution in the backend vault.
type SecretResolver interface {
	Get(name string) (string, error)
}

// Workspace is the narrow application contract used by Fiber handlers. It is
// intentionally independent of Wails so the HTTP API is testable in-process.
type Workspace interface {
	ListPipelines() ([]domain.PipelineSummary, error)
	ListFunctions() ([]domain.FunctionSummary, error)
	ListAllTriggers() ([]domain.TriggerBinding, error)
	ListSchedules() ([]domain.TriggerBinding, error)
	ListReports() ([]domain.Report, error)
	ListRecentExecutions(limit int) ([]domain.Execution, error)
	GetExecution(id string) (domain.Execution, error)
	GetMetricsOverview(filter domain.MetricsFilter) (domain.MetricsOverview, error)
	RecordHTTPMetric(event domain.MetricActivityEvent) error
	StartPublishedPipeline(pipelineID, triggerNodeID string, input map[string]any) (domain.Execution, error)
	HandleWebhook(path string, body []byte, signature string) (domain.Execution, error)
	GetSettings() domain.Settings
	SaveSettings(settings domain.Settings) error
	GetLlamaRuntimeStatus() domain.LlamaRuntimeStatus
	StartLlamaRuntime() (domain.LlamaRuntimeStatus, error)
	StopLlamaRuntime() domain.LlamaRuntimeStatus
	ListInstalledLlamaModels() ([]domain.LocalModel, error)
	SelectInstalledLlamaModel(path string) error
	SearchLlamaModels(request domain.ModelSearchRequest) ([]domain.ModelSearchResult, error)
	GetLlamaModelDetail(repository string) (domain.ModelDetail, error)
	ListProviderModels(providerID string) ([]llm.ModelInfo, error)
}

// Server owns the Fiber listener and can be safely reconfigured from Settings.
type Server struct {
	workspace Workspace
	vault     SecretResolver

	mu       sync.RWMutex
	app      *fiber.App
	listener net.Listener
	config   domain.APISettings
	message  string
	wg       sync.WaitGroup
}

// New creates a disabled HTTP API server.
func New(workspace Workspace, vault SecretResolver) *Server {
	return &Server{workspace: workspace, vault: vault}
}

// Configure starts, stops, or replaces the listener to match persisted settings.
func (s *Server) Configure(ctx context.Context, config domain.APISettings) error {
	config = normalizeConfig(config)
	s.mu.RLock()
	unchanged := s.app != nil && sameConfig(s.config, config)
	s.mu.RUnlock()
	if unchanged {
		return nil
	}
	if err := s.Stop(ctx); err != nil {
		return err
	}
	if !config.Enabled {
		s.mu.Lock()
		s.config, s.message = config, "API is disabled"
		s.mu.Unlock()
		return nil
	}
	listener, err := net.Listen("tcp", net.JoinHostPort(config.BindAddress, strconv.Itoa(config.Port)))
	if err != nil {
		return fmt.Errorf("listen on Neuropipe API address: %w", err)
	}
	app := s.buildApp(config)
	s.mu.Lock()
	s.app, s.listener, s.config, s.message = app, listener, config, ""
	s.wg.Add(1)
	s.mu.Unlock()
	go func() {
		defer s.wg.Done()
		if err := app.Listener(listener, fiber.ListenConfig{DisableStartupMessage: true}); err != nil && !errors.Is(err, net.ErrClosed) {
			s.mu.Lock()
			if s.app == app {
				s.message = err.Error()
			}
			s.mu.Unlock()
		}
	}()
	return nil
}

// Stop shuts down the current listener and waits for its owned serving goroutine.
func (s *Server) Stop(ctx context.Context) error {
	s.mu.Lock()
	app, listener := s.app, s.listener
	s.app, s.listener = nil, nil
	s.mu.Unlock()
	if app == nil {
		return nil
	}
	shutdownCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	err := app.ShutdownWithContext(shutdownCtx)
	if listener != nil {
		_ = listener.Close()
	}
	s.wg.Wait()
	if err != nil && !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded) {
		return fmt.Errorf("stop Neuropipe API: %w", err)
	}
	return nil
}

// Status returns renderer-safe listener state.
func (s *Server) Status() domain.APIStatus {
	s.mu.RLock()
	config, running, message := s.config, s.app != nil, s.message
	s.mu.RUnlock()
	status := domain.APIStatus{Running: running, Message: message}
	if running {
		status.Endpoint = "http://" + net.JoinHostPort(config.BindAddress, strconv.Itoa(config.Port))
	}
	if config.AuthMode == domain.APIAuthToken && strings.TrimSpace(config.TokenRef) != "" {
		_, err := s.vault.Get(config.TokenRef)
		status.TokenConfigured = err == nil
	}
	return status
}

func (s *Server) buildApp(config domain.APISettings) *fiber.App {
	app := fiber.New(fiber.Config{
		AppName:      "Neuropipe API",
		BodyLimit:    4 * 1024 * 1024,
		ErrorHandler: errorHandler,
	})
	app.Use(s.cors(config))
	app.Use(s.metricsMiddleware())
	app.Get("/health", func(c fiber.Ctx) error { return c.JSON(fiber.Map{"status": "ok"}) })
	app.Get("/openapi.json", func(c fiber.Ctx) error { return c.JSON(openAPI()) })
	app.Post("/hooks/*", s.webhook)

	v1 := app.Group("/v1", s.authenticate(config))
	v1.Get("/pipelines", s.listPipelines)
	v1.Get("/functions", s.listFunctions)
	v1.Get("/triggers", s.listTriggers)
	v1.Get("/schedules", s.listSchedules)
	v1.Get("/reports", s.listReports)
	v1.Get("/executions", s.listExecutions)
	v1.Get("/executions/:id", s.getExecution)
	v1.Post("/pipelines/:id/runs", s.runPipeline)
	admin := v1.Group("/admin", s.authorizeAdmin(config))
	admin.Get("/settings", s.getSettings)
	admin.Put("/settings", s.saveSettings)
	admin.Get("/runtime", s.runtimeStatus)
	admin.Post("/runtime/start", s.startRuntime)
	admin.Post("/runtime/stop", s.stopRuntime)
	admin.Get("/models/installed", s.installedModels)
	admin.Put("/models/selected", s.selectModel)
	admin.Get("/models", s.searchModels)
	admin.Get("/models/:repository/*", s.modelDetail)
	admin.Get("/providers/:id/models", s.providerModels)
	admin.Get("/metrics/overview", s.metricsOverview)
	return app
}

func (s *Server) metricsMiddleware() fiber.Handler {
	return func(c fiber.Ctx) error {
		started := time.Now()
		err := c.Next()
		status := c.Response().StatusCode()
		if err != nil && status < 400 {
			status = fiber.StatusInternalServerError
		}
		outcome := "success"
		if status >= 500 {
			outcome = "server_error"
		} else if status >= 400 {
			outcome = "client_error"
		}
		_ = s.workspace.RecordHTTPMetric(domain.MetricActivityEvent{Kind: "api." + metricRoute(c.Path()), Outcome: outcome, DurationMS: float64(time.Since(started)) / float64(time.Millisecond), OccurredAt: time.Now().UTC()})
		return err
	}
}

func metricRoute(path string) string {
	switch {
	case strings.HasPrefix(path, "/hooks/"):
		return "webhook"
	case strings.HasPrefix(path, "/v1/admin/metrics"):
		return "metrics"
	case strings.HasPrefix(path, "/v1/admin/"):
		return "admin"
	case strings.HasPrefix(path, "/v1/pipelines"):
		return "pipelines"
	case strings.HasPrefix(path, "/v1/executions"):
		return "executions"
	case strings.HasPrefix(path, "/v1/"):
		return "read"
	default:
		return "system"
	}
}

func (s *Server) authenticate(config domain.APISettings) fiber.Handler {
	return func(c fiber.Ctx) error {
		if config.AuthMode == domain.APIAuthNone {
			return c.Next()
		}
		token, err := s.vault.Get(config.TokenRef)
		if err != nil || !validBearer(c.Get("Authorization"), token) {
			return fiber.NewError(fiber.StatusUnauthorized, "a valid bearer token is required")
		}
		return c.Next()
	}
}

func (s *Server) authorizeAdmin(config domain.APISettings) fiber.Handler {
	return func(c fiber.Ctx) error {
		if config.AuthMode != domain.APIAuthToken || !config.AdminEnabled {
			return fiber.NewError(fiber.StatusForbidden, "the administrative API is disabled")
		}
		return c.Next()
	}
}

func (s *Server) cors(config domain.APISettings) fiber.Handler {
	allowed := make(map[string]struct{}, len(config.AllowedOrigins))
	for _, origin := range config.AllowedOrigins {
		allowed[origin] = struct{}{}
	}
	return func(c fiber.Ctx) error {
		origin := c.Get("Origin")
		if origin != "" {
			if _, ok := allowed[origin]; ok {
				c.Set("Access-Control-Allow-Origin", origin)
				c.Set("Vary", "Origin")
				c.Set("Access-Control-Allow-Headers", "Authorization, Content-Type, X-Neuropipe-Signature")
				c.Set("Access-Control-Allow-Methods", "GET, POST, PUT, OPTIONS")
			}
		}
		if c.Method() == fiber.MethodOptions {
			return c.SendStatus(fiber.StatusNoContent)
		}
		return c.Next()
	}
}

func (s *Server) listPipelines(c fiber.Ctx) error {
	value, err := s.workspace.ListPipelines()
	return respond(c, value, err)
}
func (s *Server) listFunctions(c fiber.Ctx) error {
	value, err := s.workspace.ListFunctions()
	return respond(c, value, err)
}
func (s *Server) listTriggers(c fiber.Ctx) error {
	value, err := s.workspace.ListAllTriggers()
	return respond(c, value, err)
}
func (s *Server) listSchedules(c fiber.Ctx) error {
	value, err := s.workspace.ListSchedules()
	return respond(c, value, err)
}
func (s *Server) listReports(c fiber.Ctx) error {
	value, err := s.workspace.ListReports()
	return respond(c, value, err)
}

func (s *Server) listExecutions(c fiber.Ctx) error {
	limit, _ := strconv.Atoi(c.Query("limit", "20"))
	value, err := s.workspace.ListRecentExecutions(limit)
	return respond(c, value, err)
}

func (s *Server) getExecution(c fiber.Ctx) error {
	value, err := s.workspace.GetExecution(c.Params("id"))
	return respond(c, value, err)
}

func (s *Server) runPipeline(c fiber.Ctx) error {
	var request struct {
		TriggerNodeID string         `json:"triggerNodeId"`
		Input         map[string]any `json:"input"`
	}
	if err := c.Bind().Body(&request); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "request body must be JSON")
	}
	if request.Input == nil {
		request.Input = map[string]any{}
	}
	execution, err := s.workspace.StartPublishedPipeline(c.Params("id"), request.TriggerNodeID, request.Input)
	if err != nil {
		return err
	}
	return c.Status(fiber.StatusAccepted).JSON(execution)
}

func (s *Server) webhook(c fiber.Ctx) error {
	path := "/" + strings.TrimPrefix(c.Params("*"), "/")
	execution, err := s.workspace.HandleWebhook(path, append([]byte(nil), c.Body()...), c.Get("X-Neuropipe-Signature"))
	if err != nil {
		return err
	}
	return c.Status(fiber.StatusAccepted).JSON(execution)
}

func (s *Server) getSettings(c fiber.Ctx) error {
	return c.JSON(publicSettings(s.workspace.GetSettings()))
}

func (s *Server) saveSettings(c fiber.Ctx) error {
	current := s.workspace.GetSettings()
	var next domain.Settings
	if err := c.Bind().Body(&next); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "request body must be JSON")
	}
	// HTTP administration can never attach or replace secret references.
	next.API.TokenRef = current.API.TokenRef
	for index := range next.Providers {
		for _, existing := range current.Providers {
			if next.Providers[index].ID == existing.ID {
				next.Providers[index].APIKeyRef = existing.APIKeyRef
			}
		}
	}
	if err := s.workspace.SaveSettings(next); err != nil {
		return err
	}
	return c.JSON(publicSettings(s.workspace.GetSettings()))
}

func (s *Server) runtimeStatus(c fiber.Ctx) error { return c.JSON(s.workspace.GetLlamaRuntimeStatus()) }
func (s *Server) startRuntime(c fiber.Ctx) error {
	value, err := s.workspace.StartLlamaRuntime()
	return respond(c, value, err)
}
func (s *Server) stopRuntime(c fiber.Ctx) error { return c.JSON(s.workspace.StopLlamaRuntime()) }
func (s *Server) installedModels(c fiber.Ctx) error {
	value, err := s.workspace.ListInstalledLlamaModels()
	return respond(c, value, err)
}

func (s *Server) selectModel(c fiber.Ctx) error {
	var request struct {
		Path string `json:"path"`
	}
	if err := c.Bind().Body(&request); err != nil || strings.TrimSpace(request.Path) == "" {
		return fiber.NewError(fiber.StatusBadRequest, "an installed model path is required")
	}
	if err := s.workspace.SelectInstalledLlamaModel(request.Path); err != nil {
		return err
	}
	return c.SendStatus(fiber.StatusNoContent)
}

func (s *Server) searchModels(c fiber.Ctx) error {
	value, err := s.workspace.SearchLlamaModels(domain.ModelSearchRequest{Query: c.Query("q"), Sort: c.Query("sort", "recommended")})
	return respond(c, value, err)
}

func (s *Server) modelDetail(c fiber.Ctx) error {
	repository := c.Params("repository") + "/" + strings.Trim(c.Params("*"), "/")
	value, err := s.workspace.GetLlamaModelDetail(repository)
	return respond(c, value, err)
}

func (s *Server) providerModels(c fiber.Ctx) error {
	value, err := s.workspace.ListProviderModels(c.Params("id"))
	return respond(c, value, err)
}

func (s *Server) metricsOverview(c fiber.Ctx) error {
	filter := domain.MetricsFilter{PipelineIDs: splitQuery(c.Query("pipeline")), ProviderIDs: splitQuery(c.Query("provider")), Models: splitQuery(c.Query("model")), TriggerKinds: triggerKinds(splitQuery(c.Query("trigger"))), Statuses: runStatuses(splitQuery(c.Query("status")))}
	if from := strings.TrimSpace(c.Query("from")); from != "" {
		value, err := time.Parse(time.RFC3339, from)
		if err != nil {
			return fiber.NewError(fiber.StatusBadRequest, "from must be RFC3339")
		}
		filter.From = value
	}
	if to := strings.TrimSpace(c.Query("to")); to != "" {
		value, err := time.Parse(time.RFC3339, to)
		if err != nil {
			return fiber.NewError(fiber.StatusBadRequest, "to must be RFC3339")
		}
		filter.To = value
	}
	value, err := s.workspace.GetMetricsOverview(filter)
	return respond(c, value, err)
}

func splitQuery(value string) []string {
	parts := strings.Split(value, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		if normalized := strings.TrimSpace(part); normalized != "" {
			result = append(result, normalized)
		}
	}
	return result
}

func triggerKinds(values []string) []domain.TriggerKind {
	result := make([]domain.TriggerKind, 0, len(values))
	for _, value := range values {
		result = append(result, domain.TriggerKind(value))
	}
	return result
}

func runStatuses(values []string) []domain.RunStatus {
	result := make([]domain.RunStatus, 0, len(values))
	for _, value := range values {
		result = append(result, domain.RunStatus(value))
	}
	return result
}

func respond[T any](c fiber.Ctx, value T, err error) error {
	if err != nil {
		return err
	}
	return c.JSON(value)
}

func errorHandler(c fiber.Ctx, err error) error {
	status := fiber.StatusInternalServerError
	message := "internal server error"
	var fiberError *fiber.Error
	if errors.As(err, &fiberError) {
		status, message = fiberError.Code, fiberError.Message
	} else if err != nil {
		status, message = fiber.StatusBadRequest, err.Error()
	}
	return c.Status(status).JSON(fiber.Map{"error": message})
}

func validBearer(header, expected string) bool {
	const prefix = "Bearer "
	if !strings.HasPrefix(header, prefix) || expected == "" {
		return false
	}
	presented := strings.TrimSpace(strings.TrimPrefix(header, prefix))
	if len(presented) != len(expected) {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(presented), []byte(expected)) == 1
}

func normalizeConfig(config domain.APISettings) domain.APISettings {
	if config.BindAddress == "" {
		config.BindAddress = "127.0.0.1"
	}
	if config.Port == 0 {
		config.Port = 7878
	}
	if config.AuthMode == "" {
		config.AuthMode = domain.APIAuthToken
	}
	if config.AllowedOrigins == nil {
		config.AllowedOrigins = []string{}
	}
	return config
}

func sameConfig(left, right domain.APISettings) bool {
	left, right = normalizeConfig(left), normalizeConfig(right)
	if left.Enabled != right.Enabled || left.BindAddress != right.BindAddress || left.Port != right.Port || left.AuthMode != right.AuthMode || left.TokenRef != right.TokenRef || left.AdminEnabled != right.AdminEnabled || len(left.AllowedOrigins) != len(right.AllowedOrigins) {
		return false
	}
	for index := range left.AllowedOrigins {
		if left.AllowedOrigins[index] != right.AllowedOrigins[index] {
			return false
		}
	}
	return true
}

func publicSettings(settings domain.Settings) domain.Settings {
	settings.API.TokenRef = ""
	for index := range settings.Providers {
		settings.Providers[index].APIKeyRef = ""
	}
	return settings
}

func openAPI() map[string]any {
	return map[string]any{
		"openapi": "3.1.0",
		"info":    map[string]string{"title": "Neuropipe local API", "version": "v1"},
		"paths": map[string]any{
			"/health":                    map[string]any{"get": map[string]string{"summary": "Health check"}},
			"/v1/pipelines":              map[string]any{"get": map[string]string{"summary": "List pipelines"}},
			"/v1/pipelines/{id}/runs":    map[string]any{"post": map[string]string{"summary": "Queue a published pipeline"}},
			"/v1/executions/{id}":        map[string]any{"get": map[string]string{"summary": "Read execution and node logs"}},
			"/v1/admin/metrics/overview": map[string]any{"get": map[string]string{"summary": "Read local operational metrics"}},
			"/hooks/{path}":              map[string]any{"post": map[string]string{"summary": "Deliver an HMAC-signed webhook"}},
		},
	}
}

// DecodeJSON is used by tests and keeps raw webhook input validation explicit.
func DecodeJSON(data []byte) (map[string]any, error) {
	var value map[string]any
	if err := json.Unmarshal(data, &value); err != nil {
		return nil, err
	}
	return value, nil
}

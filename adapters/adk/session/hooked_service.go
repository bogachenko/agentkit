package adksession

import (
	"context"
	"errors"

	"github.com/bogachenko/agentkit/core/port"
	adksdk "google.golang.org/adk/session"
)

// EventHook lets session metadata logic run before ADK persists an event.
type EventHook interface {
	BeforeAppendEvent(ctx context.Context, sess adksdk.Session, event *adksdk.Event) error
}

// HookedSessionService adds deterministic hook execution without replacing ADK session storage.
type HookedSessionService struct {
	base   adksdk.Service
	logger port.Logger
	hooks  []EventHook
}

// HookedSessionServiceConfig keeps ADK storage and hook dependencies explicit.
type HookedSessionServiceConfig struct {
	Base   adksdk.Service
	Logger port.Logger
	Hooks  []EventHook
}

// NewHookedSessionService rejects missing storage and filters nil hooks at construction time.
func NewHookedSessionService(cfg HookedSessionServiceConfig) (*HookedSessionService, error) {
	if cfg.Base == nil {
		return nil, errors.New("base session service is required")
	}

	hooks := make([]EventHook, 0, len(cfg.Hooks))
	for _, hook := range cfg.Hooks {
		if hook == nil {
			continue
		}

		hooks = append(hooks, hook)
	}

	return &HookedSessionService{
		base:   cfg.Base,
		logger: cfg.Logger,
		hooks:  hooks,
	}, nil
}

// Create preserves ADK session.Service behavior while allowing this wrapper to satisfy the same interface.
func (s *HookedSessionService) Create(ctx context.Context, req *adksdk.CreateRequest) (*adksdk.CreateResponse, error) {
	return s.base.Create(ctx, req)
}

// Get preserves ADK session.Service behavior while allowing this wrapper to satisfy the same interface.
func (s *HookedSessionService) Get(ctx context.Context, req *adksdk.GetRequest) (*adksdk.GetResponse, error) {
	return s.base.Get(ctx, req)
}

// List preserves ADK session.Service behavior while allowing this wrapper to satisfy the same interface.
func (s *HookedSessionService) List(ctx context.Context, req *adksdk.ListRequest) (*adksdk.ListResponse, error) {
	return s.base.List(ctx, req)
}

// Delete preserves ADK session.Service behavior while allowing this wrapper to satisfy the same interface.
func (s *HookedSessionService) Delete(ctx context.Context, req *adksdk.DeleteRequest) error {
	return s.base.Delete(ctx, req)
}

// AppendEvent runs metadata hooks before delegating persistence to ADK storage.
func (s *HookedSessionService) AppendEvent(ctx context.Context, sess adksdk.Session, event *adksdk.Event) error {
	for _, hook := range s.hooks {
		if err := hook.BeforeAppendEvent(ctx, sess, event); err != nil && s.logger != nil {
			s.logger.Printf("session event hook failed: %v", err)
		}
	}

	return s.base.AppendEvent(ctx, sess, event)
}

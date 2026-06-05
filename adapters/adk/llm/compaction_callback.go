package llm

import (
	"context"
	"fmt"

	"encoding/json"
	"github.com/bogachenko/agentkit/core/compaction"
	"github.com/bogachenko/agentkit/core/port"
	"google.golang.org/adk/agent"
	"google.golang.org/adk/agent/llmagent"
	"google.golang.org/adk/model"
	"reflect"
)

type CompactionCallbackConfig struct {
	Compactor *compaction.Compactor
	Tracer    port.Tracer
	Logger    port.Logger
}

func CompactionBeforeModelCallback(cfg CompactionCallbackConfig) llmagent.BeforeModelCallback {
	return func(cctx agent.CallbackContext, llmRequest *model.LLMRequest) (*model.LLMResponse, error) {
		if cfg.Compactor == nil || llmRequest == nil {
			return nil, nil
		}

		ctx := context.Context(cctx)
		if ctx == nil {
			ctx = context.Background()
		}

		request := RequestToCore(llmRequest, []string{
			"agent=" + cctx.AgentName(),
			"app=" + cctx.AppName(),
			"session=" + cctx.SessionID(),
			"invocation=" + cctx.InvocationID(),
		})

		ctx, span := startCompactionSpan(ctx, cfg.Tracer, map[string]any{
			"agentkit.compaction.agent":           cctx.AgentName(),
			"agentkit.compaction.app":             cctx.AppName(),
			"agentkit.compaction.session_id":      cctx.SessionID(),
			"agentkit.compaction.invocation_id":   cctx.InvocationID(),
			"agentkit.compaction.messages_before": len(request.Messages),
			"agentkit.compaction.raw_contents":    len(llmRequest.Contents),
			"agentkit.compaction.enabled":         cfg.Compactor.Config().Enabled,
			"agentkit.compaction.token_threshold": cfg.Compactor.Config().TokenThreshold,
		})
		defer endSpan(span)

		if cfg.Logger != nil {
			cfg.Logger.Printf("context compaction before model agent=%s messages=%d raw_contents=%d", cctx.AgentName(), len(request.Messages), len(llmRequest.Contents))
		}

		result, err := cfg.Compactor.Compact(ctx, request, NewState(cctx.State()))
		setCompactionDiagnostics(span, result)
		if err != nil {
			if span != nil {
				span.RecordError(err)
				span.SetAttributes(map[string]any{"agentkit.compaction.error": err.Error()})
			}
			if cfg.Logger != nil {
				cfg.Logger.Printf("context compaction failed: %v", err)
			}
			return nil, err
		}

		if result.Changed {
			if err := ApplyCoreMessages(llmRequest, result.Messages); err != nil {
				wrapped := fmt.Errorf("apply compacted messages: %w", err)
				if span != nil {
					span.RecordError(wrapped)
				}
				return nil, wrapped
			}
		}

		traceCompactionResult(span, result)

		if cfg.Logger != nil {
			cfg.Logger.Printf("context compaction result agent=%s changed=%t compacted=%t messages_before=%d messages_after=%d tokens_before=%d tokens_after=%d last_compacted_index=%d", cctx.AgentName(), result.Changed, result.Compacted, result.MessagesBefore, result.MessagesAfter, result.EstimatedTokensBefore, result.EstimatedTokensAfter, result.LastCompactedIndex)
		}

		return nil, nil
	}
}

func startCompactionSpan(ctx context.Context, tracer port.Tracer, attrs map[string]any) (context.Context, port.Span) {
	if tracer == nil {
		return ctx, nil
	}
	return tracer.Start(ctx, "agentkit.compaction.before_model", attrs)
}

func endSpan(span port.Span) {
	if span != nil {
		span.End()
	}
}

func traceCompactionResult(span port.Span, result compaction.Result) {
	if span == nil {
		return
	}

	span.SetAttributes(map[string]any{
		"agentkit.compaction.changed":                    result.Changed,
		"agentkit.compaction.sanitized":                  result.Sanitized,
		"agentkit.compaction.compacted":                  result.Compacted,
		"agentkit.compaction.summary_generated":          result.SummaryGenerated,
		"agentkit.compaction.summary_reused":             result.SummaryReused,
		"agentkit.compaction.hard_fallback_used":         result.HardFallbackUsed,
		"agentkit.compaction.messages_after":             result.MessagesAfter,
		"agentkit.compaction.last_compacted_index":       result.LastCompactedIndex,
		"agentkit.compaction.estimated_tokens_before":    result.EstimatedTokensBefore,
		"agentkit.compaction.estimated_tokens_sanitized": result.EstimatedTokensSanitized,
		"agentkit.compaction.estimated_tokens_after":     result.EstimatedTokensAfter,
		"langfuse.observation.output":                    compactionObservationOutput(result),
	})

	span.AddEvent("agentkit.compaction.result", map[string]any{
		"changed":                 result.Changed,
		"compacted":               result.Compacted,
		"messages_before":         result.MessagesBefore,
		"messages_after":          result.MessagesAfter,
		"estimated_tokens_before": result.EstimatedTokensBefore,
		"estimated_tokens_after":  result.EstimatedTokensAfter,
	})
}

func compactionObservationOutput(result compaction.Result) string {
	return fmt.Sprintf("changed=%t compacted=%t sanitized=%t hard_fallback=%t messages=%d->%d estimated_tokens=%d->%d last_compacted_index=%d", result.Changed, result.Compacted, result.Sanitized, result.HardFallbackUsed, result.MessagesBefore, result.MessagesAfter, result.EstimatedTokensBefore, result.EstimatedTokensAfter, result.LastCompactedIndex)
}

func setCompactionDiagnostics(span port.Span, result any) {
	if span == nil {
		return
	}

	metadata := exportedStructMetadata(result)
	attrs := map[string]any{
		"langfuse.observation.metadata": jsonString(metadata),
	}

	for key, value := range metadata {
		switch value.(type) {
		case string, bool, int, int64, float64:
			attrs["agentkit.compaction."+key] = value
		}
	}

	span.SetAttributes(attrs)
}

func exportedStructMetadata(value any) map[string]any {
	out := map[string]any{}

	v := reflect.ValueOf(value)
	if !v.IsValid() {
		return out
	}

	if v.Kind() == reflect.Pointer {
		if v.IsNil() {
			return out
		}
		v = v.Elem()
	}

	if v.Kind() != reflect.Struct {
		return out
	}

	t := v.Type()
	for i := 0; i < v.NumField(); i++ {
		field := t.Field(i)
		if field.PkgPath != "" {
			continue
		}

		fv := v.Field(i)
		if !fv.CanInterface() {
			continue
		}

		out[field.Name] = fv.Interface()
	}

	return out
}

func jsonString(value any) string {
	data, err := json.Marshal(value)
	if err != nil {
		return "{}"
	}

	return string(data)
}

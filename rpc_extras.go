package codexgo

import (
	"context"
	"encoding/json"

	"github.com/zealbase/codex-app-server-go/internal/protocol"
	schematypes "github.com/zealbase/codex-app-server-go/internal/protocol/schema"
)

// MergeStrategy controls how ConfigValueWrite merges a value into an existing key.
type MergeStrategy string

const (
	// MergeStrategyReplace overwrites the existing value at the key path.
	MergeStrategyReplace MergeStrategy = "replace"
	// MergeStrategyUpsert merges into the existing value (e.g. table merge).
	MergeStrategyUpsert MergeStrategy = "upsert"
)

// ConfigReadRequest reads the effective config (optionally with layer breakdown).
type ConfigReadRequest struct {
	CWD           string `json:"cwd,omitempty"`
	IncludeLayers bool   `json:"includeLayers,omitempty"`
}

// ConfigReadResult holds the resolved config. The nested config, layers, and
// origins documents are passed through as raw JSON; decode them as needed.
type ConfigReadResult struct {
	Config  json.RawMessage `json:"config"`
	Layers  json.RawMessage `json:"layers,omitempty"`
	Origins json.RawMessage `json:"origins,omitempty"`
}

// ConfigValueWriteRequest writes a single value at a dotted key path.
type ConfigValueWriteRequest struct {
	FilePath        string          `json:"filePath,omitempty"`
	KeyPath         string          `json:"keyPath"`
	Value           json.RawMessage `json:"value"`
	MergeStrategy   MergeStrategy   `json:"mergeStrategy,omitempty"`
	ExpectedVersion string          `json:"expectedVersion,omitempty"`
}

// ConfigBatchWriteRequest applies multiple edits atomically. Edits is the raw
// JSON array of edit objects as defined by the protocol.
type ConfigBatchWriteRequest struct {
	FilePath         string          `json:"filePath,omitempty"`
	Edits            json.RawMessage `json:"edits"`
	ExpectedVersion  string          `json:"expectedVersion,omitempty"`
	ReloadUserConfig bool            `json:"reloadUserConfig,omitempty"`
}

// ConfigRead returns the effective configuration. When IncludeLayers is set the
// per-layer breakdown and value origins are included.
func (c *Client) ConfigRead(ctx context.Context, req ConfigReadRequest) (ConfigReadResult, error) {
	var resp ConfigReadResult
	if err := c.transport.Call(ctx, protocol.MethodConfigRead, req, &resp); err != nil {
		return ConfigReadResult{}, err
	}
	return resp, nil
}

// ConfigValueWrite writes a single configuration value at the given key path.
func (c *Client) ConfigValueWrite(ctx context.Context, req ConfigValueWriteRequest) error {
	return c.transport.Call(ctx, protocol.MethodConfigValueWrite, req, nil)
}

// ConfigBatchWrite applies a batch of configuration edits.
func (c *Client) ConfigBatchWrite(ctx context.Context, req ConfigBatchWriteRequest) error {
	return c.transport.Call(ctx, protocol.MethodConfigBatchWrite, req, nil)
}

type (
	// SkillsListEntry is the per-cwd skills listing.
	SkillsListEntry = schematypes.SkillsListEntry
	// SkillMetadata describes a single discovered skill.
	SkillMetadata = schematypes.SkillMetadata
	// SkillErrorInfo describes a skill discovery error.
	SkillErrorInfo = schematypes.SkillErrorInfo

	SkillsListRequest          = schematypes.SkillsListParams
	SkillsListResult           = schematypes.SkillsListResponse
	SkillsConfigWriteRequest   = schematypes.SkillsConfigWriteParams
	SkillsConfigWriteResult    = schematypes.SkillsConfigWriteResponse
	SkillsExtraRootsSetRequest = schematypes.SkillsExtraRootsSetParams
)

// SkillsList lists skills discoverable from the given working directories.
func (c *Client) SkillsList(ctx context.Context, req SkillsListRequest) (SkillsListResult, error) {
	var resp SkillsListResult
	if err := c.transport.Call(ctx, protocol.MethodSkillsList, req, &resp); err != nil {
		return SkillsListResult{}, err
	}
	return resp, nil
}

// SkillsConfigWrite enables or disables a skill (by name or path) and returns
// the effective enabled state.
func (c *Client) SkillsConfigWrite(ctx context.Context, req SkillsConfigWriteRequest) (SkillsConfigWriteResult, error) {
	var resp SkillsConfigWriteResult
	if err := c.transport.Call(ctx, protocol.MethodSkillsConfigWrite, req, &resp); err != nil {
		return SkillsConfigWriteResult{}, err
	}
	return resp, nil
}

// SkillsExtraRootsSet sets additional filesystem roots scanned for skills.
func (c *Client) SkillsExtraRootsSet(ctx context.Context, req SkillsExtraRootsSetRequest) error {
	return c.transport.Call(ctx, protocol.MethodSkillsExtraRootsSet, req, nil)
}

// ExperimentalFeature describes a single experimental feature flag.
type ExperimentalFeature struct {
	Name           string          `json:"name"`
	DisplayName    string          `json:"displayName,omitempty"`
	Description    string          `json:"description,omitempty"`
	Announcement   string          `json:"announcement,omitempty"`
	Enabled        bool            `json:"enabled"`
	DefaultEnabled bool            `json:"defaultEnabled"`
	Stage          json.RawMessage `json:"stage,omitempty"`
}

// ExperimentalFeatureListRequest lists experimental features. Cursor/Limit drive
// pagination; an empty NextCursor in the result indicates the final page.
type ExperimentalFeatureListRequest struct {
	ThreadID string `json:"threadId,omitempty"`
	Cursor   string `json:"cursor,omitempty"`
	Limit    int    `json:"limit,omitempty"`
}

// ExperimentalFeatureListResult is a page of experimental features.
type ExperimentalFeatureListResult struct {
	Data       []ExperimentalFeature `json:"data"`
	NextCursor string                `json:"nextCursor,omitempty"`
}

// ExperimentalFeatureEnablementSetRequest sets enablement for one or more
// features, keyed by feature name.
type ExperimentalFeatureEnablementSetRequest struct {
	Enablement map[string]bool `json:"enablement"`
}

// ExperimentalFeatureEnablementSetResult reports the resulting enablement map.
type ExperimentalFeatureEnablementSetResult struct {
	Enablement map[string]bool `json:"enablement"`
}

// ExperimentalFeatureList returns a page of experimental features.
func (c *Client) ExperimentalFeatureList(ctx context.Context, req ExperimentalFeatureListRequest) (ExperimentalFeatureListResult, error) {
	var resp ExperimentalFeatureListResult
	if err := c.transport.Call(ctx, protocol.MethodExperimentalFeatureList, req, &resp); err != nil {
		return ExperimentalFeatureListResult{}, err
	}
	return resp, nil
}

// ExperimentalFeatureEnablementSet enables or disables experimental features.
func (c *Client) ExperimentalFeatureEnablementSet(ctx context.Context, req ExperimentalFeatureEnablementSetRequest) (ExperimentalFeatureEnablementSetResult, error) {
	var resp ExperimentalFeatureEnablementSetResult
	if err := c.transport.Call(ctx, protocol.MethodExperimentalFeatureEnablementSet, req, &resp); err != nil {
		return ExperimentalFeatureEnablementSetResult{}, err
	}
	return resp, nil
}

// HooksListRequest lists configured lifecycle hooks for the given working
// directories. This is the server-side hooks/list RPC and is distinct from the
// client-side hook bridge (internal/hookbridge).
type HooksListRequest struct {
	CWDs []string `json:"cwds,omitempty"`
}

// HooksListResult holds the hooks listing. Data is passed through as raw JSON
// (an array of per-cwd hook entries) for forward compatibility.
type HooksListResult struct {
	Data json.RawMessage `json:"data"`
}

// HooksList returns the configured lifecycle hooks discoverable from the given
// working directories.
func (c *Client) HooksList(ctx context.Context, req HooksListRequest) (HooksListResult, error) {
	var resp HooksListResult
	if err := c.transport.Call(ctx, protocol.MethodHooksList, req, &resp); err != nil {
		return HooksListResult{}, err
	}
	return resp, nil
}

// ModelProviderCapabilities reports which provider-backed capabilities are
// available for the current session.
type ModelProviderCapabilities struct {
	NamespaceTools  bool `json:"namespaceTools"`
	ImageGeneration bool `json:"imageGeneration"`
	WebSearch       bool `json:"webSearch"`
}

// ModelProviderCapabilitiesRead returns the capabilities the active model
// provider supports (namespaced tools, image generation, web search).
func (c *Client) ModelProviderCapabilitiesRead(ctx context.Context) (ModelProviderCapabilities, error) {
	var resp ModelProviderCapabilities
	if err := c.transport.Call(ctx, protocol.MethodModelProviderCapabilitiesRead, struct{}{}, &resp); err != nil {
		return ModelProviderCapabilities{}, err
	}
	return resp, nil
}

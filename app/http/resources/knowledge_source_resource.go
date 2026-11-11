/*
|--------------------------------------------------------------------------
| Knowledge Source Resources
|--------------------------------------------------------------------------
|
| How sources, their runs and their documents read over the API. Ids are
| public ids. A source's settings come back without its secrets; it only
| says which secrets are set, so a form can show them as kept.
|
*/

package resources

import (
	"encoding/json"
	"time"

	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/knowledgesources"
	"github.com/dimmihq/dimmi/app/integrations/knowledge/sources"
	"github.com/dimmihq/dimmi/app/models"
)

/*
Public id prefixes of knowledge sources, runs and documents.
*/
const (
	prefixKnowledgeSource   = "ks"
	prefixKnowledgeSyncRun  = "ksr"
	prefixKnowledgeDocument = "kd"
)

/*
KnowledgeSource is one source with its counts and latest run.
*/
type KnowledgeSource struct {
	ID            string            `json:"id"`
	Object        string            `json:"object"`
	Type          string            `json:"type"`
	Name          string            `json:"name"`
	Mode          string            `json:"mode"`
	Targets       []string          `json:"targets"`
	HelpCenterID  *string           `json:"help_center_id"`
	Settings      json.RawMessage   `json:"settings"`
	SecretsSet    []string          `json:"secrets_set"`
	Include       []string          `json:"include"`
	Exclude       []string          `json:"exclude"`
	Status        string            `json:"status"`
	Schedule      string            `json:"schedule"`
	NextSyncAt    *time.Time        `json:"next_sync_at"`
	LastSyncedAt  *time.Time        `json:"last_synced_at"`
	LastSuccessAt *time.Time        `json:"last_success_at"`
	LastErrorCode *string           `json:"last_error_code"`
	LastError     *string           `json:"last_error"`
	DocumentCount int32             `json:"document_count"`
	Counts        map[string]int32  `json:"counts"`
	LastRun       *KnowledgeSyncRun `json:"last_run"`
	CreatedAt     time.Time         `json:"created_at"`
	UpdatedAt     time.Time         `json:"updated_at"`
}

/*
NewKnowledgeSource shapes a source.
*/
func NewKnowledgeSource(v knowledgesources.SourceView) KnowledgeSource {
	s := v.Source
	out := KnowledgeSource{
		ID: KnowledgeSourceID(s.ID), Object: "knowledge_source", Type: s.Type, Name: s.Name, Mode: s.Mode,
		Targets: s.Targets, Settings: s.Config, SecretsSet: orEmpty(v.Secrets), Include: s.Include,
		Exclude: s.Exclude, Status: s.Status, Schedule: s.Schedule, NextSyncAt: s.NextSyncAt,
		LastSyncedAt: s.LastSyncedAt, LastSuccessAt: s.LastSuccessAt, LastErrorCode: s.LastErrorCode,
		LastError: s.LastError, DocumentCount: s.DocumentCount, Counts: v.Counts, CreatedAt: s.CreatedAt,
		UpdatedAt: s.UpdatedAt,
	}

	if s.HelpCenterID != nil {
		id := models.PublicID(models.PrefixHelpCenter, *s.HelpCenterID)
		out.HelpCenterID = &id
	}

	if v.LastRun != nil {
		run := NewKnowledgeSyncRun(*v.LastRun)
		out.LastRun = &run
	}

	return out
}

/*
KnowledgeSourceID is a source's public id.
*/
func KnowledgeSourceID(id ids.UUID) string {
	return ids.Encode(prefixKnowledgeSource, id)
}

/*
KnowledgeSyncRun is one run of a source and its counters.
*/
type KnowledgeSyncRun struct {
	ID         string          `json:"id"`
	Object     string          `json:"object"`
	SourceID   string          `json:"source_id"`
	Trigger    string          `json:"trigger"`
	Status     string          `json:"status"`
	FullRun    bool            `json:"full_run"`
	Stats      json.RawMessage `json:"stats"`
	ErrorCode  *string         `json:"error_code"`
	Error      *string         `json:"error"`
	StartedAt  *time.Time      `json:"started_at"`
	FinishedAt *time.Time      `json:"finished_at"`
	CreatedAt  time.Time       `json:"created_at"`
}

/*
NewKnowledgeSyncRun shapes a run.
*/
func NewKnowledgeSyncRun(r models.KnowledgeSyncRun) KnowledgeSyncRun {
	return KnowledgeSyncRun{
		ID: ids.Encode(prefixKnowledgeSyncRun, r.ID), Object: "knowledge_sync_run",
		SourceID: KnowledgeSourceID(r.SourceID), Trigger: r.Trigger, Status: r.Status, FullRun: r.FullRun,
		Stats: r.Stats, ErrorCode: r.ErrorCode, Error: r.Error, StartedAt: r.StartedAt, FinishedAt: r.FinishedAt,
		CreatedAt: r.CreatedAt,
	}
}

/*
KnowledgeSyncRunID decodes a run's public id.
*/
func KnowledgeSyncRunID(registry *ids.Registry, public string) (ids.UUID, error) {
	return registry.Decode(prefixKnowledgeSyncRun, public)
}

/*
KnowledgeSourceUUID decodes a source's public id.
*/
func KnowledgeSourceUUID(registry *ids.Registry, public string) (ids.UUID, error) {
	return registry.Decode(prefixKnowledgeSource, public)
}

/*
KnowledgeDocumentUUID decodes a document's public id.
*/
func KnowledgeDocumentUUID(registry *ids.Registry, public string) (ids.UUID, error) {
	return registry.Decode(prefixKnowledgeDocument, public)
}

/*
KnowledgeDocument is one document of a source.
*/
type KnowledgeDocument struct {
	ID              string     `json:"id"`
	Object          string     `json:"object"`
	ExternalID      string     `json:"external_id"`
	URL             *string    `json:"url"`
	Path            string     `json:"path"`
	Title           string     `json:"title"`
	Locale          string     `json:"locale"`
	Status          string     `json:"status"`
	ExcludedBy      *string    `json:"excluded_by"`
	ErrorCode       *string    `json:"error_code"`
	Error           *string    `json:"error"`
	ChunkCount      int32      `json:"chunk_count"`
	LinkedArticleID *string    `json:"linked_article_id"`
	FetchedAt       *time.Time `json:"fetched_at"`
	IndexedAt       *time.Time `json:"indexed_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

/*
NewKnowledgeDocument shapes a document.
*/
func NewKnowledgeDocument(d models.ListKnowledgeDocumentsRow) KnowledgeDocument {
	out := KnowledgeDocument{
		ID: ids.Encode(prefixKnowledgeDocument, d.ID), Object: "knowledge_document", ExternalID: d.ExternalID,
		URL: d.Url, Path: d.Path, Title: d.Title, Locale: d.Locale, Status: d.Status, ExcludedBy: d.ExcludedBy,
		ErrorCode: d.ErrorCode, Error: d.Error, ChunkCount: d.ChunkCount, FetchedAt: d.FetchedAt,
		IndexedAt: d.IndexedAt, UpdatedAt: d.UpdatedAt,
	}

	if d.LinkedArticleID != nil {
		id := models.PublicID(models.PrefixArticle, *d.LinkedArticleID)
		out.LinkedArticleID = &id
	}

	return out
}

/*
KnowledgeSourceType is one driver a source can be added from.
*/
type KnowledgeSourceType struct {
	Object string `json:"object"`
	sources.Description
}

/*
NewKnowledgeSourceType shapes a driver's description.
*/
func NewKnowledgeSourceType(d sources.Description) KnowledgeSourceType {
	return KnowledgeSourceType{Object: "knowledge_source_type", Description: d}
}

/*
|--------------------------------------------------------------------------
| Knowledge Contract
|--------------------------------------------------------------------------
|
| The agent only ever answers from what the workspace's knowledge
| index returns. Whoever owns the index — today a stub, later the help
| center — sits behind this one interface, so the pipeline's retrieval
| step never changes when the real index lands.
|
*/

package contracts

import (
	"context"
	"time"

	"github.com/dimmihq/dimmi/framework/ids"
)

/*
Knowledge is the workspace's searchable body of answers. Every call is
scoped to one workspace; the implementation decides how chunks are
ranked and never errors when a search can fall back to keyword match.
*/
type Knowledge interface {
	/*
		Search returns ranked chunks inside one workspace. With no
		embeddings the answer is keyword search, never an error.
	*/
	Search(ctx context.Context, workspaceID ids.UUID, query string, opts KnowledgeSearchOptions) (KnowledgeSearchResult, error)

	/*
		Document returns the full normalized Markdown of one document,
		filtered by the same visibility rule as Search.
	*/
	Document(ctx context.Context, workspaceID, documentID ids.UUID, visibility []KnowledgeVisibility) (KnowledgeDocument, error)

	/*
		Status reports whether the workspace has searchable content;
		the agent's readiness check reads it.
	*/
	Status(ctx context.Context, workspaceID ids.UUID) (KnowledgeStatus, error)

	/*
		EmbeddingKeyInUse reports whether a provider key feeds the
		index, so the key cannot be deleted under it.
	*/
	EmbeddingKeyInUse(ctx context.Context, workspaceID, providerKeyID ids.UUID) (bool, error)

	/*
		PublicHosts lists the help center hosts whose links the agent
		may emit.
	*/
	PublicHosts(ctx context.Context, workspaceID ids.UUID) ([]string, error)
}

/*
KnowledgeVisibility marks who may ever see a chunk's text.
*/
type KnowledgeVisibility string

/*
The visibilities. Public text may reach a customer; agent-only text is
for the agent's eyes and ears only and never renders on a public
surface.
*/
const (
	KnowledgeVisibilityPublic    KnowledgeVisibility = "public"
	KnowledgeVisibilityAgentOnly KnowledgeVisibility = "agent_only"
)

/*
KnowledgeSearchOptions narrows one search.
*/
type KnowledgeSearchOptions struct {
	Limit              int
	AllowedAudienceIDs []ids.UUID
	Visibility         []KnowledgeVisibility
	Queries            []string
	Locale             string
	HelpCenterIDs      []ids.UUID
	SourceIDs          []ids.UUID
	ExcludeChunkIDs    []ids.UUID
	ExcludeArticles    bool
	ExcludeSources     bool
	ExcludeChangelog   bool
	Mode               string
	MaxPerDocument     int
	QueryEmbedTimeout  time.Duration
	Surface            string
	LogSearch          bool
}

/*
KnowledgeSearchResult is what one search found, plus how it was found.
*/
type KnowledgeSearchResult struct {
	Chunks         []KnowledgeChunk
	Mode           string
	Degraded       string
	EmbeddingModel string
	EmbeddingKeyID *ids.UUID
	EmbeddingUsage *LLMUsage
	Timings        map[string]time.Duration
}

/*
KnowledgeChunk is one citable passage of one document.
*/
type KnowledgeChunk struct {
	ID               ids.UUID
	DocumentID       ids.UUID
	ArticleID        *ids.UUID
	HelpCenterID     *ids.UUID
	SourceID         *ids.UUID
	ChangelogEntryID *ids.UUID
	SourceType       string
	Title            string
	HeadingPath      []string
	URL              string
	Text             string
	Similarity       *float64
	Score            float64
	Rank             int
	MatchedBy        []string
	Visibility       KnowledgeVisibility
	Locale           string
	ContentHash      string
	UpdatedAt        time.Time
}

/*
KnowledgeDocument is one document's full normalized Markdown under the
same visibility rule as Search.
*/
type KnowledgeDocument struct {
	ID         ids.UUID
	SourceType string
	Title      string
	URL        string
	Text       string
	Visibility KnowledgeVisibility
	Locale     string
	UpdatedAt  time.Time
}

/*
KnowledgeStatus is the index's readiness answer: whether anything is
searchable and which embedder it uses, for the agent's enable check.
*/
type KnowledgeStatus struct {
	HasContent     bool
	DocumentCount  int64
	ChunkCount     int64
	EmbeddingModel string
}

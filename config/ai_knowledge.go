/*
|--------------------------------------------------------------------------
| AI Knowledge Settings
|--------------------------------------------------------------------------
|
| Semantic search needs an embedder and bounds: the built-in embedder
| every workspace starts with, with its own key apart from the language
| models', and how long a search waits for vectors before it answers
| from keywords alone.
|
*/

package config

/*
AIManagedEmbeddings is the built-in embedder every workspace gets
semantic search from, with its own key apart from the language
models'. Without a key the built-in embedder is absent. Model is the
Voyage model and Dims the size of its vectors. RerankModel, on the same
key, orders what a search found by how well each passage answers the
question; empty turns reranking off.
*/
type AIManagedEmbeddings struct {
	APIKey      string `koanf:"api_key"                      secret:"true"`
	BaseURL     string `default:"https://api.voyageai.com/v1" koanf:"base_url"`
	Model       string `default:"voyage-4-lite"               koanf:"model"`
	Dims        int    `default:"1024"                        koanf:"dims"`
	RerankModel string `default:"rerank-2.5-lite"             koanf:"rerank_model"`
}

/*
AIKnowledgeConfig bounds retrieval: how long a search waits for the
query's vector and for the vector query before it answers from
keywords as the last resort. The built-in embedder answers a short
query in a fraction of a second, so the wait is short; a vector found
once is cached and later searches for the same words wait for nothing.
A query whose vector has not come back after QueryEmbedHedgeMS is asked
for a second time, and whichever answer comes first is used, so one
slow call does not send a search to keywords; zero turns that off.
RerankTimeoutMS is how long the reranker has; past it the passages are
returned nearest first. The three waits add up to under two seconds, the
longest a search by meaning may take.
CrawlIntervalMS is the least time between two requests the website
crawler sends one host; zero lifts the pace, for tests only. With a
Cloudflare API token, websites are crawled through Cloudflare's
Browser Run instead, which renders pages that need JavaScript; the
account is found from the token unless CloudflareAccountID names it.
CloudflareBrowserHourMicros is what an hour of that browser costs, in
millionths of a dollar, so crawl time can be billed on.
*/
type AIKnowledgeConfig struct {
	QueryEmbedTimeoutMS int    `default:"1000" koanf:"query_embed_timeout_ms"`
	QueryEmbedHedgeMS   int    `default:"350"  koanf:"query_embed_hedge_ms"`
	VectorTimeoutMS     int    `default:"250"  koanf:"vector_timeout_ms"`
	RerankTimeoutMS     int    `default:"700"  koanf:"rerank_timeout_ms"`
	CrawlIntervalMS     int    `default:"1000" koanf:"crawl_interval_ms"`
	CloudflareAPIToken  string `koanf:"cloudflare_api_token"  secret:"true"`
	CloudflareAccountID string `koanf:"cloudflare_account_id"`

	CloudflareBrowserHourMicros int64 `default:"90000" koanf:"cloudflare_browser_hour_micros"`
}

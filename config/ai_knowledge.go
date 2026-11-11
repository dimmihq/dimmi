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
upstream model and Dims the size its vectors are shortened to.
*/
type AIManagedEmbeddings struct {
	APIKey  string `koanf:"api_key"`
	BaseURL string `default:"https://api.deepinfra.com/v1/openai" koanf:"base_url"`
	Model   string `default:"Qwen/Qwen3-Embedding-8B"             koanf:"model"`
	Dims    int    `default:"1024"                                koanf:"dims"`
}

/*
AIKnowledgeConfig bounds retrieval: how long a search waits for the
query's vector and for the vector query before it answers from
keywords alone. Hosted embedders answer a short query in one to five
seconds, so the wait leaves room for that; a vector found once is
cached and later searches for the same words wait for nothing.
CrawlIntervalMS is the least time between two requests the website
crawler sends one host; zero lifts the pace, for tests only. With a
Cloudflare API token, websites are crawled through Cloudflare's
Browser Run instead, which renders pages that need JavaScript; the
account is found from the token unless CloudflareAccountID names it.
CloudflareBrowserHourMicros is what an hour of that browser costs, in
millionths of a dollar, so crawl time can be billed on.
*/
type AIKnowledgeConfig struct {
	QueryEmbedTimeoutMS int    `default:"4000" koanf:"query_embed_timeout_ms"`
	VectorTimeoutMS     int    `default:"250"  koanf:"vector_timeout_ms"`
	CrawlIntervalMS     int    `default:"1000" koanf:"crawl_interval_ms"`
	CloudflareAPIToken  string `koanf:"cloudflare_api_token"`
	CloudflareAccountID string `koanf:"cloudflare_account_id"`

	CloudflareBrowserHourMicros int64 `default:"90000" koanf:"cloudflare_browser_hour_micros"`
}

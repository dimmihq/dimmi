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
keywords alone.
*/
type AIKnowledgeConfig struct {
	QueryEmbedTimeoutMS int `default:"1500" koanf:"query_embed_timeout_ms"`
	VectorTimeoutMS     int `default:"250"  koanf:"vector_timeout_ms"`
}

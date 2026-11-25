/*
|--------------------------------------------------------------------------
| Reranker Contract
|--------------------------------------------------------------------------
|
| A reranker reads a question beside each passage a search found and
| scores how well the passage answers it. It is slower than comparing
| vectors, so it only orders the few passages the vector search
| already picked, and puts the ones that answer the question first.
|
*/

package contracts

import "context"

/*
MaxRerankDocuments is the most passages one rerank request carries.
*/
const MaxRerankDocuments = 100

/*
RerankRequest asks for the documents to be ordered by how well each
answers the query. TopK keeps only the best; zero keeps them all.
*/
type RerankRequest struct {
	Model     string
	Query     string
	Documents []string
	TopK      int
}

/*
RerankResult is one document's place: its index in the request and its
relevance, higher is better.
*/
type RerankResult struct {
	Index int
	Score float64
}

/*
RerankResponse is the documents best first, the tokens read and the
model that read them.
*/
type RerankResponse struct {
	Results []RerankResult
	Usage   EmbedUsage
	Model   string
}

/*
Reranker orders passages by how well they answer a question.
*/
type Reranker interface {
	/*
		Rerank returns the documents best first, each named by its
		index in the request.
	*/
	Rerank(ctx context.Context, req RerankRequest) (*RerankResponse, error)
}

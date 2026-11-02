/*
|--------------------------------------------------------------------------
| Vectors
|--------------------------------------------------------------------------
|
| Embeddings live next to the rows they describe, in pgvector columns, so
| similarity search runs in the same query and transaction as any other
| filter. Vectors travel as text, which needs no type registration on the
| connection and works behind any pooler.
|
*/

package database

import "github.com/pgvector/pgvector-go"

/*
Vector is a pgvector value. It binds as a query argument and scans from
a vector column.
*/
type Vector = pgvector.Vector

/*
NewVector returns a vector holding the values.
*/
func NewVector(values []float32) Vector {
	return pgvector.NewVector(values)
}

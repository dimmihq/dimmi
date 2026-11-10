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

/*
HalfVector is a pgvector halfvec value: half the storage of a vector,
precise enough for similarity search. It binds and scans like Vector.
*/
type HalfVector = pgvector.HalfVector

/*
NewHalfVector returns a half-precision vector holding the values.
*/
func NewHalfVector(values []float32) HalfVector {
	return pgvector.NewHalfVector(values)
}

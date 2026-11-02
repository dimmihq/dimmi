/*
|--------------------------------------------------------------------------
| Resource Shapes
|--------------------------------------------------------------------------
|
| Every response wraps its payload the same way: one resource under data,
| or a page of them under data with the cursors to move on under meta.
| Clients and generated SDKs rely on that envelope never changing, so it
| is built here and handlers only fill it.
|
*/

package http

import "github.com/dimmihq/dimmi/framework/http/pagination"

/*
Resource is a single resource response.
*/
type Resource[T any] struct {
	Data T `json:"data"`
}

/*
Collection is a page of resources.

Meta holds the cursor to the next and to the previous page, null at
either end, and the page size. Links holds optional URLs, such as the
next page, and is left out when empty.
*/
type Collection[T any] struct {
	Data []T `json:"data"`
	Meta struct {
		NextCursor *string `json:"next_cursor"`
		PrevCursor *string `json:"prev_cursor"`
		PerPage    int     `json:"per_page"`
	} `json:"meta"`
	Links map[string]string `json:"links,omitempty"`
}

/*
NewResource wraps one resource.
*/
func NewResource[T any](data T) Resource[T] {
	return Resource[T]{Data: data}
}

/*
NewCollection wraps a page of resources. An empty page still has a data
list, never null.
*/
func NewCollection[T any](page pagination.Page[T]) Collection[T] {
	c := Collection[T]{Data: page.Items}
	if c.Data == nil {
		c.Data = []T{}
	}

	c.Meta.NextCursor = cursor(page.NextCursor)
	c.Meta.PrevCursor = cursor(page.PrevCursor)
	c.Meta.PerPage = page.Limit

	return c
}

func cursor(s string) *string {
	if s == "" {
		return nil
	}

	return &s
}

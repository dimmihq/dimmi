package dimmi

import (
	"context"
	"encoding/json"
	"net/url"
	"strconv"
)

/*
Iterator walks every item of a list endpoint, fetching pages as needed.

	it := dimmi.List[api.Conversation](client, "/conversations", nil)
	for it.Next(ctx) {
		fmt.Println(it.Item().Id)
	}
	if err := it.Err(); err != nil { ... }
*/
type Iterator[T any] struct {
	next  func(ctx context.Context) ([]T, bool, error)
	page  []T
	index int
	item  T
	done  bool
	err   error
}

// Next moves to the next item, fetching a page when needed.
func (it *Iterator[T]) Next(ctx context.Context) bool {
	for it.index >= len(it.page) {
		if it.done || it.err != nil {
			return false
		}
		page, more, err := it.next(ctx)
		if err != nil {
			it.err = err
			return false
		}
		it.page, it.index, it.done = page, 0, !more
	}
	it.item = it.page[it.index]
	it.index++
	return true
}

// Item is the current item.
func (it *Iterator[T]) Item() T { return it.item }

// Err is the error that stopped the walk, nil at a clean end.
func (it *Iterator[T]) Err() error { return it.err }

// All collects every remaining item.
func (it *Iterator[T]) All(ctx context.Context) ([]T, error) {
	var out []T
	for it.Next(ctx) {
		out = append(out, it.Item())
	}
	return out, it.Err()
}

type listPage[T any] struct {
	Data []T `json:"data"`
	Meta struct {
		NextCursor *string `json:"next_cursor"`
	} `json:"meta"`
}

/*
List walks a cursor-paginated list: it follows meta.next_cursor until
there is none. query may set limit, filters, sort and q.
*/
func List[T any](c *Client, path string, query url.Values) *Iterator[T] {
	q := clone(query)
	return &Iterator[T]{next: func(ctx context.Context) ([]T, bool, error) {
		var page listPage[T]
		if err := c.Do(ctx, "GET", path, q, nil, &page); err != nil {
			return nil, false, err
		}
		if page.Meta.NextCursor == nil || *page.Meta.NextCursor == "" {
			return page.Data, false, nil
		}
		q.Set("cursor", *page.Meta.NextCursor)
		return page.Data, true, nil
	}}
}

/*
Pages walks a list by page number, page=1, 2, ..., stopping at an empty
page or one shorter than limit (25 unless query sets it).
*/
func Pages[T any](c *Client, path string, query url.Values) *Iterator[T] {
	q := clone(query)
	limit, _ := strconv.Atoi(q.Get("limit"))
	if limit <= 0 {
		limit = 25
	}
	number := 1
	if n, err := strconv.Atoi(q.Get("page")); err == nil && n > 0 {
		number = n
	}
	return &Iterator[T]{next: func(ctx context.Context) ([]T, bool, error) {
		q.Set("page", strconv.Itoa(number))
		var page listPage[T]
		if err := c.Do(ctx, "GET", path, q, nil, &page); err != nil {
			return nil, false, err
		}
		number++
		return page.Data, len(page.Data) >= limit, nil
	}}
}

// RawList is List over undecoded items.
func RawList(c *Client, path string, query url.Values) *Iterator[json.RawMessage] {
	return List[json.RawMessage](c, path, query)
}

func clone(v url.Values) url.Values {
	out := url.Values{}
	for k, vs := range v {
		out[k] = append([]string(nil), vs...)
	}
	return out
}

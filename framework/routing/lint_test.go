package routing_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/routing"
)

func TestLint(t *testing.T) {
	t.Parallel()

	rules := routing.Rules{
		VerbDenylist: []string{"close", "login"},
		PutAllowlist: []string{"/api/v1/user/avatar"},
		Prefixes:     []string{"/api/v1/"},
		Exceptions:   []string{"/api/v1/oauth/*", "/api/v1/mcp"},
	}

	lint := func(declare func(r *routing.Router)) []string {
		r := routing.New(routing.Options{})
		r.Group("/api/v1", declare)

		return routing.Lint(r.Routes(), rules)
	}

	t.Run("passes resource routes", func(t *testing.T) {
		t.Parallel()
		problems := lint(func(r *routing.Router) {
			r.Handle(http.MethodGet, "/orders", "orders.index", nothing())
			r.Handle(http.MethodPost, "/orders", "orders.store", nothing())
			r.Handle(http.MethodPatch, "/orders", "orders.update-many", nothing())
			r.Handle(http.MethodGet, "/orders/{tkt}", "orders.show", nothing())
			r.Handle(http.MethodPatch, "/orders/{tkt}", "orders.update", nothing())
			r.Handle(http.MethodDelete, "/orders/{tkt}", "orders.destroy", nothing())
			r.Handle(http.MethodPost, "/orders/{tkt}/merges", "orders.merges.store", nothing())
			r.Handle(http.MethodPatch, "/orders/{tkt}/read-state", "orders.read-state.update", nothing())
			r.Handle(http.MethodGet, "/user", "user.show", nothing())
			r.Handle(http.MethodPut, "/user/avatar", "user.avatar.update", nothing())
			r.Handle(http.MethodDelete, "/sessions", "sessions.destroy-many", nothing())
			r.Handle(http.MethodGet, "/", "index", nothing())
		})

		assert.Empty(t, problems)
	})

	t.Run("reports verbs, stray puts and wrong names", func(t *testing.T) {
		t.Parallel()
		problems := lint(func(r *routing.Router) {
			r.Handle(http.MethodPost, "/orders/{tkt}/close", "orders.close.store", nothing())
			r.Handle(http.MethodPut, "/orders/{tkt}", "orders.update", nothing())
			r.Handle(http.MethodGet, "/orders", "orders.list", nothing())
			r.Handle(http.MethodPost, "/orders/{tkt}", "orders.store", nothing())
			r.Handle(http.MethodGet, "/types", "", nothing())
		})

		assert.Equal(t, []string{
			`GET /api/v1/orders: name "orders.list" should be one of ` +
				`orders.index, orders.show`,
			`POST /api/v1/orders/{tkt}: POST is not a resource action on this path`,
			`PUT /api/v1/orders/{tkt}: PUT only replaces a singleton's whole content; use PATCH`,
			`POST /api/v1/orders/{tkt}/close: path segment "close" is a verb; use a noun`,
			`GET /api/v1/types: route has no name`,
		}, problems)
	})

	t.Run("skips exceptions and routes outside the prefixes", func(t *testing.T) {
		t.Parallel()
		problems := lint(func(r *routing.Router) {
			r.Handle(http.MethodPost, "/oauth/token", "oauth.token", nothing())
			r.Handle(http.MethodPost, "/mcp", "mcp", nothing())
		})
		r := routing.New(routing.Options{})
		r.Handle(http.MethodPost, "/login", "login", nothing())

		assert.Empty(t, problems)
		assert.Empty(t, routing.Lint(r.Routes(), rules))
	})

	t.Run("checks every route without prefixes", func(t *testing.T) {
		t.Parallel()
		r := routing.New(routing.Options{})
		r.Handle(http.MethodPost, "/login", "login.store", nothing())

		assert.Equal(t, []string{`POST /login: path segment "login" is a verb; use a noun`},
			routing.Lint(r.Routes(), routing.Rules{VerbDenylist: []string{"login"}}))
	})
}

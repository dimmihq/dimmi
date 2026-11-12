import json

import httpx
import pytest
import respx

import dimmi
from dimmi import (
    AsyncDimmi,
    AuthenticationError,
    Dimmi,
    DimmiError,
    NotFoundError,
    PermissionDeniedError,
    RateLimitError,
    ServerError,
    ValidationError,
)
from dimmi._generated.api.default import contacts_index

BASE = "https://dimmi.test/api/v1"


def make(handler, **kwargs):
    sleeps = []
    client = Dimmi(
        token="tok_test",
        workspace="ws_1",
        base_url="https://dimmi.test",
        transport=httpx.MockTransport(handler),
        sleep=sleeps.append,
        **kwargs,
    )
    return client, sleeps


def test_sends_auth_and_workspace_headers_and_decodes_json():
    seen = {}

    def handler(request):
        seen.update(request.headers)
        seen["url"] = str(request.url)
        return httpx.Response(200, json={"data": []})

    client, _ = make(handler)
    assert client.get("/conversations", params={"status": "open", "skip": None}) == {"data": []}
    assert seen["authorization"] == "Bearer tok_test"
    assert seen["dimmi-workspace"] == "ws_1"
    assert seen["user-agent"].startswith("dimmi-python/")
    assert seen["url"] == f"{BASE}/conversations?status=open"


def test_env_fallbacks(monkeypatch):
    monkeypatch.setenv("DIMMI_TOKEN", "tok_env")
    monkeypatch.setenv("DIMMI_WORKSPACE", "ws_env")
    monkeypatch.setenv("DIMMI_BASE_URL", "http://localhost:8080/")
    client = Dimmi()
    assert client.base_url == "http://localhost:8080/api/v1"
    assert client.workspace == "ws_env"
    assert "tok_env" not in repr(client)


def test_missing_token_raises(monkeypatch):
    monkeypatch.delenv("DIMMI_TOKEN", raising=False)
    monkeypatch.delenv("DIMMI_API_TOKEN", raising=False)
    with pytest.raises(ValueError, match="token_missing"):
        Dimmi()


def test_retries_429_honoring_retry_after_with_one_idempotency_key():
    calls = []

    def handler(request):
        calls.append(request.headers.get("idempotency-key"))
        if len(calls) < 3:
            return httpx.Response(429, headers={"Retry-After": "2"}, json={"code": "rate_limited", "message": "slow"})
        assert json.loads(request.content) == {"email": "a@b.test"}
        return httpx.Response(201, json={"data": {"id": "ct_1"}})

    client, sleeps = make(handler)
    assert client.post("/contacts", json={"email": "a@b.test"}) == {"data": {"id": "ct_1"}}
    assert sleeps == [2.0, 2.0]
    assert len(calls) == 3 and calls[0] and len(set(calls)) == 1


def test_retries_503_with_backoff_then_gives_up():
    calls = []

    def handler(request):
        calls.append(1)
        return httpx.Response(503, json={"code": "unavailable", "message": "down"})

    client, sleeps = make(handler, max_retries=2)
    with pytest.raises(ServerError) as info:
        client.get("/conversations")
    assert info.value.status == 503 and info.value.code == "unavailable"
    assert len(calls) == 3 and len(sleeps) == 2 and all(s > 0 for s in sleeps)


def test_rate_limit_error_after_retries_exposes_retry_after():
    client, sleeps = make(lambda r: httpx.Response(429, headers={"Retry-After": "120"}), max_retries=1)
    with pytest.raises(RateLimitError) as info:
        client.get("/x")
    assert info.value.retry_after == 120.0
    assert sleeps == [60.0]  # capped at max_retry_wait


def test_does_not_retry_500_or_422():
    calls = []

    def handler(request):
        calls.append(1)
        return httpx.Response(500, json={"code": "internal", "message": "boom"})

    client, _ = make(handler)
    with pytest.raises(ServerError):
        client.get("/x")
    assert len(calls) == 1


def test_explicit_idempotency_key_is_sent():
    seen = []
    client, _ = make(lambda r: seen.append(r.headers.get("idempotency-key")) or httpx.Response(201, json={}))
    client.post("/contacts", json={}, idempotency_key="key-123")
    client.request("POST", "/contacts", json={}, idempotency_key="key-456")
    assert seen == ["key-123", "key-456"]


def test_no_auto_idempotency_key_without_retries_or_on_get():
    seen = []
    client, _ = make(lambda r: seen.append(r.headers.get("idempotency-key")) or httpx.Response(200, json={}), max_retries=0)
    client.post("/contacts", json={})
    client2, _ = make(lambda r: seen.append(r.headers.get("idempotency-key")) or httpx.Response(200, json={}))
    client2.get("/contacts")
    assert seen == [None, None]


def test_retries_connection_errors():
    calls = []

    def handler(request):
        calls.append(1)
        if len(calls) == 1:
            raise httpx.ConnectError("refused", request=request)
        return httpx.Response(200, json={"ok": True})

    client, sleeps = make(handler)
    assert client.get("/x") == {"ok": True}
    assert len(sleeps) == 1


@pytest.mark.parametrize(
    "status,cls",
    [
        (400, dimmi.BadRequestError),
        (401, AuthenticationError),
        (403, PermissionDeniedError),
        (404, NotFoundError),
        (409, dimmi.ConflictError),
        (422, ValidationError),
        (500, ServerError),
        (502, ServerError),
        (418, DimmiError),
    ],
)
def test_error_mapping(status, cls):
    body = {"code": "c", "message": "m", "errors": {"email": ["is invalid"]}}
    client, _ = make(lambda r: httpx.Response(status, json=body))
    with pytest.raises(cls) as info:
        client.get("/x")
    err = info.value
    assert type(err) is cls and isinstance(err, DimmiError)
    assert (err.status, err.code, err.message) == (status, "c", "m")
    assert err.fields == {"email": ["is invalid"]}


def test_error_accepts_fields_key_and_non_json_body():
    client, _ = make(lambda r: httpx.Response(422, json={"code": "invalid", "message": "bad", "fields": {"name": "required"}}))
    with pytest.raises(ValidationError) as info:
        client.post("/contacts", json={})
    assert info.value.fields == {"name": ["required"]}
    client, _ = make(lambda r: httpx.Response(502, text="Bad Gateway"))
    with pytest.raises(ServerError) as info:
        client.get("/x")
    assert info.value.message == "Bad Gateway"


def test_cursor_pagination():
    pages = {
        None: {"data": [1, 2], "meta": {"next_cursor": "c2"}},
        "c2": {"data": [3], "meta": {"next_cursor": "c3"}},
        "c3": {"data": [], "meta": {"next_cursor": None}},
    }
    seen = []

    def handler(request):
        seen.append(dict(request.url.params))
        return httpx.Response(200, json=pages[request.url.params.get("cursor")])

    client, _ = make(handler)
    assert list(client.paginate("/conversations", {"status": "open"}, limit=2)) == [1, 2, 3]
    assert seen[0] == {"status": "open", "limit": "2"}
    assert seen[1]["cursor"] == "c2"


def test_page_pagination_stops_on_short_page():
    seen = []

    def handler(request):
        page = int(request.url.params["page"])
        seen.append(page)
        data = {1: [1, 2], 2: [3, 4], 3: [5]}[page]
        return httpx.Response(200, json={"data": data})

    client, _ = make(handler)
    assert list(client.pages("/articles", limit=2)) == [1, 2, 3, 4, 5]
    assert seen == [1, 2, 3]


def test_page_pagination_stops_on_empty_page():
    client, _ = make(lambda r: httpx.Response(200, json={"data": [1, 2] if r.url.params["page"] == "1" else []}))
    assert list(client.pages("/articles", limit=2)) == [1, 2]


def test_generated_client_shares_auth_and_errors():
    with respx.mock(base_url=BASE, assert_all_called=True) as mock:
        route = mock.get("/contacts").mock(return_value=httpx.Response(401, json={"code": "unauthenticated", "message": "no"}))
        client = Dimmi(token="tok_gen", workspace="ws_9", base_url="https://dimmi.test")
        with pytest.raises(AuthenticationError):
            contacts_index.sync_detailed(client=client.api)
        sent = route.calls.last.request
        assert sent.headers["authorization"] == "Bearer tok_gen"
        assert sent.headers["dimmi-workspace"] == "ws_9"


async def test_async_client_retries_and_paginates():
    calls = []

    def handler(request):
        calls.append(request.url.params.get("cursor"))
        if len(calls) == 1:
            return httpx.Response(429, headers={"Retry-After": "0"})
        if request.url.params.get("cursor") == "n":
            return httpx.Response(200, json={"data": ["b"], "meta": {"next_cursor": None}})
        return httpx.Response(200, json={"data": ["a"], "meta": {"next_cursor": "n"}})

    async def no_sleep(_):
        return None

    async with AsyncDimmi(
        token="t", base_url="https://dimmi.test", transport=httpx.MockTransport(handler), sleep=no_sleep
    ) as client:
        assert [x async for x in client.paginate("/conversations")] == ["a", "b"]
        with pytest.raises(NotFoundError):
            await AsyncDimmi(
                token="t", base_url="https://dimmi.test", transport=httpx.MockTransport(lambda r: httpx.Response(404, json={}))
            ).get("/missing")

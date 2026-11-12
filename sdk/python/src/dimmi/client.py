"""The dimmi REST client.

``Dimmi`` holds one httpx client that adds the token and workspace
headers, retries 429/503 honoring Retry-After and raises typed errors.
The generated client at ``.api`` shares it, so every operation in the
OpenAPI spec works with the same auth and retries::

    from dimmi import Dimmi
    from dimmi._generated.api.default import conversations_index

    client = Dimmi()  # DIMMI_API_TOKEN, DIMMI_WORKSPACE, DIMMI_BASE_URL
    page = conversations_index.sync(client=client.api)
    contact = client.post("/contacts", json={"email": "ada@example.com"})
    for conversation in client.paginate("/conversations"):
        ...
"""

from __future__ import annotations

import os
from typing import (
    Any,
    AsyncIterator,
    Dict,
    Iterator,
    Mapping,
    Optional,
    Tuple,
    Union,
)

import httpx

from ._generated.client import AuthenticatedClient
from ._transport import AsyncRetryTransport, RetryTransport
from ._version import __version__
from .errors import error_from_response

DEFAULT_BASE_URL = "https://app.trydimmi.com"
DEFAULT_TIMEOUT = 30.0
API_PREFIX = "/api/v1"

Params = Optional[Mapping[str, Any]]
TimeoutTypes = Union[float, httpx.Timeout, None]


def _env(*names: str) -> Optional[str]:
    for name in names:
        value = os.environ.get(name)
        if value:
            return value.strip()
    return None


def _api_root(base_url: str) -> str:
    root = base_url.rstrip("/")
    if not root.endswith(API_PREFIX):
        root += API_PREFIX
    return root


def _clean(params: Params) -> Dict[str, Any]:
    return {k: v for k, v in (params or {}).items() if v is not None}


def _decode(response: httpx.Response) -> Any:
    if response.status_code == 204 or not response.content:
        return None
    if "json" in response.headers.get("content-type", ""):
        return response.json()
    return response.text


def _raise_for_status(response: httpx.Response) -> None:
    if response.status_code >= 400:
        response.read()
        raise error_from_response(response)


async def _araise_for_status(response: httpx.Response) -> None:
    if response.status_code >= 400:
        await response.aread()
        raise error_from_response(response)


def _default_headers() -> Dict[str, str]:
    return {"Accept": "application/json", "User-Agent": f"dimmi-python/{__version__}"}


class _Base:
    def _configure(
        self,
        token: Optional[str],
        workspace: Optional[str],
        base_url: Optional[str],
    ) -> Tuple[str, Dict[str, str]]:
        token = token or _env("DIMMI_API_TOKEN", "DIMMI_TOKEN")
        if not token:
            raise ValueError(
                "[dimmi] token_missing: pass token= or set DIMMI_API_TOKEN to an API token from Settings > API."
            )
        self.workspace = workspace or _env("DIMMI_WORKSPACE")
        self.base_url = _api_root(base_url or _env("DIMMI_BASE_URL") or DEFAULT_BASE_URL)
        headers = {"Authorization": f"Bearer {token}"}
        if self.workspace:
            headers["Dimmi-Workspace"] = self.workspace
        self._token = token
        return token, headers

    @staticmethod
    def _request_headers(
        idempotency_key: Optional[str], headers: Optional[Mapping[str, str]]
    ) -> Dict[str, str]:
        out = dict(headers or {})
        if idempotency_key:
            out["Idempotency-Key"] = idempotency_key
        return out

    def __repr__(self) -> str:  # never shows the token
        return f"{type(self).__name__}(base_url={self.base_url!r}, workspace={self.workspace!r})"


class Dimmi(_Base):
    """A dimmi API client.

    Args:
        token: an API token; defaults to DIMMI_API_TOKEN or DIMMI_TOKEN.
        workspace: the workspace id (ws_...) sent as Dimmi-Workspace;
            defaults to DIMMI_WORKSPACE.
        base_url: https://app.trydimmi.com or your self-hosted origin;
            defaults to DIMMI_BASE_URL. "/api/v1" is added when missing.
        max_retries: retries for 429/503 answers and connection failures.
        timeout: seconds, or an httpx.Timeout.
        transport / async_transport: inner httpx transports, for tests or proxies.
    """

    def __init__(
        self,
        token: Optional[str] = None,
        workspace: Optional[str] = None,
        base_url: Optional[str] = None,
        *,
        max_retries: int = 3,
        timeout: TimeoutTypes = DEFAULT_TIMEOUT,
        max_retry_wait: float = 60.0,
        transport: Optional[httpx.BaseTransport] = None,
        async_transport: Optional[httpx.AsyncBaseTransport] = None,
        sleep: Any = None,
    ) -> None:
        token, headers = self._configure(token, workspace, base_url)
        retry = {"headers": headers, "max_retries": max_retries, "max_retry_wait": max_retry_wait}
        sync_retry = dict(retry, sleep=sleep) if sleep else retry
        self.http = httpx.Client(
            base_url=self.base_url,
            headers=_default_headers(),
            timeout=timeout,
            transport=RetryTransport(transport, **sync_retry),
            event_hooks={"response": [_raise_for_status]},
        )
        self._async_http = httpx.AsyncClient(
            base_url=self.base_url,
            headers=_default_headers(),
            timeout=timeout,
            transport=AsyncRetryTransport(async_transport, **retry),
            event_hooks={"response": [_araise_for_status]},
        )
        self.api = AuthenticatedClient(base_url=self.base_url, token=token)
        self.api.set_httpx_client(self.http)
        self.api.set_async_httpx_client(self._async_http)

    def request(
        self,
        method: str,
        path: str,
        *,
        json: Any = None,
        params: Params = None,
        idempotency_key: Optional[str] = None,
        headers: Optional[Mapping[str, str]] = None,
    ) -> Any:
        """Calls any endpoint and answers its decoded JSON body (None for 204)."""
        response = self.http.request(
            method.upper(),
            path,
            json=json,
            params=_clean(params),
            headers=self._request_headers(idempotency_key, headers),
        )
        return _decode(response)

    def get(self, path: str, *, params: Params = None) -> Any:
        return self.request("GET", path, params=params)

    def post(
        self, path: str, *, json: Any = None, params: Params = None, idempotency_key: Optional[str] = None
    ) -> Any:
        return self.request("POST", path, json=json, params=params, idempotency_key=idempotency_key)

    def put(self, path: str, *, json: Any = None, params: Params = None) -> Any:
        return self.request("PUT", path, json=json, params=params)

    def patch(self, path: str, *, json: Any = None, params: Params = None) -> Any:
        return self.request("PATCH", path, json=json, params=params)

    def delete(self, path: str, *, params: Params = None) -> Any:
        return self.request("DELETE", path, params=params)

    def paginate(self, path: str, params: Params = None, *, limit: int = 100) -> Iterator[Any]:
        """Yields every item of a cursor-paginated list, following meta.next_cursor."""
        query = _clean(params)
        query.setdefault("limit", limit)
        seen = set()
        while True:
            body = self.get(path, params=query) or {}
            yield from body.get("data") or []
            cursor = (body.get("meta") or {}).get("next_cursor")
            if not cursor or cursor in seen:
                return
            seen.add(cursor)
            query["cursor"] = cursor

    def pages(self, path: str, params: Params = None, *, limit: int = 100) -> Iterator[Any]:
        """Yields every item of a page-numbered list, stopping at an empty or short page."""
        query = _clean(params)
        query.setdefault("limit", limit)
        page = int(query.pop("page", 1))
        while True:
            body = self.get(path, params={**query, "page": page}) or {}
            data = body.get("data") or []
            yield from data
            per_page = (body.get("meta") or {}).get("per_page") or query["limit"]
            if not data or len(data) < int(per_page):
                return
            page += 1

    def close(self) -> None:
        self.http.close()

    async def aclose(self) -> None:
        """Closes both the sync client and the async one used by ``.api`` asyncio calls."""
        self.http.close()
        await self._async_http.aclose()

    def __enter__(self) -> "Dimmi":
        return self

    def __exit__(self, *exc: Any) -> None:
        self.close()


class AsyncDimmi(_Base):
    """The asyncio twin of Dimmi; ``.api`` works with the generated ``asyncio`` calls."""

    def __init__(
        self,
        token: Optional[str] = None,
        workspace: Optional[str] = None,
        base_url: Optional[str] = None,
        *,
        max_retries: int = 3,
        timeout: TimeoutTypes = DEFAULT_TIMEOUT,
        max_retry_wait: float = 60.0,
        transport: Optional[httpx.AsyncBaseTransport] = None,
        sleep: Any = None,
    ) -> None:
        token, headers = self._configure(token, workspace, base_url)
        retry: Dict[str, Any] = {"headers": headers, "max_retries": max_retries, "max_retry_wait": max_retry_wait}
        if sleep:
            retry["sleep"] = sleep
        self.http = httpx.AsyncClient(
            base_url=self.base_url,
            headers=_default_headers(),
            timeout=timeout,
            transport=AsyncRetryTransport(transport, **retry),
            event_hooks={"response": [_araise_for_status]},
        )
        self.api = AuthenticatedClient(base_url=self.base_url, token=token)
        self.api.set_async_httpx_client(self.http)

    async def request(
        self,
        method: str,
        path: str,
        *,
        json: Any = None,
        params: Params = None,
        idempotency_key: Optional[str] = None,
        headers: Optional[Mapping[str, str]] = None,
    ) -> Any:
        response = await self.http.request(
            method.upper(),
            path,
            json=json,
            params=_clean(params),
            headers=self._request_headers(idempotency_key, headers),
        )
        return _decode(response)

    async def get(self, path: str, *, params: Params = None) -> Any:
        return await self.request("GET", path, params=params)

    async def post(
        self, path: str, *, json: Any = None, params: Params = None, idempotency_key: Optional[str] = None
    ) -> Any:
        return await self.request("POST", path, json=json, params=params, idempotency_key=idempotency_key)

    async def put(self, path: str, *, json: Any = None, params: Params = None) -> Any:
        return await self.request("PUT", path, json=json, params=params)

    async def patch(self, path: str, *, json: Any = None, params: Params = None) -> Any:
        return await self.request("PATCH", path, json=json, params=params)

    async def delete(self, path: str, *, params: Params = None) -> Any:
        return await self.request("DELETE", path, params=params)

    async def paginate(self, path: str, params: Params = None, *, limit: int = 100) -> AsyncIterator[Any]:
        query = _clean(params)
        query.setdefault("limit", limit)
        seen = set()
        while True:
            body = await self.get(path, params=query) or {}
            for item in body.get("data") or []:
                yield item
            cursor = (body.get("meta") or {}).get("next_cursor")
            if not cursor or cursor in seen:
                return
            seen.add(cursor)
            query["cursor"] = cursor

    async def pages(self, path: str, params: Params = None, *, limit: int = 100) -> AsyncIterator[Any]:
        query = _clean(params)
        query.setdefault("limit", limit)
        page = int(query.pop("page", 1))
        while True:
            body = await self.get(path, params={**query, "page": page}) or {}
            data = body.get("data") or []
            for item in data:
                yield item
            per_page = (body.get("meta") or {}).get("per_page") or query["limit"]
            if not data or len(data) < int(per_page):
                return
            page += 1

    async def aclose(self) -> None:
        await self.http.aclose()

    async def __aenter__(self) -> "AsyncDimmi":
        return self

    async def __aexit__(self, *exc: Any) -> None:
        await self.aclose()

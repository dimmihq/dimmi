"""The httpx transports every dimmi request goes through.

They add the token and workspace headers, give each POST an
Idempotency-Key so a retry can never act twice, and retry 429 and 503
answers (and connection failures) honoring Retry-After. The facade and
the generated client share one httpx client built on these transports.
"""

from __future__ import annotations

import asyncio
import random
import time
import uuid
from typing import Callable, Dict, Optional

import httpx

from .errors import parse_retry_after

RETRY_STATUSES = frozenset({429, 503})
IDEMPOTENT_METHODS = frozenset({"GET", "HEAD", "OPTIONS", "PUT", "DELETE"})


class _Policy:
    def __init__(
        self,
        headers: Dict[str, str],
        max_retries: int,
        max_retry_wait: float,
        backoff: float,
    ) -> None:
        self.headers = headers
        self.max_retries = max(0, int(max_retries))
        self.max_retry_wait = max_retry_wait
        self.backoff = backoff

    def prepare(self, request: httpx.Request) -> None:
        for name, value in self.headers.items():
            if value and name not in request.headers:
                request.headers[name] = value
        if (
            request.method == "POST"
            and self.max_retries > 0
            and "idempotency-key" not in request.headers
        ):
            request.headers["Idempotency-Key"] = str(uuid.uuid4())

    def can_retry_error(self, request: httpx.Request, error: Exception) -> bool:
        if isinstance(error, (httpx.ConnectError, httpx.ConnectTimeout)):
            return True
        if not isinstance(error, (httpx.ReadError, httpx.RemoteProtocolError, httpx.ReadTimeout)):
            return False
        return request.method in IDEMPOTENT_METHODS or "idempotency-key" in request.headers

    def wait(self, attempt: int, response: Optional[httpx.Response]) -> float:
        if response is not None:
            after = parse_retry_after(response.headers.get("retry-after"))
            if after is not None:
                return min(after, self.max_retry_wait)
        delay = self.backoff * (2**attempt)
        return min(delay + random.uniform(0, delay / 2), self.max_retry_wait)


class RetryTransport(httpx.BaseTransport):
    """A sync transport adding dimmi headers and retries around another transport."""

    def __init__(
        self,
        transport: Optional[httpx.BaseTransport] = None,
        *,
        headers: Optional[Dict[str, str]] = None,
        max_retries: int = 3,
        max_retry_wait: float = 60.0,
        backoff: float = 0.5,
        sleep: Callable[[float], None] = time.sleep,
    ) -> None:
        self._transport = transport or httpx.HTTPTransport()
        self._policy = _Policy(headers or {}, max_retries, max_retry_wait, backoff)
        self._sleep = sleep

    def handle_request(self, request: httpx.Request) -> httpx.Response:
        self._policy.prepare(request)
        if self._policy.max_retries > 0:
            request.read()
        attempt = 0
        while True:
            try:
                response = self._transport.handle_request(request)
            except httpx.TransportError as error:
                if attempt >= self._policy.max_retries or not self._policy.can_retry_error(request, error):
                    raise
                self._sleep(self._policy.wait(attempt, None))
                attempt += 1
                continue
            if response.status_code not in RETRY_STATUSES or attempt >= self._policy.max_retries:
                return response
            delay = self._policy.wait(attempt, response)
            response.close()
            self._sleep(delay)
            attempt += 1

    def close(self) -> None:
        self._transport.close()


class AsyncRetryTransport(httpx.AsyncBaseTransport):
    """The async twin of RetryTransport."""

    def __init__(
        self,
        transport: Optional[httpx.AsyncBaseTransport] = None,
        *,
        headers: Optional[Dict[str, str]] = None,
        max_retries: int = 3,
        max_retry_wait: float = 60.0,
        backoff: float = 0.5,
        sleep: Optional[Callable[[float], "asyncio.Future[None]"]] = None,
    ) -> None:
        self._transport = transport or httpx.AsyncHTTPTransport()
        self._policy = _Policy(headers or {}, max_retries, max_retry_wait, backoff)
        self._sleep = sleep or asyncio.sleep

    async def handle_async_request(self, request: httpx.Request) -> httpx.Response:
        self._policy.prepare(request)
        if self._policy.max_retries > 0:
            await request.aread()
        attempt = 0
        while True:
            try:
                response = await self._transport.handle_async_request(request)
            except httpx.TransportError as error:
                if attempt >= self._policy.max_retries or not self._policy.can_retry_error(request, error):
                    raise
                await self._sleep(self._policy.wait(attempt, None))
                attempt += 1
                continue
            if response.status_code not in RETRY_STATUSES or attempt >= self._policy.max_retries:
                return response
            delay = self._policy.wait(attempt, response)
            await response.aclose()
            await self._sleep(delay)
            attempt += 1

    async def aclose(self) -> None:
        await self._transport.aclose()

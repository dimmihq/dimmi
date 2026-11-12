"""Typed errors for API calls.

Every response of 400 or above becomes a DimmiError subclass carrying the
status, the error code and message from the body, and the per-field
messages of a validation failure.
"""

from __future__ import annotations

import json
from typing import Any, Dict, List, Mapping, Optional

import httpx

__all__ = [
    "DimmiError",
    "BadRequestError",
    "AuthenticationError",
    "PermissionDeniedError",
    "NotFoundError",
    "ConflictError",
    "ValidationError",
    "RateLimitError",
    "ServerError",
    "error_from_response",
]


class DimmiError(Exception):
    """An API call dimmi refused or could not answer."""

    def __init__(
        self,
        status: int,
        code: str,
        message: str,
        fields: Optional[Mapping[str, List[str]]] = None,
        *,
        response: Optional[httpx.Response] = None,
        body: Any = None,
    ) -> None:
        super().__init__(f"{status} {code}: {message}" if code else f"{status}: {message}")
        self.status = status
        self.code = code
        self.message = message
        self.fields: Dict[str, List[str]] = dict(fields or {})
        self.response = response
        self.body = body

    @property
    def request_id(self) -> Optional[str]:
        if self.response is None:
            return None
        return self.response.headers.get("x-request-id")


class BadRequestError(DimmiError):
    """400: the request could not be read."""


class AuthenticationError(DimmiError):
    """401: the token is missing, wrong or revoked."""


class PermissionDeniedError(DimmiError):
    """403: the token cannot do this in this workspace."""


class NotFoundError(DimmiError):
    """404: nothing at that path in this workspace."""


class ConflictError(DimmiError):
    """409: the request conflicts with the current state, or an idempotency key was reused."""


class ValidationError(DimmiError):
    """422: one or more fields are invalid; see ``fields``."""


class RateLimitError(DimmiError):
    """429: too many requests, still limited after the retries ran out."""

    @property
    def retry_after(self) -> Optional[float]:
        if self.response is None:
            return None
        return parse_retry_after(self.response.headers.get("retry-after"))


class ServerError(DimmiError):
    """5xx: dimmi failed to answer."""


_BY_STATUS = {
    400: BadRequestError,
    401: AuthenticationError,
    403: PermissionDeniedError,
    404: NotFoundError,
    409: ConflictError,
    422: ValidationError,
    429: RateLimitError,
}


def error_from_response(response: httpx.Response) -> DimmiError:
    """Builds the typed error for a failed response. The body must be read."""
    status = response.status_code
    body: Any = None
    try:
        body = response.json()
    except (json.JSONDecodeError, UnicodeDecodeError, ValueError):
        body = response.text or None
    code, message, fields = "", "", {}
    if isinstance(body, dict):
        err = body.get("error") if isinstance(body.get("error"), dict) else body
        code = str(err.get("code") or "")
        message = str(err.get("message") or err.get("detail") or err.get("title") or "")
        raw_fields = err.get("fields") if err.get("fields") is not None else err.get("errors")
        fields = _fields(raw_fields)
    elif isinstance(body, str):
        message = body[:500]
    if not message:
        message = response.reason_phrase or "request failed"
    cls = _BY_STATUS.get(status)
    if cls is None:
        cls = ServerError if status >= 500 else DimmiError
    return cls(status, code, message, fields, response=response, body=body)


def _fields(raw: Any) -> Dict[str, List[str]]:
    out: Dict[str, List[str]] = {}
    if isinstance(raw, dict):
        for key, value in raw.items():
            if isinstance(value, list):
                out[str(key)] = [str(v) for v in value]
            elif value is not None:
                out[str(key)] = [str(value)]
    elif isinstance(raw, list):
        # [{"field": "email", "message": "..."}]
        for item in raw:
            if isinstance(item, dict) and item.get("field"):
                out.setdefault(str(item["field"]), []).append(str(item.get("message", "")))
    return out


def parse_retry_after(value: Optional[str]) -> Optional[float]:
    """Seconds to wait from a Retry-After header: delta seconds or an HTTP date."""
    if not value:
        return None
    value = value.strip()
    try:
        return max(0.0, float(value))
    except ValueError:
        pass
    from email.utils import parsedate_to_datetime
    import datetime as _dt

    try:
        when = parsedate_to_datetime(value)
    except (TypeError, ValueError):
        return None
    if when.tzinfo is None:
        when = when.replace(tzinfo=_dt.timezone.utc)
    return max(0.0, (when - _dt.datetime.now(_dt.timezone.utc)).total_seconds())

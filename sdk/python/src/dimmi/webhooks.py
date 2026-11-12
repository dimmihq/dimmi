"""Verifying dimmi webhooks.

dimmi signs every delivery with Standard Webhooks: the webhook-id,
webhook-timestamp and webhook-signature headers over the raw body, with
the endpoint's whsec_ secret. Verify the raw bytes before parsing::

    from dimmi import Webhook, WebhookVerificationError

    webhook = Webhook(os.environ["DIMMI_WEBHOOK_SECRET"])
    try:
        event = webhook.verify(await request.body(), request.headers)
    except WebhookVerificationError:
        return Response(status_code=400)
    if event.type == "conversation.created":
        ...
"""

from __future__ import annotations

import datetime as _dt
from dataclasses import dataclass, field
from typing import Any, Dict, Iterable, Mapping, Optional, Tuple, Union

from standardwebhooks.webhooks import Webhook as _StandardWebhook
from standardwebhooks.webhooks import WebhookVerificationError as _StandardError

__all__ = ["Webhook", "WebhookEvent", "WebhookActor", "WebhookVerificationError"]

Headers = Union[Mapping[str, str], Iterable[Tuple[str, str]]]


class WebhookVerificationError(Exception):
    """The request is not a genuine, fresh dimmi delivery."""


@dataclass(frozen=True)
class WebhookActor:
    type: Optional[str] = None
    id: Optional[str] = None


@dataclass(frozen=True)
class WebhookEvent:
    """One verified delivery."""

    id: str
    type: str
    created_at: Optional[_dt.datetime]
    workspace_id: Optional[str]
    data: Dict[str, Any]
    actor: Optional[WebhookActor]
    test: bool
    raw: Dict[str, Any] = field(repr=False)

    @property
    def object(self) -> Any:
        """The resource the event is about: data["object"]."""
        return self.data.get("object")

    @property
    def api_version(self) -> Optional[str]:
        return self.raw.get("api_version")

    @property
    def test_mode(self) -> bool:
        return bool(self.raw.get("test_mode"))

    @property
    def sequence(self) -> Optional[int]:
        return self.raw.get("sequence")


def _parse_time(value: Any) -> Optional[_dt.datetime]:
    if isinstance(value, (int, float)) and not isinstance(value, bool):
        return _dt.datetime.fromtimestamp(value, tz=_dt.timezone.utc)
    if not isinstance(value, str) or not value:
        return None
    text = value[:-1] + "+00:00" if value.endswith(("Z", "z")) else value
    # Python 3.10 reads at most 6 fractional digits.
    if "." in text:
        head, _, rest = text.partition(".")
        digits = len(rest) - len(rest.lstrip("0123456789"))
        text = head + "." + rest[:digits][:6].ljust(6, "0") + rest[digits:]
    try:
        return _dt.datetime.fromisoformat(text)
    except ValueError:
        return None


def _header_dict(headers: Headers) -> Dict[str, str]:
    items = headers.items() if hasattr(headers, "items") else headers
    return {str(k).lower(): str(v) for k, v in items}


class Webhook:
    """Verifies deliveries for one endpoint secret (whsec_...)."""

    def __init__(self, secret: str) -> None:
        if not secret:
            raise ValueError("[dimmi] webhook_secret_missing: pass the endpoint's whsec_ signing secret.")
        try:
            self._webhook = _StandardWebhook(secret.strip())
        except Exception as error:  # bad base64 or empty secret
            raise ValueError("[dimmi] webhook_secret_invalid: the secret must be whsec_ followed by base64.") from error

    def verify(self, payload: Union[bytes, str], headers: Headers) -> WebhookEvent:
        """Checks the signature and timestamp and answers the event.

        Raises WebhookVerificationError for a wrong signature, missing
        headers, a timestamp more than five minutes off, or a body that
        is not a dimmi event.
        """
        try:
            body = self._webhook.verify(payload, _header_dict(headers))
        except _StandardError as error:
            raise WebhookVerificationError(str(error)) from error
        except (ValueError, TypeError) as error:  # malformed signature header or body
            raise WebhookVerificationError(f"invalid webhook: {error}") from error
        if not isinstance(body, dict) or not body.get("type"):
            raise WebhookVerificationError("the body is not a dimmi event")
        actor = body.get("actor")
        return WebhookEvent(
            id=str(body.get("id") or ""),
            type=str(body["type"]),
            created_at=_parse_time(body.get("created_at")),
            workspace_id=body.get("workspace_id"),
            data=body.get("data") if isinstance(body.get("data"), dict) else {},
            actor=WebhookActor(actor.get("type"), actor.get("id")) if isinstance(actor, dict) else None,
            test=bool(body.get("test")),
            raw=body,
        )

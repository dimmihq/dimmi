import base64
import datetime as dt
import json

import pytest
from standardwebhooks import Webhook as StandardWebhook

from dimmi import Webhook, WebhookEvent, WebhookVerificationError

SECRET = "whsec_" + base64.b64encode(b"0123456789abcdef0123456789abcdef").decode()

EVENT = {
    "id": "evt_1",
    "object": "event",
    "type": "conversation.created",
    "api_version": "2026-01-01",
    "created_at": "2026-11-12T10:00:00.123456789Z",
    "workspace_id": "ws_1",
    "sequence": 9,
    "test": True,
    "test_mode": False,
    "truncated": False,
    "actor": {"type": "user", "id": "usr_1"},
    "data": {"object": {"id": "cnv_1"}},
}


def signed(body, msg_id="msg_1", when=None, secret=SECRET):
    when = when or dt.datetime.now(dt.timezone.utc)
    sig = StandardWebhook(secret).sign(msg_id, when, body)
    return {
        "Webhook-Id": msg_id,
        "Webhook-Timestamp": str(int(when.timestamp())),
        "Webhook-Signature": f"v1,AAAA {sig}",
    }


def test_verify_returns_typed_event():
    body = json.dumps(EVENT)
    event = Webhook(SECRET).verify(body.encode(), signed(body))
    assert isinstance(event, WebhookEvent)
    assert (event.id, event.type, event.workspace_id, event.test) == ("evt_1", "conversation.created", "ws_1", True)
    assert event.created_at == dt.datetime(2026, 11, 12, 10, 0, 0, 123456, tzinfo=dt.timezone.utc)
    assert event.actor.type == "user" and event.actor.id == "usr_1"
    assert event.object == {"id": "cnv_1"} and event.sequence == 9 and event.raw == EVENT


def test_verify_accepts_header_pairs_and_str_payload():
    body = json.dumps(EVENT)
    assert Webhook(SECRET).verify(body, list(signed(body).items())).id == "evt_1"


@pytest.mark.parametrize(
    "mutate",
    [
        lambda body, h: (body + " ", h),
        lambda body, h: (body, {**h, "Webhook-Signature": "v1,AAAA"}),
        lambda body, h: (body, {k: v for k, v in h.items() if k != "Webhook-Id"}),
        lambda body, h: (body, {**h, "Webhook-Signature": "garbage"}),
        lambda body, h: (body, signed(body, when=dt.datetime.now(dt.timezone.utc) - dt.timedelta(minutes=10))),
        lambda body, h: (body, signed(body, secret="whsec_" + base64.b64encode(b"other-secret-other-secret").decode())),
    ],
)
def test_verify_rejects(mutate):
    body = json.dumps(EVENT)
    payload, headers = mutate(body, signed(body))
    with pytest.raises(WebhookVerificationError):
        Webhook(SECRET).verify(payload, headers)


def test_rejects_signed_non_event():
    body = json.dumps({"hello": "world"})
    with pytest.raises(WebhookVerificationError):
        Webhook(SECRET).verify(body, signed(body))


def test_bad_secret():
    with pytest.raises(ValueError):
        Webhook("")

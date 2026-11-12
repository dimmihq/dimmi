import datetime as dt
import json
import time
from pathlib import Path

import jwt
import pytest
from cryptography.hazmat.primitives import serialization
from cryptography.hazmat.primitives.asymmetric import ec, ed25519, rsa

from dimmi import (
    IdentityCompany,
    IdentityError,
    IdentityUser,
    messenger_snippet,
    sign_identity_token,
    to_claims,
    token_response,
)

SECRET = "dmi_idsec_" + "x" * 40


def pem(key):
    return key.private_bytes(
        serialization.Encoding.PEM, serialization.PrivateFormat.PKCS8, serialization.NoEncryption()
    ).decode()


@pytest.fixture(autouse=True)
def clean_env(monkeypatch):
    for name in ("DIMMI_IDENTITY_SECRET", "DIMMI_IDENTITY_KID", "DIMMI_IDENTITY_PRIVATE_KEY"):
        monkeypatch.delenv(name, raising=False)


def test_claims_full_user():
    claims = to_claims(
        {
            "id": 42,
            "email": "ada@example.com",
            "email_verified": True,
            "name": "Ada",
            "phone": "",
            "phone_verified": True,
            "avatar_url": None,
            "locale": "en",
            "timezone": "Europe/Berlin",
            "signed_up_at": "2024-01-02T03:04:05Z",
            "attributes": {"plan": "pro", "seats": 3, "trial": False, "note": None},
            "company": {"id": 7, "name": "Acme", "domain": "", "attributes": {"tier": "gold"}},
        }
    )
    assert claims == {
        "sub": "42",
        "email": "ada@example.com",
        "email_verified": True,
        "name": "Ada",
        "locale": "en",
        "timezone": "Europe/Berlin",
        "custom_attributes": {"plan": "pro", "seats": 3, "trial": False, "note": None, "signed_up_at": 1704164645},
        "company": {"id": "7", "name": "Acme", "custom_attributes": {"tier": "gold"}},
    }


def test_email_verified_only_when_set_and_true():
    assert "email_verified" not in to_claims({"id": "u", "email_verified": True})
    assert "email_verified" not in to_claims({"id": "u", "email": "a@b.c"})
    assert "email_verified" not in to_claims({"id": "u", "email": "a@b.c", "email_verified": "yes"})


def test_camel_case_aliases_and_dataclass():
    a = to_claims({"id": "u", "email": "a@b.c", "emailVerified": True, "signedUpAt": 1700000000000})
    b = to_claims(
        IdentityUser(
            id="u", email="a@b.c", email_verified=True, signed_up_at=1700000000,
            company=IdentityCompany(id="c1", name="Co"),
        )
    )
    assert a["custom_attributes"] == {"signed_up_at": 1700000000} == b["custom_attributes"]
    assert a["email_verified"] is True and b["company"] == {"id": "c1", "name": "Co"}


@pytest.mark.parametrize(
    "value,expected",
    [
        (dt.datetime(2024, 1, 1, tzinfo=dt.timezone.utc), 1704067200),
        (dt.datetime(2024, 1, 1), 1704067200),
        (dt.date(2024, 1, 1), 1704067200),
        (1704067200, 1704067200),
        (1704067200123, 1704067200),
        ("2024-01-01T00:00:00+00:00", 1704067200),
        ("not a date", None),
        ("", None),
    ],
)
def test_signed_up_at(value, expected):
    claims = to_claims({"id": "u", "signed_up_at": value})
    assert claims.get("custom_attributes", {}).get("signed_up_at") == expected


@pytest.mark.parametrize(
    "user,code",
    [
        ({}, "identify_user_invalid"),
        ({"id": ""}, "identify_user_invalid"),
        ({"id": "  "}, "identify_user_invalid"),
        ({"id": "x" * 256}, "identify_user_invalid"),
        ({"id": 1.5}, "identify_user_invalid"),
        ({"id": "u", "plan": "pro"}, "user_field_unknown"),
        ({"id": "u", "attributes": {f"k{i}": i for i in range(101)}}, "attribute_limit"),
        ({"id": "u", "attributes": {"PlanName": 1}}, "attribute_key_invalid"),
        ({"id": "u", "attributes": {"plan": {"a": 1}}}, "attribute_value_invalid"),
        ({"id": "u", "attributes": {"plan": [1]}}, "attribute_value_invalid"),
        ({"id": "u", "company": {"name": "x"}}, "identify_user_invalid"),
    ],
)
def test_claim_errors(user, code):
    with pytest.raises(IdentityError) as info:
        to_claims(user)
    assert info.value.code == code
    assert str(info.value).startswith(f"[dimmi] {code}: ")


def test_hs256_round_trip_with_whole_secret_as_key():
    before = int(time.time())
    token = sign_identity_token({"id": "u1", "email": "a@b.c"}, secret=SECRET, kid="isk_1", audience="wgt_1", nonce="n1")
    header = jwt.get_unverified_header(token)
    assert header == {"alg": "HS256", "typ": "JWT", "kid": "isk_1"}
    claims = jwt.decode(token, SECRET, algorithms=["HS256"], audience="wgt_1")
    assert claims["sub"] == "u1" and claims["nonce"] == "n1"
    assert before <= claims["iat"] <= before + 2 and claims["exp"] == claims["iat"] + 600


def test_hs256_from_env_without_kid(monkeypatch):
    monkeypatch.setenv("DIMMI_IDENTITY_SECRET", SECRET + "\n")
    token = sign_identity_token({"id": "u"}, expires_in=60)
    assert "kid" not in jwt.get_unverified_header(token)
    claims = jwt.decode(token, SECRET, algorithms=["HS256"])
    assert claims["exp"] - claims["iat"] == 60 and "aud" not in claims


def test_es256_round_trip():
    key = ec.generate_private_key(ec.SECP256R1())
    token = sign_identity_token({"id": "u"}, private_key=pem(key), kid="isk_es")
    assert jwt.get_unverified_header(token) == {"alg": "ES256", "typ": "JWT", "kid": "isk_es"}
    assert jwt.decode(token, key.public_key(), algorithms=["ES256"])["sub"] == "u"


def test_es256_with_repo_key_if_present():
    path = Path(__file__).resolve().parents[3] / "tmp" / "sdk-matrix" / "es256.pem"
    if not path.exists():
        pytest.skip("no shared ES256 key")
    key = serialization.load_pem_private_key(path.read_bytes(), None)
    token = sign_identity_token({"id": "u"}, private_key=path.read_text(), kid="isk_x")
    assert jwt.decode(token, key.public_key(), algorithms=["ES256"])["sub"] == "u"


def test_rs256_and_eddsa_round_trip(monkeypatch):
    rsa_key = rsa.generate_private_key(public_exponent=65537, key_size=2048)
    token = sign_identity_token({"id": "u"}, private_key=pem(rsa_key), kid="k")
    assert jwt.get_unverified_header(token)["alg"] == "RS256"
    jwt.decode(token, rsa_key.public_key(), algorithms=["RS256"])
    ed_key = ed25519.Ed25519PrivateKey.generate()
    monkeypatch.setenv("DIMMI_IDENTITY_PRIVATE_KEY", pem(ed_key))
    monkeypatch.setenv("DIMMI_IDENTITY_KID", "k2")
    token = sign_identity_token({"id": "u"})
    assert jwt.get_unverified_header(token) == {"alg": "EdDSA", "typ": "JWT", "kid": "k2"}
    jwt.decode(token, ed_key.public_key(), algorithms=["EdDSA"])


def test_key_errors():
    key = ec.generate_private_key(ec.SECP256R1())
    cases = [
        (dict(private_key=pem(key)), "kid_required"),
        (dict(), "secret_missing"),
        (dict(private_key="-----BEGIN PUBLIC KEY-----\nAAA\n-----END PUBLIC KEY-----", kid="k"), "private_key_invalid"),
        (dict(private_key="garbage", kid="k"), "private_key_invalid"),
        (dict(private_key=pem(key), kid="k", algorithm="RS256"), "private_key_invalid"),
        (dict(secret=SECRET, expires_in=0), "expires_in_invalid"),
        (dict(secret=SECRET, expires_in=3601), "expires_in_invalid"),
    ]
    for kwargs, code in cases:
        with pytest.raises(IdentityError) as info:
            sign_identity_token({"id": "u"}, **kwargs)
        assert info.value.code == code, kwargs


def test_token_response():
    body, headers = token_response("abc")
    assert json.loads(body) == {"token": "abc"}
    assert headers == {"Content-Type": "application/json", "Cache-Control": "no-store, private"}
    assert json.loads(token_response(None)[0]) == {"token": None}


def test_messenger_snippet_with_token_escapes():
    html = messenger_snippet("wgt_1", "http://localhost:8098/", token="a.b</script>c", nonce='n"1')
    assert 'Dimmi("identify",function(){return "a.b\\u003c/script\\u003ec"})' in html
    assert '<script async src="http://localhost:8098/widget/v1/loader.js" data-app-id="wgt_1" nonce="n&quot;1"></script>' in html
    assert html.count('nonce="n&quot;1"') == 2
    assert "</script>c" not in html


def test_messenger_snippet_with_identity_url():
    html = messenger_snippet("wgt_1", "https://app.trydimmi.com", identity_url="/dimmi/token?a=1&b=2")
    assert html == (
        '<script async src="https://app.trydimmi.com/widget/v1/loader.js" data-app-id="wgt_1" '
        'data-identity-url="/dimmi/token?a=1&amp;b=2"></script>'
    )

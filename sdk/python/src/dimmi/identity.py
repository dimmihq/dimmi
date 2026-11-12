"""Identity tokens for the dimmi messenger.

Your backend signs a short-lived JWT naming the signed-in user; the
messenger hands it to dimmi, which trusts the user without a login. You
pass a plain user (a dict or IdentityUser), never claims: the id is
required, an email counts as verified only when you say so, attributes
are flat values under snake_case keys, and empty fields are left out.

A shared secret (dmi_idsec_..., DIMMI_IDENTITY_SECRET) signs HS256 with
the whole string as the key. A PKCS #8 PEM private key
(DIMMI_IDENTITY_PRIVATE_KEY) signs ES256, RS256 or EdDSA, whichever it
is, and needs the key id dimmi showed (DIMMI_IDENTITY_KID).
"""

from __future__ import annotations

import datetime as _dt
import html
import json
import os
import re
import time
from dataclasses import asdict, dataclass, field
from typing import Any, Dict, Mapping, Optional, Tuple, Union

import jwt

__all__ = [
    "IdentityError",
    "IdentityUser",
    "IdentityCompany",
    "to_claims",
    "sign_identity_token",
    "messenger_snippet",
    "token_response",
    "TOKEN_HEADERS",
]

AttributeValue = Union[str, int, float, bool, None]
_ATTRIBUTE_KEY = re.compile(r"^[a-z0-9_]{1,64}$")
_ALGORITHMS = ("HS256", "RS256", "ES256", "EdDSA")

TOKEN_HEADERS: Dict[str, str] = {
    "Content-Type": "application/json",
    "Cache-Control": "no-store, private",
}


class IdentityError(ValueError):
    """A user or key that cannot become a token, with a stable code and the fix."""

    def __init__(self, code: str, fix: str) -> None:
        super().__init__(f"[dimmi] {code}: {fix}")
        self.code = code


@dataclass
class IdentityCompany:
    id: Union[str, int]
    name: Optional[str] = None
    domain: Optional[str] = None
    attributes: Dict[str, AttributeValue] = field(default_factory=dict)


@dataclass
class IdentityUser:
    id: Union[str, int]
    email: Optional[str] = None
    email_verified: bool = False
    name: Optional[str] = None
    phone: Optional[str] = None
    phone_verified: bool = False
    avatar_url: Optional[str] = None
    locale: Optional[str] = None
    timezone: Optional[str] = None
    signed_up_at: Union[_dt.datetime, _dt.date, int, float, str, None] = None
    company: Union[IdentityCompany, Mapping[str, Any], None] = None
    attributes: Dict[str, AttributeValue] = field(default_factory=dict)


# snake_case is the Python spelling; the camelCase names of the other
# dimmi SDKs are accepted too so a user object can be shared.
_ALIASES = {
    "emailVerified": "email_verified",
    "phoneVerified": "phone_verified",
    "avatarUrl": "avatar_url",
    "signedUpAt": "signed_up_at",
}
_KNOWN = {
    "id",
    "email",
    "email_verified",
    "name",
    "phone",
    "phone_verified",
    "avatar_url",
    "locale",
    "timezone",
    "signed_up_at",
    "company",
    "attributes",
}


def _user_dict(user: Union[IdentityUser, Mapping[str, Any]]) -> Dict[str, Any]:
    if isinstance(user, IdentityUser):
        return {k: v for k, v in asdict(user).items()}
    if not isinstance(user, Mapping):
        raise IdentityError("identify_user_invalid", "pass the user as a dict or IdentityUser.")
    out: Dict[str, Any] = {}
    for key, value in user.items():
        name = _ALIASES.get(key, key)
        if name not in _KNOWN:
            raise IdentityError("user_field_unknown", f"put {key} under attributes.")
        out[name] = value
    return out


def _string_id(value: Any) -> Optional[str]:
    if isinstance(value, bool):
        return None
    if isinstance(value, int):
        return str(value)
    if isinstance(value, str):
        return value
    return None


def to_claims(user: Union[IdentityUser, Mapping[str, Any]]) -> Dict[str, Any]:
    """Turns a user into token claims, raising IdentityError for a user dimmi refuses."""
    raw_id = user.id if isinstance(user, IdentityUser) else (user.get("id") if isinstance(user, Mapping) else None)
    sub = _string_id(raw_id)
    if sub is None or sub.strip() == "" or len(sub) > 255:
        raise IdentityError("identify_user_invalid", "the user needs an id: a string or number, 1 to 255 long.")
    data = _user_dict(user)
    claims: Dict[str, Any] = {"sub": sub}
    _set(claims, "email", data.get("email"))
    if data.get("email") and data.get("email_verified") is True:
        claims["email_verified"] = True
    _set(claims, "name", data.get("name"))
    _set(claims, "phone", data.get("phone"))
    if data.get("phone") and data.get("phone_verified") is True:
        claims["phone_verified"] = True
    _set(claims, "avatar_url", data.get("avatar_url"))
    _set(claims, "locale", data.get("locale"))
    _set(claims, "timezone", data.get("timezone"))
    attributes = dict(_check_attributes(data.get("attributes") or {}))
    signed_up = _epoch_seconds(data.get("signed_up_at"))
    if signed_up is not None:
        attributes["signed_up_at"] = signed_up
    if attributes:
        claims["custom_attributes"] = attributes
    company = data.get("company")
    if company is not None:
        claims["company"] = _company_claims(company)
    return claims


def _set(claims: Dict[str, Any], name: str, value: Any) -> None:
    if isinstance(value, str) and value != "":
        claims[name] = value


def _company_claims(company: Union[IdentityCompany, Mapping[str, Any]]) -> Dict[str, Any]:
    if isinstance(company, IdentityCompany):
        data: Mapping[str, Any] = asdict(company)
    elif isinstance(company, Mapping):
        data = company
        for key in data:
            if key not in ("id", "name", "domain", "attributes"):
                raise IdentityError("user_field_unknown", f"put company {key} under company attributes.")
    else:
        raise IdentityError("identify_user_invalid", "company must be a dict or IdentityCompany.")
    cid = _string_id(data.get("id"))
    if not cid:
        raise IdentityError("identify_user_invalid", "company needs an id.")
    out: Dict[str, Any] = {"id": cid}
    _set(out, "name", data.get("name"))
    _set(out, "domain", data.get("domain"))
    attributes = data.get("attributes") or {}
    if attributes:
        out["custom_attributes"] = _check_attributes(attributes)
    return out


def _check_attributes(attributes: Mapping[str, Any]) -> Dict[str, AttributeValue]:
    if not isinstance(attributes, Mapping):
        raise IdentityError("attribute_value_invalid", "attributes must be a dict of flat values.")
    if len(attributes) > 100:
        raise IdentityError("attribute_limit", "send at most 100 attributes; update others through the API.")
    for key, value in attributes.items():
        if not isinstance(key, str) or not _ATTRIBUTE_KEY.match(key):
            raise IdentityError("attribute_key_invalid", f"use snake_case keys such as plan_name, not {key}.")
        if value is not None and not isinstance(value, (str, int, float, bool)):
            raise IdentityError("attribute_value_invalid", f"{key} must be a string, number, boolean or null.")
    return dict(attributes)


def _epoch_seconds(value: Any) -> Optional[int]:
    if value is None or value == "" or isinstance(value, bool):
        return None
    if isinstance(value, _dt.datetime):
        if value.tzinfo is None:
            value = value.replace(tzinfo=_dt.timezone.utc)
        return int(value.timestamp() // 1)
    if isinstance(value, _dt.date):
        return int(_dt.datetime(value.year, value.month, value.day, tzinfo=_dt.timezone.utc).timestamp())
    if isinstance(value, (int, float)):
        return int(value // 1000) if value > 1e11 else int(value // 1)
    if isinstance(value, str):
        text = value.strip()
        if text.endswith("Z") or text.endswith("z"):
            text = text[:-1] + "+00:00"
        try:
            parsed = _dt.datetime.fromisoformat(text)
        except ValueError:
            return None
        return _epoch_seconds(parsed)
    return None


def _env(name: str) -> Optional[str]:
    value = os.environ.get(name)
    return value if value else None


def _signing_key(
    secret: Optional[str], kid: Optional[str], private_key: Optional[Union[str, bytes]], algorithm: Optional[str]
) -> Tuple[Any, str, Optional[str]]:
    kid = kid or _env("DIMMI_IDENTITY_KID")
    pem = private_key or _env("DIMMI_IDENTITY_PRIVATE_KEY")
    if algorithm is not None and algorithm not in _ALGORITHMS:
        raise IdentityError("private_key_invalid", "algorithm is HS256, ES256, RS256 or EdDSA.")
    if pem:
        if not kid:
            raise IdentityError(
                "kid_required", "set DIMMI_IDENTITY_KID to the key id dimmi shows for your public key."
            )
        key, alg = _load_private_key(pem, algorithm)
        return key, alg, kid
    secret = secret or _env("DIMMI_IDENTITY_SECRET")
    if not secret:
        raise IdentityError(
            "secret_missing", "set DIMMI_IDENTITY_SECRET to the secret from Settings > Identity verification."
        )
    if algorithm not in (None, "HS256"):
        raise IdentityError("private_key_invalid", f"{algorithm} needs private_key=; a secret signs HS256.")
    return secret.strip(), "HS256", kid


def _load_private_key(pem: Union[str, bytes], algorithm: Optional[str]) -> Tuple[Any, str]:
    from cryptography.hazmat.primitives.asymmetric import ec, ed25519, rsa
    from cryptography.hazmat.primitives.serialization import load_pem_private_key

    data = pem.encode() if isinstance(pem, str) else pem
    if re.search(rb"BEGIN (RSA |EC )?PUBLIC KEY", data):
        raise IdentityError("private_key_invalid", "that is a public key; sign with the private key.")
    invalid = IdentityError("private_key_invalid", "the private key must be PKCS #8 PEM: Ed25519, P-256 or RSA.")
    try:
        key = load_pem_private_key(data.strip() + b"\n", password=None)
    except (ValueError, TypeError) as error:
        raise invalid from error
    if isinstance(key, ed25519.Ed25519PrivateKey):
        found = "EdDSA"
    elif isinstance(key, ec.EllipticCurvePrivateKey) and isinstance(key.curve, ec.SECP256R1):
        found = "ES256"
    elif isinstance(key, rsa.RSAPrivateKey):
        found = "RS256"
    else:
        raise invalid
    if algorithm and algorithm != found:
        raise IdentityError("private_key_invalid", f"the key signs {found}, not {algorithm}.")
    return key, found


def sign_identity_token(
    user: Union[IdentityUser, Mapping[str, Any]],
    secret: Optional[str] = None,
    kid: Optional[str] = None,
    private_key: Optional[Union[str, bytes]] = None,
    algorithm: Optional[str] = None,
    expires_in: int = 600,
    audience: Optional[str] = None,
    nonce: Optional[str] = None,
) -> str:
    """Signs an identity token for the user.

    Args:
        user: a dict or IdentityUser; ``id`` is required.
        secret: the dmi_idsec_ secret; defaults to DIMMI_IDENTITY_SECRET.
        kid: the key id; defaults to DIMMI_IDENTITY_KID. Required with a private key.
        private_key: a PKCS #8 PEM private key; defaults to DIMMI_IDENTITY_PRIVATE_KEY.
        algorithm: ES256, RS256 or EdDSA; found from the key when left out.
        expires_in: seconds the token lives, 1 to 3600.
        audience: the aud claim: "dimmi", a messenger app id or a workspace id.
        nonce: a one-time value, for tokens that must work only once.
    """
    if isinstance(expires_in, bool) or not isinstance(expires_in, int) or expires_in <= 0 or expires_in > 3600:
        raise IdentityError("expires_in_invalid", "expires_in is 1 to 3600 seconds.")
    claims = to_claims(user)
    now = int(time.time())
    claims["iat"] = now
    claims["exp"] = now + expires_in
    if audience:
        claims["aud"] = audience
    if nonce:
        claims["nonce"] = nonce
    key, alg, key_id = _signing_key(secret, kid, private_key, algorithm)
    headers: Dict[str, str] = {"typ": "JWT"}
    if key_id:
        headers["kid"] = key_id
    return jwt.encode(claims, key, algorithm=alg, headers=headers)


def token_response(token: Optional[str]) -> Tuple[str, Dict[str, str]]:
    """The answer the messenger expects from your token endpoint: a body and headers.

    Serve it with status 200, also when nobody is signed in (token None).
    """
    return json.dumps({"token": token}), dict(TOKEN_HEADERS)


def _script_json(value: str) -> str:
    # JSON is valid JavaScript; escaping <, > and & keeps "</script>" out.
    return json.dumps(value).replace("<", "\\u003c").replace(">", "\\u003e").replace("&", "\\u0026")


def messenger_snippet(
    app_id: str,
    host: str = "https://app.trydimmi.com",
    token: Optional[str] = None,
    identity_url: Optional[str] = None,
    nonce: Optional[str] = None,
) -> str:
    """The HTML that boots the messenger, identified by a token or an identity URL.

    Args:
        app_id: the messenger app id, wgt_...
        host: the dimmi origin serving /widget/v1/loader.js.
        token: an identity token signed for the signed-in user.
        identity_url: a path such as /dimmi/token the messenger fetches a fresh token from.
        nonce: a CSP nonce for both script tags.
    """
    nonce_attr = f' nonce="{html.escape(nonce, quote=True)}"' if nonce else ""
    parts = []
    if token:
        parts.append(
            f"<script{nonce_attr}>window.Dimmi=window.Dimmi||function(){{(Dimmi.q=Dimmi.q||[]).push([].slice.call(arguments))}};"
            f"Dimmi(\"identify\",function(){{return {_script_json(token)}}});</script>"
        )
    loader = html.escape(host.rstrip("/") + "/widget/v1/loader.js", quote=True)
    attrs = f' data-app-id="{html.escape(app_id, quote=True)}"'
    if identity_url:
        attrs += f' data-identity-url="{html.escape(identity_url, quote=True)}"'
    parts.append(f'<script async src="{loader}"{attrs}{nonce_attr}></script>')
    return "\n".join(parts)

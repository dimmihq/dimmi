"""dimmi: the Python backend SDK for dimmi.

- ``Dimmi`` / ``AsyncDimmi``: the REST API with auth, retries, pagination
  and typed errors; ``.api`` is the generated client for every operation.
- ``sign_identity_token`` / ``messenger_snippet`` / ``token_response``:
  identify signed-in users to the messenger.
- ``Webhook``: verify signed webhook deliveries.
"""

from ._version import __version__
from .client import AsyncDimmi, Dimmi
from .errors import (
    AuthenticationError,
    BadRequestError,
    ConflictError,
    DimmiError,
    NotFoundError,
    PermissionDeniedError,
    RateLimitError,
    ServerError,
    ValidationError,
)
from .identity import (
    IdentityCompany,
    IdentityError,
    IdentityUser,
    TOKEN_HEADERS,
    messenger_snippet,
    sign_identity_token,
    to_claims,
    token_response,
)
from .webhooks import Webhook, WebhookActor, WebhookEvent, WebhookVerificationError

__all__ = [
    "__version__",
    "Dimmi",
    "AsyncDimmi",
    "DimmiError",
    "BadRequestError",
    "AuthenticationError",
    "PermissionDeniedError",
    "NotFoundError",
    "ConflictError",
    "ValidationError",
    "RateLimitError",
    "ServerError",
    "IdentityError",
    "IdentityUser",
    "IdentityCompany",
    "TOKEN_HEADERS",
    "sign_identity_token",
    "to_claims",
    "messenger_snippet",
    "token_response",
    "Webhook",
    "WebhookEvent",
    "WebhookActor",
    "WebhookVerificationError",
]

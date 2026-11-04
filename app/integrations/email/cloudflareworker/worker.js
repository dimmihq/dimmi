// Hands each mail Cloudflare Email Routing receives to the install, signed
// with the shared secret. When the install stays unreachable the mail goes
// to FALLBACK_ADDRESS when set, otherwise the sender gets a bounce, so no
// mail is ever lost silently.

const ATTEMPTS = 3;
const RETRY_DELAY_MS = 1000;

function hex(buffer) {
  return [...new Uint8Array(buffer)].map((byte) => byte.toString(16).padStart(2, "0")).join("");
}

async function sign(secret, timestamp, raw) {
  const digest = hex(await crypto.subtle.digest("SHA-256", raw));
  const key = await crypto.subtle.importKey(
    "raw",
    new TextEncoder().encode(secret),
    { name: "HMAC", hash: "SHA-256" },
    false,
    ["sign"],
  );
  const signed = new TextEncoder().encode(`${timestamp}.${digest}`);

  return hex(await crypto.subtle.sign("HMAC", key, signed));
}

async function post(message, raw, env) {
  const timestamp = Math.floor(Date.now() / 1000).toString();
  const response = await fetch(env.WEBHOOK_URL, {
    method: "POST",
    headers: {
      "Content-Type": "message/rfc822",
      "X-Inbound-Timestamp": timestamp,
      "X-Inbound-Signature": await sign(env.INBOUND_SECRET, timestamp, raw),
      "X-Inbound-Envelope-From": message.from,
      "X-Inbound-Envelope-To": message.to,
      "X-Inbound-Fallback": env.FALLBACK_ADDRESS ? "on" : "off",
    },
    body: raw,
  });

  return response.ok;
}

export default {
  async email(message, env) {
    const raw = await new Response(message.raw).arrayBuffer();

    for (let attempt = 0; attempt < ATTEMPTS; attempt += 1) {
      try {
        if (await post(message, raw, env)) {
          return;
        }
      } catch (error) {
        console.error("inbound post failed", error);
      }

      await new Promise((resolve) => setTimeout(resolve, RETRY_DELAY_MS));
    }

    if (env.FALLBACK_ADDRESS) {
      await message.forward(env.FALLBACK_ADDRESS);

      return;
    }

    message.setReject("temporarily unavailable, try again later");
  },
};

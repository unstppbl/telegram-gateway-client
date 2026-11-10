# Security policy

## Supported versions

Security fixes target the latest published release. Upgrade to the latest
release when reporting or investigating an issue. This package is pre-1.0;
review release notes when upgrading.

## Report a vulnerability

Use [GitHub's private vulnerability reporting](https://github.com/unstppbl/telegram-gateway-client/security/advisories/new)
to report a security issue privately. Include the affected version, impact,
reproduction steps, and a minimal example with synthetic data.

If private reporting is unavailable, open an issue requesting a private contact
channel without disclosing the vulnerability. Never post access tokens or
personal data in public reports.

## Using the client

- Store Gateway access tokens in a secret manager or environment variable.
- Use trusted HTTPS endpoints and HTTP transports. The default client sends
  tokens in the authorization header and rejects redirects.
- Set request deadlines appropriate to your application. A timeout after a send
  does not tell you whether Telegram accepted it.
- Authenticate delivery reports with `DecodeReport` before trusting their
  contents. Use a positive maximum age and keep the server clock synchronized.
- Make callback processing idempotent. Signature verification does not prevent
  replay during the accepted timestamp window.
- Treat codes, phone numbers, request IDs, and callback payloads as sensitive.
  Keep them out of public logs and bind verification requests to the correct
  application session.

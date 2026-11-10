# Changelog

## v0.1.0 — 2026-11-10

Initial public release.

- Standalone Go client for all four Telegram Gateway API methods.
- Typed send parameters and delivery, verification, cost, balance, and refund
  response fields.
- Signed delivery report decoding with timestamp freshness checks and bounded
  request bodies.
- Inspectable API and HTTP errors, context cancellation, configurable HTTP
  clients, and a default 10-second timeout.
- Standard-library-only implementation, examples, MIT license, and GitHub CI.

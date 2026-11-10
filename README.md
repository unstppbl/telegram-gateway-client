<p align="center">
  <img src="assets/header.svg" alt="Telegram Gateway for Go — verification, delivered" width="100%">
</p>

<p align="center">
  <a href="https://github.com/unstppbl/telegram-gateway-client/actions/workflows/ci.yml"><img src="https://github.com/unstppbl/telegram-gateway-client/actions/workflows/ci.yml/badge.svg" alt="CI"></a>
  <a href="https://pkg.go.dev/github.com/unstppbl/telegram-gateway-client"><img src="https://pkg.go.dev/badge/github.com/unstppbl/telegram-gateway-client.svg" alt="Go Reference"></a>
  <a href="https://github.com/unstppbl/telegram-gateway-client/releases"><img src="https://img.shields.io/github/v/release/unstppbl/telegram-gateway-client?color=229ed9" alt="Release"></a>
  <a href="LICENSE"><img src="https://img.shields.io/badge/license-MIT-229ed9" alt="MIT license"></a>
</p>

<p align="center">
  <strong>Send verification codes through Telegram with a small, idiomatic Go package.</strong><br>
  Complete Gateway API coverage, typed responses, and authenticated delivery reports.<br>
  Built entirely on the Go standard library.
</p>

<p align="center">
  <a href="#quickstart">Quickstart</a> ·
  <a href="#api-coverage">API coverage</a> ·
  <a href="#delivery-reports">Delivery reports</a> ·
  <a href="https://pkg.go.dev/github.com/unstppbl/telegram-gateway-client">Go reference</a>
</p>

---

## Why this package

| Feature | What you get |
| --- | --- |
| Complete API | Send codes, check send ability, verify codes, and request revocation. |
| Flexible verification | Telegram-generated or application-generated codes; every documented send option. |
| Typed status | Delivery and verification states, request costs, balances, and refund information. |
| Signed callbacks | HMAC verification, timestamp freshness checks, and bounded report bodies. |
| Predictable HTTP | Context cancellation, a default 10-second timeout, custom HTTP clients, and no redirects. |
| Small footprint | No third-party runtime dependencies. Safe to share a configured client between goroutines. |

This is an independent client for the [Telegram Gateway API](https://core.telegram.org/gateway/api). It is not affiliated with Telegram. Gateway delivers phone verification messages to Telegram users; obtain your access token from [Telegram Gateway](https://gateway.telegram.org/).

## Install

Requires **Go 1.26 or later**.

```sh
go get github.com/unstppbl/telegram-gateway-client@v0.1.0
```

## Quickstart

Keep your access token in an environment variable or secret manager.

```go
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	gateway "github.com/unstppbl/telegram-gateway-client"
)

func main() {
	client, err := gateway.New(os.Getenv("TELEGRAM_GATEWAY_TOKEN"))
	if err != nil {
		log.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	status, err := client.SendVerificationMessage(ctx, gateway.SendVerificationMessageParams{
		PhoneNumber: "+12025550123", // Replace with the user's E.164 phone number.
		CodeLength:  6,
		TTL:         300, // Seconds.
	})
	if err != nil {
		log.Fatal(err)
	}

	// Persist this ID with your application's verification session.
	fmt.Println(status.RequestID)
}
```

Once the user enters their code, verify it against the saved request:

```go
status, err := client.CheckVerificationStatus(ctx, gateway.CheckVerificationStatusParams{
	RequestID: savedRequestID,
	Code:      userEnteredCode,
})
if err != nil {
	return err
}
verified := status.VerificationStatus != nil &&
	status.VerificationStatus.Status == gateway.VerificationCodeValid
```

Use a fresh context for each interaction. A successful API request does not itself mean that a code is valid: check `VerificationStatus.Status` before authenticating the user.

## API coverage

| Go method | Telegram method | Result |
| --- | --- | --- |
| `SendVerificationMessage(ctx, params)` | `sendVerificationMessage` | `RequestStatus` |
| `CheckSendAbility(ctx, phoneNumber)` | `checkSendAbility` | `RequestStatus` |
| `CheckVerificationStatus(ctx, params)` | `checkVerificationStatus` | `RequestStatus` |
| `RevokeVerificationMessage(ctx, requestID)` | `revokeVerificationMessage` | `bool` |

All methods accept a context and return an error. Responses preserve optional fields: `IsRefunded` and `RemainingBalance` are pointers so an absent value remains distinct from `false` or zero. Status strings remain representable if Telegram adds new values.

### Sending options

`SendVerificationMessageParams` exposes these fields:

| Field | Purpose |
| --- | --- |
| `PhoneNumber` | Required recipient in E.164 format. |
| `RequestID` | Reuse a successful `CheckSendAbility` request. |
| `SenderUsername` | Use an eligible verified channel owned by your Gateway account. |
| `Code` | Supply your own numeric code, 4–8 digits. Takes precedence over `CodeLength`. |
| `CodeLength` | Ask Telegram to generate a code of 4–8 digits. |
| `CallbackURL` | HTTPS endpoint for signed delivery reports; up to 256 bytes. |
| `Payload` | Application metadata; up to 128 bytes. |
| `TTL` | Message lifetime in seconds, 30–3600. Zero leaves it unspecified. |

Choose `Code` or `CodeLength` explicitly. Omitted optional fields are left to Telegram's API behavior. For exact provider semantics, see the [official API reference](https://core.telegram.org/gateway/api#sendverificationmessage).

### Checking ability and reusing a request

```go
ability, err := client.CheckSendAbility(ctx, phoneNumber)
if err != nil {
	return err
}
status, err := client.SendVerificationMessage(ctx, gateway.SendVerificationMessageParams{
	PhoneNumber: phoneNumber,
	RequestID:   ability.RequestID,
	CodeLength:  6,
})
```

`CheckSendAbility` can incur a charge. Pass its `RequestID` when sending to use that paid request; do not treat this method as a free health check. The client performs no automatic retries because replaying a send without a reusable request can create another charge. Decide how to handle an uncertain network result in your application.

`RevokeVerificationMessage` returning `true` acknowledges a revocation request. It does not guarantee removal of a message already delivered or read. See [Telegram's delivery and revocation rules](https://core.telegram.org/gateway/api).

## Delivery reports

Supply `CallbackURL` when sending, then verify the incoming report **before using its contents**:

```go
func reportHandler(client *gateway.Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		report, err := client.DecodeReport(r, 5*time.Minute)
		if err != nil {
			http.Error(w, "invalid report", http.StatusBadRequest)
			return
		}

		// Persist the update idempotently, keyed by report.RequestID and status.
		// Return a failure response if your durable processing fails.
		_ = report
		w.WriteHeader(http.StatusOK)
	}
}
```

This snippet uses `net/http` and `time`. `DecodeReport` checks the signature over the original bytes, accepts POST bodies up to 1 MiB, and checks `X-Request-Timestamp` against the supplied positive maximum age. Reports more than 30 seconds in the future are rejected. Keep your server clock synchronized and choose the age window for your environment.

The helper consumes the request body. It returns `ErrInvalidSignature` or `ErrExpiredReport` for signature and freshness failures, usable with `errors.Is`. It does not store report IDs or prevent replay inside the accepted time window. Telegram can redeliver a report, so apply updates idempotently and return HTTP 200 only after processing succeeds. A delivery report alone is not proof that a user entered the correct code.

## Errors and HTTP configuration

Use `errors.As` to inspect provider errors without parsing strings:

```go
var apiErr *gateway.APIError
if errors.As(err, &apiErr) {
	fmt.Printf("Telegram rejected %s: %s (HTTP %d)\n",
		apiErr.Method, apiErr.Code, apiErr.StatusCode)
}

var httpErr *gateway.HTTPError
if errors.As(err, &httpErr) {
	fmt.Printf("HTTP failure: %d\n", httpErr.StatusCode)
}

if errors.Is(err, context.DeadlineExceeded) {
	// Apply your application's timeout policy.
}
```

`APIError` contains Telegram's error code. `HTTPError` represents an unsuccessful HTTP response without a usable API error. Transport and context errors remain unwrap-compatible. The client never retries a request automatically.

```go
client, err := gateway.New(token,
	gateway.WithHTTPClient(&http.Client{Timeout: 15 * time.Second}),
)
```

The client copies the supplied `http.Client` settings and disables redirects. Its transport remains shared; configure it before use and do not mutate it concurrently. `WithBaseURL` supports a custom HTTPS endpoint or local HTTP test server; only use endpoints and transports you trust with the token.

## Examples and development

The [send example](examples/send/main.go) reads its token and recipient from the
environment. From a clone of this repository:

```sh
export TELEGRAM_GATEWAY_TOKEN='your-gateway-token'
export TELEGRAM_PHONE_NUMBER='+12025550123'
go run ./examples/send
```

Replace the example phone number with your intended recipient. Running this
example makes a real send request and can incur a Gateway charge.

See the [Go reference](https://pkg.go.dev/github.com/unstppbl/telegram-gateway-client) for exported types and examples. Run the test suite without a Telegram account or access token:

```sh
go test -race -cover ./...
go vet ./...
```

Tests use local HTTP servers and signed report fixtures. CI checks formatting, tests Go 1.26 and 1.27 on Linux, runs a Windows test job, and scans for reachable Go vulnerabilities. See [Contributing](CONTRIBUTING.md) for checks and release instructions, [Security](SECURITY.md) for private reporting, and the [Changelog](CHANGELOG.md) for release notes.

## License

[MIT](LICENSE). Use it in personal and commercial projects.

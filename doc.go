// Package gateway provides a Go client for the Telegram Gateway API.
//
// Create a client with New and reuse it across goroutines. Each API call accepts
// a context. Requests use JSON over HTTPS and bearer authentication. The client
// does not retry requests: sending messages and checking send ability can incur
// charges. Configure retries only after accounting for Telegram's billing rules.
//
// DecodeReport authenticates delivery callbacks before decoding their JSON.
// Applications must still process callbacks idempotently and check the actual
// verification status before treating a phone number as verified.
//
// This is an independent library, unaffiliated with Telegram.
// See https://core.telegram.org/gateway/api for the upstream API contract.
package gateway

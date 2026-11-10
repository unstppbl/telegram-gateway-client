package gateway

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const maxReportBodyBytes = 1 << 20

var (
	// ErrInvalidSignature indicates that a delivery report could not be authenticated.
	ErrInvalidSignature = errors.New("gateway: invalid report signature")
	// ErrExpiredReport indicates that a signed report is too old or too far in the future.
	ErrExpiredReport = errors.New("gateway: report timestamp is outside the allowed window")
)

// DecodeReport authenticates and decodes a delivery report from Telegram.
// It rejects request bodies larger than 1 MiB and verifies their exact bytes before
// decoding JSON. maxAge must be positive; reports older than maxAge or more than
// 30 seconds in the future are rejected. Keep the server clock synchronized.
//
// A valid signature does not make a report unique. Applications should process
// repeated reports idempotently, since Telegram can retry delivery.
func (c *Client) DecodeReport(r *http.Request, maxAge time.Duration) (RequestStatus, error) {
	var status RequestStatus
	if c == nil || strings.TrimSpace(c.token) == "" {
		return status, errors.New("gateway: client is not initialized")
	}
	if maxAge <= 0 {
		return status, errors.New("gateway: report maximum age must be positive")
	}
	if r == nil || r.Method != http.MethodPost || r.Body == nil {
		return status, errors.New("gateway: delivery report must be a POST request with a body")
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, maxReportBodyBytes+1))
	if err != nil {
		return status, fmt.Errorf("gateway: read delivery report: %w", err)
	}
	if len(body) > maxReportBodyBytes {
		return status, errors.New("gateway: delivery report exceeds 1 MiB")
	}
	if err := verifyReport(c.token, body, r.Header.Get("X-Request-Timestamp"), r.Header.Get("X-Request-Signature"), time.Now(), maxAge); err != nil {
		return status, err
	}
	if err := json.Unmarshal(body, &status); err != nil {
		// JSON errors can contain values from the report, such as an overflowing
		// number. Keep codes, phone numbers and payloads out of error messages.
		return RequestStatus{}, errors.New("gateway: invalid delivery report JSON")
	}
	if strings.TrimSpace(status.RequestID) == "" {
		return RequestStatus{}, errors.New("gateway: delivery report is missing request_id")
	}
	return status, nil
}

func verifyReport(token string, body []byte, timestamp, signature string, now time.Time, maxAge time.Duration) error {
	provided, err := hex.DecodeString(signature)
	if err != nil || len(provided) != sha256.Size {
		return ErrInvalidSignature
	}
	key := sha256.Sum256([]byte(token))
	mac := hmac.New(sha256.New, key[:])
	_, _ = io.WriteString(mac, timestamp+"\n")
	_, _ = mac.Write(body)
	if !hmac.Equal(provided, mac.Sum(nil)) {
		return ErrInvalidSignature
	}
	if timestamp == "" || strings.IndexFunc(timestamp, func(r rune) bool { return r < '0' || r > '9' }) >= 0 {
		return fmt.Errorf("%w: malformed timestamp", ErrInvalidSignature)
	}
	seconds, err := strconv.ParseInt(timestamp, 10, 64)
	if err != nil {
		return fmt.Errorf("%w: malformed timestamp", ErrInvalidSignature)
	}
	age := now.Sub(time.Unix(seconds, 0))
	if age > maxAge || age < -30*time.Second {
		return ErrExpiredReport
	}
	return nil
}

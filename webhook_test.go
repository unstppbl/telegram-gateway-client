package gateway

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"
)

const reportTestToken = "report-test-token"

func signedReport(body, timestamp string) *http.Request {
	r := httptest.NewRequest(http.MethodPost, "/telegram/report", strings.NewReader(body))
	r.Header.Set("X-Request-Timestamp", timestamp)
	r.Header.Set("X-Request-Signature", reportSignature(reportTestToken, body, timestamp))
	return r
}

// This deliberately computes the signature separately from production code.
func reportSignature(token, body, timestamp string) string {
	secret := sha256.Sum256([]byte(token))
	h := hmac.New(sha256.New, secret[:])
	_, _ = h.Write([]byte(timestamp))
	_, _ = h.Write([]byte{10})
	_, _ = h.Write([]byte(body))
	return hex.EncodeToString(h.Sum(nil))
}

func TestDecodeReport(t *testing.T) {
	body := ` {"request_id":"request-1","phone_number":"+12025550123","request_cost":0.01,"is_refunded":true,"remaining_balance":12.5,"delivery_status":{"status":"delivered","updated_at":1794300000},"verification_status":{"status":"code_valid","updated_at":1794300001,"code_entered":"012345"},"payload":"order-123","future_field":true} `
	r := signedReport(body, strconv.FormatInt(time.Now().Unix(), 10))
	c := &Client{token: reportTestToken}
	got, err := c.DecodeReport(r, 5*time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if got.RequestID != "request-1" || got.PhoneNumber != "+12025550123" || got.RequestCost != 0.01 || got.Payload != "order-123" {
		t.Fatalf("report fields lost: %#v", got)
	}
	if got.DeliveryStatus == nil || got.DeliveryStatus.Status != "delivered" || got.VerificationStatus == nil || got.VerificationStatus.CodeEntered != "012345" {
		t.Fatalf("nested report fields lost: %#v", got)
	}
}

func TestDecodeReportRejectsUnauthenticatedData(t *testing.T) {
	const body = `{"request_id":"request-1"}`
	timestamp := strconv.FormatInt(time.Now().Unix(), 10)
	tests := []struct {
		name   string
		mutate func(*http.Request)
	}{
		{"missing signature", func(r *http.Request) { r.Header.Del("X-Request-Signature") }},
		{"missing timestamp", func(r *http.Request) { r.Header.Del("X-Request-Timestamp") }},
		{"nonhex signature", func(r *http.Request) { r.Header.Set("X-Request-Signature", strings.Repeat("z", 64)) }},
		{"short signature", func(r *http.Request) { r.Header.Set("X-Request-Signature", "00") }},
		{"wrong token", func(r *http.Request) {
			r.Header.Set("X-Request-Signature", reportSignature("other-token", body, timestamp))
		}},
		{"tampered body", func(r *http.Request) { r.Body = io.NopCloser(strings.NewReader(`{"request_id":"request-2"}`)) }},
		{"reserialized body", func(r *http.Request) { r.Body = io.NopCloser(strings.NewReader(body + "\n")) }},
		{"tampered timestamp", func(r *http.Request) { r.Header.Set("X-Request-Timestamp", "1794300000") }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := signedReport(body, timestamp)
			tt.mutate(r)
			_, err := (&Client{token: reportTestToken}).DecodeReport(r, time.Minute)
			if !errors.Is(err, ErrInvalidSignature) {
				t.Fatalf("got %v, want ErrInvalidSignature", err)
			}
		})
	}
	// Authentication happens before JSON parsing.
	r := signedReport("not json", timestamp)
	r.Header.Set("X-Request-Signature", strings.Repeat("0", 64))
	_, err := (&Client{token: reportTestToken}).DecodeReport(r, time.Minute)
	if !errors.Is(err, ErrInvalidSignature) {
		t.Fatalf("unauthenticated JSON was parsed: %v", err)
	}
}

func TestVerifyReportFreshness(t *testing.T) {
	now := time.Unix(1794300000, 0)
	const body = `{"request_id":"request-1"}`
	tests := []struct {
		name      string
		timestamp string
		want      error
	}{
		{"current", "1794300000", nil},
		{"old at limit", "1794299700", nil},
		{"expired", "1794299699", ErrExpiredReport},
		{"future at allowance", "1794300030", nil},
		{"future", "1794300031", ErrExpiredReport},
		{"missing", "", ErrInvalidSignature},
		{"not decimal", "yesterday", ErrInvalidSignature},
		{"negative", "-1", ErrInvalidSignature},
		{"explicit sign", "+1794300000", ErrInvalidSignature},
		{"overflow", "9999999999999999999999", ErrInvalidSignature},
		{"far future", "9223372036854775807", ErrExpiredReport},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := verifyReport(reportTestToken, []byte(body), tt.timestamp, reportSignature(reportTestToken, body, tt.timestamp), now, 5*time.Minute)
			if !errors.Is(err, tt.want) {
				t.Fatalf("got %v, want %v", err, tt.want)
			}
		})
	}
}

func TestVerifyReportSignatureVector(t *testing.T) {
	// Independently generated with Python's hashlib.sha256 and hmac.new.
	body := []byte("{\"request_id\":\"report-vector\",\"payload\":\"space matters\"}\n")
	err := verifyReport(reportTestToken, body, "1794300000", "c9ac5271934e287401781369fa1ae988cd2f5497829e7fe1b6a63d6afe7a3f95", time.Unix(1794300000, 0), time.Minute)
	if err != nil {
		t.Fatal(err)
	}
}

func TestDecodeReportRejectsMalformedReport(t *testing.T) {
	c := &Client{token: reportTestToken}
	timestamp := strconv.FormatInt(time.Now().Unix(), 10)
	for _, body := range []string{"", "null", "[]", "{}", `{"request_id":" "}`, `{"request_id":"request-1"} {}`, `{"request_id":"request-1"} junk`, `{"request_id":123}`, `{"request_id":"request-1"`} {
		t.Run(body, func(t *testing.T) {
			if _, err := c.DecodeReport(signedReport(body, timestamp), time.Minute); err == nil {
				t.Fatal("accepted malformed report")
			}
		})
	}
}

func TestDecodeReportDoesNotExposeMalformedValues(t *testing.T) {
	const sensitive = "123456789012345678901234567890"
	body := `{"request_id":"request-1","verification_status":{"status":"code_valid","updated_at":` + sensitive + `}}`
	r := signedReport(body, strconv.FormatInt(time.Now().Unix(), 10))
	status, err := (&Client{token: reportTestToken}).DecodeReport(r, time.Minute)
	if err == nil {
		t.Fatal("accepted overflowing timestamp")
	}
	if strings.Contains(err.Error(), sensitive) {
		t.Fatalf("error exposes a received value: %v", err)
	}
	if status != (RequestStatus{}) {
		t.Fatalf("returned a partial report on error: %#v", status)
	}
}

func TestDecodeReportRejectsOversizeBody(t *testing.T) {
	body := `{"request_id":"request-1","payload":"` + strings.Repeat("x", maxReportBodyBytes) + `"}`
	r := signedReport(body, strconv.FormatInt(time.Now().Unix(), 10))
	if _, err := (&Client{token: reportTestToken}).DecodeReport(r, time.Minute); err == nil || !strings.Contains(err.Error(), "1 MiB") {
		t.Fatalf("oversized report: %v", err)
	}
}

type failingReportBody struct{ err error }

func (b failingReportBody) Read([]byte) (int, error) { return 0, b.err }
func (b failingReportBody) Close() error             { return nil }

func TestDecodeReportInvalidInputs(t *testing.T) {
	c := &Client{token: reportTestToken}
	r := signedReport(`{"request_id":"request-1"}`, strconv.FormatInt(time.Now().Unix(), 10))
	if _, err := c.DecodeReport(nil, time.Minute); err == nil {
		t.Fatal("accepted nil request")
	}
	if _, err := c.DecodeReport(r, 0); err == nil {
		t.Fatal("accepted zero maximum age")
	}
	if _, err := c.DecodeReport(r, -time.Minute); err == nil {
		t.Fatal("accepted negative maximum age")
	}
	if _, err := (*Client)(nil).DecodeReport(r, time.Minute); err == nil {
		t.Fatal("accepted nil client")
	}
	if _, err := (&Client{}).DecodeReport(r, time.Minute); err == nil {
		t.Fatal("accepted uninitialized client")
	}
	r.Method = http.MethodGet
	if _, err := c.DecodeReport(r, time.Minute); err == nil {
		t.Fatal("accepted GET request")
	}
	r.Method = http.MethodPost
	r.Body = nil
	if _, err := c.DecodeReport(r, time.Minute); err == nil {
		t.Fatal("accepted nil body")
	}
	sentinel := errors.New("broken stream")
	r.Body = failingReportBody{sentinel}
	if _, err := c.DecodeReport(r, time.Minute); !errors.Is(err, sentinel) {
		t.Fatalf("body read cause lost: %v", err)
	}
}

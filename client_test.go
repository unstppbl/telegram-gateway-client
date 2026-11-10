package gateway_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	gateway "github.com/unstppbl/telegram-gateway-client"
)

const testToken = "test-only-token"

func testClient(t *testing.T, handler http.HandlerFunc) *gateway.Client {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	client, err := gateway.New(testToken, gateway.WithBaseURL(server.URL), gateway.WithHTTPClient(server.Client()))
	if err != nil {
		t.Fatal(err)
	}
	return client
}

func writeJSON(t *testing.T, w http.ResponseWriter, body any) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(body); err != nil {
		t.Error(err)
	}
}

func TestMethodsHTTPContract(t *testing.T) {
	status := `{"request_id":"request-1","phone_number":"+12025550123","request_cost":0.01,"is_refunded":false,"remaining_balance":0,"delivery_status":{"status":"delivered","updated_at":1700000000},"verification_status":{"status":"code_invalid","updated_at":1700000001,"code_entered":"012345"},"payload":"signup-123"}`
	var paths []string
	client := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %s; want POST", r.Method)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer "+testToken {
			t.Errorf("authorization = %q", got)
		}
		if got := r.Header.Get("Content-Type"); got != "application/json" {
			t.Errorf("content type = %q", got)
		}
		if r.URL.RawQuery != "" {
			t.Errorf("unexpected query parameters: %q", r.URL.RawQuery)
		}
		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Error(err)
		}
		paths = append(paths, r.URL.Path)
		switch r.URL.Path {
		case "/sendVerificationMessage":
			want := map[string]any{
				"phone_number": "+12025550123", "request_id": "request-1", "sender_username": "verified_channel",
				"code": "012345", "callback_url": "https://example.com/reports",
				"payload": "signup-123", "ttl": float64(300),
			}
			if !reflect.DeepEqual(payload, want) {
				t.Errorf("send payload = %#v; want %#v", payload, want)
			}
		case "/checkSendAbility":
			if len(payload) != 1 || payload["phone_number"] != "+12025550123" {
				t.Errorf("check send ability payload = %#v", payload)
			}
		case "/checkVerificationStatus":
			if len(payload) != 2 || payload["request_id"] != "request-1" || payload["code"] != "012345" {
				t.Errorf("check status payload = %#v", payload)
			}
		case "/revokeVerificationMessage":
			if len(payload) != 1 || payload["request_id"] != "request-1" {
				t.Errorf("revoke payload = %#v", payload)
			}
			_, _ = io.WriteString(w, `{"ok":true,"result":true}`)
			return
		default:
			t.Errorf("unexpected API path: %s", r.URL.Path)
		}
		_, _ = io.WriteString(w, `{"ok":true,"result":`+status+`}`)
	})

	sent, err := client.SendVerificationMessage(context.Background(), gateway.SendVerificationMessageParams{
		PhoneNumber: "+12025550123", RequestID: "request-1", SenderUsername: "verified_channel",
		Code: "012345", CodeLength: 6, CallbackURL: "https://example.com/reports", Payload: "signup-123", TTL: 300,
	})
	if err != nil {
		t.Fatal(err)
	}
	assertStatus(t, sent)
	ability, err := client.CheckSendAbility(context.Background(), "+12025550123")
	if err != nil {
		t.Fatal(err)
	}
	assertStatus(t, ability)
	verified, err := client.CheckVerificationStatus(context.Background(), gateway.CheckVerificationStatusParams{RequestID: "request-1", Code: "012345"})
	if err != nil {
		t.Fatal(err)
	}
	assertStatus(t, verified)
	revoked, err := client.RevokeVerificationMessage(context.Background(), "request-1")
	if err != nil || !revoked {
		t.Fatalf("revoke = %v, %v", revoked, err)
	}
	if got := strings.Join(paths, ","); got != "/sendVerificationMessage,/checkSendAbility,/checkVerificationStatus,/revokeVerificationMessage" {
		t.Errorf("paths = %s", got)
	}
}

func assertStatus(t *testing.T, status gateway.RequestStatus) {
	t.Helper()
	if status.RequestID != "request-1" || status.PhoneNumber != "+12025550123" || status.RequestCost != 0.01 || status.Payload != "signup-123" {
		t.Errorf("request fields lost: %+v", status)
	}
	if status.IsRefunded == nil || *status.IsRefunded || status.RemainingBalance == nil || *status.RemainingBalance != 0 {
		t.Errorf("optional false/zero values lost: %+v", status)
	}
	if status.DeliveryStatus == nil || status.DeliveryStatus.Status != gateway.DeliveryDelivered || status.DeliveryStatus.UpdatedAt != 1700000000 {
		t.Errorf("delivery status lost: %+v", status.DeliveryStatus)
	}
	if status.VerificationStatus == nil || status.VerificationStatus.Status != gateway.VerificationCodeInvalid || status.VerificationStatus.UpdatedAt != 1700000001 || status.VerificationStatus.CodeEntered != "012345" {
		t.Errorf("verification status lost: %+v", status.VerificationStatus)
	}
}

func TestOptionalFieldsAreOmitted(t *testing.T) {
	client := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Error(err)
		}
		switch r.URL.Path {
		case "/sendVerificationMessage":
			if len(payload) != 1 || payload["phone_number"] != "+12025550123" {
				t.Errorf("optional send fields were serialized: %#v", payload)
			}
		case "/checkVerificationStatus":
			if len(payload) != 1 || payload["request_id"] != "request-1" {
				t.Errorf("optional code was serialized: %#v", payload)
			}
		}
		writeJSON(t, w, map[string]any{"ok": true, "result": map[string]any{"request_id": "request-1", "phone_number": "+12025550123"}})
	})
	status, err := client.SendVerificationMessage(context.Background(), gateway.SendVerificationMessageParams{PhoneNumber: "+12025550123"})
	if err != nil {
		t.Fatal(err)
	}
	if status.IsRefunded != nil || status.RemainingBalance != nil || status.DeliveryStatus != nil || status.VerificationStatus != nil {
		t.Fatalf("absent optional fields should remain nil: %+v", status)
	}
	if _, err := client.CheckVerificationStatus(context.Background(), gateway.CheckVerificationStatusParams{RequestID: "request-1"}); err != nil {
		t.Fatal(err)
	}
}

func TestRevokeFalseIsNotAnError(t *testing.T) {
	client := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `{"ok":true,"result":false}`)
	})
	result, err := client.RevokeVerificationMessage(context.Background(), "request-1")
	if err != nil || result {
		t.Fatalf("revoke = %v, %v; want false, nil", result, err)
	}
}

func TestAPIErrors(t *testing.T) {
	for _, code := range []int{http.StatusOK, http.StatusBadRequest, http.StatusUnauthorized, http.StatusTooManyRequests} {
		t.Run(http.StatusText(code), func(t *testing.T) {
			var calls atomic.Int32
			client := testClient(t, func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				w.WriteHeader(code)
				_, _ = io.WriteString(w, `{"ok":false,"error":"ACCESS_TOKEN_INVALID"}`)
			})
			_, err := client.CheckSendAbility(context.Background(), "+12025550123")
			var apiErr *gateway.APIError
			if !errors.As(err, &apiErr) {
				t.Fatalf("error = %T %v; want *APIError", err, err)
			}
			if apiErr.Method != "checkSendAbility" || apiErr.StatusCode != code || apiErr.Code != "ACCESS_TOKEN_INVALID" {
				t.Errorf("API error fields = %+v", apiErr)
			}
			if calls.Load() != 1 {
				t.Fatalf("request attempted %d times; charged requests must not be retried", calls.Load())
			}
		})
	}
}

func TestHTTPErrors(t *testing.T) {
	for _, body := range []string{"upstream unavailable", `{"ok":true,"result":{"request_id":"request-1"}}`} {
		t.Run(body, func(t *testing.T) {
			var calls atomic.Int32
			client := testClient(t, func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				w.WriteHeader(http.StatusServiceUnavailable)
				_, _ = io.WriteString(w, body)
			})
			_, err := client.CheckSendAbility(context.Background(), "+12025550123")
			var httpErr *gateway.HTTPError
			if !errors.As(err, &httpErr) || httpErr.Method != "checkSendAbility" || httpErr.StatusCode != http.StatusServiceUnavailable {
				t.Fatalf("error = %T %v; want HTTPError with method and status", err, err)
			}
			if calls.Load() != 1 {
				t.Fatalf("request attempted %d times", calls.Load())
			}
		})
	}
}

func TestMalformedResponses(t *testing.T) {
	cases := map[string]string{
		"empty":             "",
		"invalid JSON":      "<html>error</html>",
		"null envelope":     "null",
		"missing ok":        `{"result":{"request_id":"request-1"}}`,
		"null ok":           `{"ok":null,"result":{"request_id":"request-1"}}`,
		"missing result":    `{"ok":true}`,
		"null result":       `{"ok":true,"result":null}`,
		"wrong result type": `{"ok":true,"result":false}`,
		"missing API error": `{"ok":false}`,
		"trailing JSON":     `{"ok":true,"result":{"request_id":"request-1"}} {}`,
		"oversized":         `{"ok":true,"result":{"request_id":"request-1","payload":"` + strings.Repeat("x", 1<<20) + `"}}`,
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			client := testClient(t, func(w http.ResponseWriter, r *http.Request) {
				_, _ = io.WriteString(w, body)
			})
			if _, err := client.CheckSendAbility(context.Background(), "+12025550123"); err == nil {
				t.Fatal("malformed response was accepted")
			}
		})
	}
}

func TestMalformedStatusResultsReturnZero(t *testing.T) {
	methods := []struct {
		name string
		call func(*gateway.Client) (gateway.RequestStatus, error)
	}{
		{"sendVerificationMessage", func(client *gateway.Client) (gateway.RequestStatus, error) {
			return client.SendVerificationMessage(context.Background(), gateway.SendVerificationMessageParams{PhoneNumber: "+12025550123"})
		}},
		{"checkSendAbility", func(client *gateway.Client) (gateway.RequestStatus, error) {
			return client.CheckSendAbility(context.Background(), "+12025550123")
		}},
		{"checkVerificationStatus", func(client *gateway.Client) (gateway.RequestStatus, error) {
			return client.CheckVerificationStatus(context.Background(), gateway.CheckVerificationStatusParams{RequestID: "request-1", Code: "012345"})
		}},
	}
	cases := []struct {
		name   string
		result string
		secret string
	}{
		{"sensitive string", `{"request_id":"request-1","verification_status":{"status":"code_valid"},"request_cost":"customer-sensitive-value-XY99"}`, "customer-sensitive-value-XY99"},
		{"sensitive number", `{"request_id":"request-1","verification_status":{"status":"code_valid"},"payload":991234567890}`, "991234567890"},
	}
	for _, method := range methods {
		for _, tc := range cases {
			t.Run(method.name+"/"+tc.name, func(t *testing.T) {
				client := testClient(t, func(w http.ResponseWriter, r *http.Request) {
					_, _ = io.WriteString(w, `{"ok":true,"result":`+tc.result+`}`)
				})
				status, err := method.call(client)
				if err == nil {
					t.Fatal("partially malformed result was accepted")
				}
				if !reflect.DeepEqual(status, gateway.RequestStatus{}) {
					t.Errorf("malformed result leaked partial status alongside error: %+v", status)
				}
				if strings.Contains(err.Error(), tc.secret) || strings.Contains(err.Error(), "code_valid") {
					t.Errorf("error echoed sensitive response values: %s", err)
				}
			})
		}
	}
}

func TestContextCancellation(t *testing.T) {
	client := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		select {
		case <-r.Context().Done():
		case <-time.After(time.Second):
			t.Error("request context was not canceled")
		}
	})
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	_, err := client.CheckSendAbility(ctx, "+12025550123")
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("error = %v; want context.DeadlineExceeded", err)
	}
	ctx, cancel = context.WithCancel(context.Background())
	cancel()
	_, err = client.CheckSendAbility(ctx, "+12025550123")
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v; want context.Canceled", err)
	}
}

func TestRedirectsAreBlockedWithoutChangingHTTPClient(t *testing.T) {
	var redirected atomic.Int32
	destination := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		redirected.Add(1)
		writeJSON(t, w, map[string]any{"ok": true, "result": map[string]any{"request_id": "unexpected"}})
	}))
	defer destination.Close()
	source := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, destination.URL, http.StatusTemporaryRedirect)
	}))
	defer source.Close()
	var callbackCalls atomic.Int32
	httpClient := source.Client()
	httpClient.CheckRedirect = func(r *http.Request, via []*http.Request) error {
		callbackCalls.Add(1)
		return nil
	}
	client, err := gateway.New(testToken, gateway.WithBaseURL(source.URL), gateway.WithHTTPClient(httpClient))
	if err != nil {
		t.Fatal(err)
	}
	_, err = client.CheckSendAbility(context.Background(), "+12025550123")
	var httpErr *gateway.HTTPError
	if !errors.As(err, &httpErr) || httpErr.StatusCode != http.StatusTemporaryRedirect {
		t.Fatalf("redirect error = %T %v", err, err)
	}
	if redirected.Load() != 0 || callbackCalls.Load() != 0 {
		t.Fatalf("gateway followed redirect: requests=%d, custom callbacks=%d", redirected.Load(), callbackCalls.Load())
	}
	response, err := httpClient.Get(source.URL)
	if err != nil {
		t.Fatal(err)
	}
	_ = response.Body.Close()
	if redirected.Load() != 1 || callbackCalls.Load() != 1 {
		t.Fatal("constructor changed the caller's http.Client redirect policy")
	}
}

func TestConstructorValidation(t *testing.T) {
	for _, token := range []string{"", " ", "token\r\nInjected: header"} {
		if _, err := gateway.New(token); err == nil {
			t.Errorf("accepted invalid access token %q", token)
		}
	}
	for _, endpoint := range []string{"not a URL", "ftp://example.com", "https://user:pass@example.com", "https://example.com?token=secret", "https://example.com/#fragment", "http://example.com"} {
		if _, err := gateway.New(testToken, gateway.WithBaseURL(endpoint)); err == nil {
			t.Errorf("accepted unsafe endpoint %q", endpoint)
		}
	}
}

func TestSendValidationHappensBeforeNetwork(t *testing.T) {
	var calls atomic.Int32
	client := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		writeJSON(t, w, map[string]any{"ok": true, "result": map[string]any{"request_id": "request-1"}})
	})
	valid := gateway.SendVerificationMessageParams{PhoneNumber: "+12025550123"}
	cases := []struct {
		name string
		edit func(*gateway.SendVerificationMessageParams)
	}{
		{"missing phone", func(p *gateway.SendVerificationMessageParams) { p.PhoneNumber = "" }},
		{"no plus", func(p *gateway.SendVerificationMessageParams) { p.PhoneNumber = "12025550123" }},
		{"non digits", func(p *gateway.SendVerificationMessageParams) { p.PhoneNumber = "+1202 5550123" }},
		{"leading zero", func(p *gateway.SendVerificationMessageParams) { p.PhoneNumber = "+02025550123" }},
		{"too many digits", func(p *gateway.SendVerificationMessageParams) { p.PhoneNumber = "+1234567890123456" }},
		{"code too short", func(p *gateway.SendVerificationMessageParams) { p.Code = "123" }},
		{"code too long", func(p *gateway.SendVerificationMessageParams) { p.Code = "123456789" }},
		{"code not numeric", func(p *gateway.SendVerificationMessageParams) { p.Code = "12x4" }},
		{"code length", func(p *gateway.SendVerificationMessageParams) { p.CodeLength = 9 }},
		{"ttl below range", func(p *gateway.SendVerificationMessageParams) { p.TTL = 29 }},
		{"ttl above range", func(p *gateway.SendVerificationMessageParams) { p.TTL = 3601 }},
		{"ttl negative", func(p *gateway.SendVerificationMessageParams) { p.TTL = -1 }},
		{"callback not https", func(p *gateway.SendVerificationMessageParams) { p.CallbackURL = "http://example.com/report" }},
		{"callback too long", func(p *gateway.SendVerificationMessageParams) {
			p.CallbackURL = "https://example.com/" + strings.Repeat("a", 256)
		}},
		{"payload byte limit", func(p *gateway.SendVerificationMessageParams) { p.Payload = strings.Repeat("é", 65) }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			params := valid
			tc.edit(&params)
			if _, err := client.SendVerificationMessage(context.Background(), params); err == nil {
				t.Fatal("invalid send parameters were accepted")
			}
		})
	}
	for _, phone := range []string{"", "12025550123", "+02025550123", "+1x"} {
		if _, err := client.CheckSendAbility(context.Background(), phone); err == nil {
			t.Errorf("invalid checkSendAbility phone %q was accepted", phone)
		}
	}
	if _, err := client.CheckVerificationStatus(context.Background(), gateway.CheckVerificationStatusParams{}); err == nil {
		t.Error("missing checkVerificationStatus request ID was accepted")
	}
	if _, err := client.RevokeVerificationMessage(context.Background(), ""); err == nil {
		t.Error("missing revokeVerificationMessage request ID was accepted")
	}
	if calls.Load() != 0 {
		t.Fatalf("invalid parameters caused %d network calls", calls.Load())
	}
}

func TestSendBoundaryValuesAndExplicitCodePrecedence(t *testing.T) {
	client := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		writeJSON(t, w, map[string]any{"ok": true, "result": map[string]any{"request_id": "request-1"}})
	})
	for _, params := range []gateway.SendVerificationMessageParams{
		{PhoneNumber: "+12", CodeLength: 4, TTL: 30},
		{PhoneNumber: "+123456789012345", CodeLength: 8, TTL: 3600, Payload: strings.Repeat("é", 64)},
		{PhoneNumber: "+12025550123", Code: "0123", CodeLength: 99},
		{PhoneNumber: "+12025550123", Code: "01234567"},
	} {
		if _, err := client.SendVerificationMessage(context.Background(), params); err != nil {
			t.Errorf("valid send params %+v rejected: %v", params, err)
		}
	}
}

func TestClientCanBeSharedByGoroutines(t *testing.T) {
	client := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		writeJSON(t, w, map[string]any{"ok": true, "result": map[string]any{"request_id": "request-1"}})
	})
	for range 8 {
		t.Run("concurrent", func(t *testing.T) {
			t.Parallel()
			if _, err := client.CheckSendAbility(context.Background(), "+12025550123"); err != nil {
				t.Error(err)
			}
		})
	}
}

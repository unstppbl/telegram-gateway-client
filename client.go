package gateway

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
	"unicode"
)

const (
	// DefaultBaseURL is Telegram's production Gateway endpoint.
	DefaultBaseURL  = "https://gatewayapi.telegram.org"
	maxResponseBody = 1 << 20
)

// Client sends requests to Telegram Gateway. Construct it with New; the zero
// value is not usable. A Client can be shared by concurrent goroutines.
type Client struct {
	token   string
	client  *http.Client
	baseURL string
}

// Option configures a Client at construction time.
type Option func(*Client) error

// WithHTTPClient supplies an HTTP client, including its transport and timeout.
// A shallow copy is used, with redirects disabled to protect bearer credentials
// and avoid replaying paid requests. The supplied client is not modified.
// Its Transport must support concurrent requests if the gateway client is shared.
func WithHTTPClient(client *http.Client) Option {
	return func(c *Client) error {
		if client == nil {
			return errors.New("telegram gateway: HTTP client must not be nil")
		}
		copy := *client
		c.client = &copy
		return nil
	}
}

// WithBaseURL overrides Telegram's endpoint, for example for a local test server.
// HTTPS is required except for HTTP loopback addresses. Credentials, queries and
// fragments are rejected. Only use an endpoint you trust with your access token.
func WithBaseURL(endpoint string) Option {
	return func(c *Client) error {
		u, err := url.Parse(endpoint)
		if err != nil || u.Hostname() == "" || u.User != nil || u.RawQuery != "" || u.ForceQuery || u.Fragment != "" {
			return errors.New("telegram gateway: invalid base URL")
		}
		loopback := strings.EqualFold(u.Hostname(), "localhost") || net.ParseIP(u.Hostname()).IsLoopback()
		if u.Scheme != "https" && !(u.Scheme == "http" && loopback) {
			return errors.New("telegram gateway: base URL must use HTTPS (HTTP is allowed on loopback)")
		}
		c.baseURL = strings.TrimRight(u.String(), "/")
		return nil
	}
}

// New creates a client with a 10-second timeout and no automatic retries.
// The token comes from Telegram Gateway account settings, not BotFather.
func New(token string, options ...Option) (*Client, error) {
	if token == "" || strings.ContainsFunc(token, func(r rune) bool { return unicode.IsSpace(r) || unicode.IsControl(r) }) {
		return nil, errors.New("telegram gateway: access token must be nonempty and contain no whitespace or control characters")
	}
	c := &Client{token: token, client: &http.Client{Timeout: 10 * time.Second}, baseURL: DefaultBaseURL}
	for _, option := range options {
		if option == nil {
			return nil, errors.New("telegram gateway: option must not be nil")
		}
		if err := option(c); err != nil {
			return nil, err
		}
	}
	c.client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	return c, nil
}

// SendVerificationMessage sends a code. This call can incur a charge. A RequestID
// returned by CheckSendAbility can be reused once to avoid a second charge.
func (c *Client) SendVerificationMessage(ctx context.Context, params SendVerificationMessageParams) (RequestStatus, error) {
	var result RequestStatus
	if err := validateSend(params); err != nil {
		return result, err
	}
	if params.Code != "" {
		params.CodeLength = 0
	}
	err := c.call(ctx, "sendVerificationMessage", params, &result)
	if err != nil {
		return RequestStatus{}, err
	}
	return result, nil
}

// CheckSendAbility checks whether a phone number can receive a message. A
// successful check can incur a charge even when no message is subsequently sent.
func (c *Client) CheckSendAbility(ctx context.Context, phoneNumber string) (RequestStatus, error) {
	var result RequestStatus
	if !validPhone(phoneNumber) {
		return result, errors.New("telegram gateway: phone number must use E.164 format")
	}
	err := c.call(ctx, "checkSendAbility", struct {
		PhoneNumber string `json:"phone_number"`
	}{phoneNumber}, &result)
	if err != nil {
		return RequestStatus{}, err
	}
	return result, nil
}

// CheckVerificationStatus fetches status and, when Code is set, submits the
// user's code for verification. An invalid code is a status, not an APIError.
func (c *Client) CheckVerificationStatus(ctx context.Context, params CheckVerificationStatusParams) (RequestStatus, error) {
	var result RequestStatus
	if strings.TrimSpace(params.RequestID) == "" {
		return result, errors.New("telegram gateway: request ID is required")
	}
	err := c.call(ctx, "checkVerificationStatus", params, &result)
	if err != nil {
		return RequestStatus{}, err
	}
	return result, nil
}

// RevokeVerificationMessage requests revocation. True acknowledges receipt;
// Telegram does not remove messages that have already been delivered or read.
func (c *Client) RevokeVerificationMessage(ctx context.Context, requestID string) (bool, error) {
	var result bool
	if strings.TrimSpace(requestID) == "" {
		return false, errors.New("telegram gateway: request ID is required")
	}
	err := c.call(ctx, "revokeVerificationMessage", struct {
		RequestID string `json:"request_id"`
	}{requestID}, &result)
	return result, err
}

type envelope struct {
	OK     *bool           `json:"ok"`
	Result json.RawMessage `json:"result"`
	Error  string          `json:"error"`
}

func (c *Client) call(ctx context.Context, method string, payload, result any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("telegram gateway: %s: encode request: %w", method, err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/"+method, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("telegram gateway: %s: create request: %w", method, err)
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "telegram-gateway-client-go")
	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("telegram gateway: %s: %w", method, err)
	}
	defer resp.Body.Close()
	raw, readErr := io.ReadAll(io.LimitReader(resp.Body, maxResponseBody+1))
	var response envelope
	validEnvelope := readErr == nil && len(raw) <= maxResponseBody && json.Unmarshal(raw, &response) == nil && response.OK != nil
	if validEnvelope && !*response.OK && response.Error != "" {
		return &APIError{Method: method, StatusCode: resp.StatusCode, Code: response.Error}
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return &HTTPError{Method: method, StatusCode: resp.StatusCode}
	}
	if readErr != nil {
		return fmt.Errorf("telegram gateway: %s: read response: %w", method, readErr)
	}
	if len(raw) > maxResponseBody {
		return fmt.Errorf("telegram gateway: %s: response exceeds 1 MiB", method)
	}
	if !validEnvelope || !*response.OK || len(response.Result) == 0 || bytes.Equal(bytes.TrimSpace(response.Result), []byte("null")) {
		return fmt.Errorf("telegram gateway: %s: invalid response envelope", method)
	}
	if err := json.Unmarshal(response.Result, result); err != nil {
		// Do not echo JSON values: results can contain codes and phone numbers.
		return fmt.Errorf("telegram gateway: %s: invalid result", method)
	}
	if status, ok := result.(*RequestStatus); ok && status.RequestID == "" {
		return fmt.Errorf("telegram gateway: %s: missing request ID in result", method)
	}
	return nil
}

func validateSend(p SendVerificationMessageParams) error {
	if !validPhone(p.PhoneNumber) {
		return errors.New("telegram gateway: phone number must use E.164 format")
	}
	if p.Code != "" {
		if len(p.Code) < 4 || len(p.Code) > 8 || !digits(p.Code) {
			return errors.New("telegram gateway: code must contain 4–8 ASCII digits")
		}
	} else if p.CodeLength != 0 && (p.CodeLength < 4 || p.CodeLength > 8) {
		return errors.New("telegram gateway: code length must be 4–8 or zero to omit")
	}
	if p.TTL != 0 && (p.TTL < 30 || p.TTL > 3600) {
		return errors.New("telegram gateway: TTL must be 30–3600 seconds or zero to omit")
	}
	if len(p.Payload) > 128 {
		return errors.New("telegram gateway: payload exceeds 128 bytes")
	}
	if p.CallbackURL != "" {
		u, err := url.Parse(p.CallbackURL)
		if err != nil || len(p.CallbackURL) > 256 || u.Scheme != "https" || u.Hostname() == "" || u.User != nil || u.Fragment != "" {
			return errors.New("telegram gateway: callback URL must be HTTPS and at most 256 bytes, without credentials or fragment")
		}
	}
	return nil
}

func validPhone(phone string) bool {
	return len(phone) >= 3 && len(phone) <= 16 && phone[0] == '+' && phone[1] >= '1' && phone[1] <= '9' && digits(phone[1:])
}

func digits(s string) bool {
	for i := range len(s) {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}

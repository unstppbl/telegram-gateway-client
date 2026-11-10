package gateway

// SendVerificationMessageParams configures a verification message.
type SendVerificationMessageParams struct {
	// PhoneNumber is the recipient's phone number in E.164 format.
	PhoneNumber string `json:"phone_number"`
	// RequestID reuses a successful CheckSendAbility request. It can be used once.
	RequestID string `json:"request_id,omitempty"`
	// SenderUsername identifies a verified channel owned by the token's account.
	SenderUsername string `json:"sender_username,omitempty"`
	// Code sets a numeric code of 4–8 digits, preserving leading zeroes.
	// When set, CodeLength is ignored and omitted from the outgoing request.
	Code string `json:"code,omitempty"`
	// CodeLength asks Telegram to generate a code of 4–8 digits.
	// Set Code or CodeLength to select how the verification code is created.
	CodeLength int `json:"code_length,omitempty"`
	// CallbackURL is an HTTPS delivery report endpoint of at most 256 bytes.
	CallbackURL string `json:"callback_url,omitempty"`
	// Payload is application data of at most 128 bytes, invisible to the recipient.
	Payload string `json:"payload,omitempty"`
	// TTL is the message lifetime in seconds, from 30 to 3600; zero omits it.
	TTL int `json:"ttl,omitempty"`
}

// CheckVerificationStatusParams identifies a request and optionally checks a code.
type CheckVerificationStatusParams struct {
	RequestID string `json:"request_id"`
	// Code may be omitted to retrieve status without submitting a code.
	Code string `json:"code,omitempty"`
}

// RequestStatus is returned by send, send-ability, status and delivery callbacks.
// Pointer fields distinguish omitted optional values from false or zero.
// Monetary values are expressed in Telegram Gateway credits.
type RequestStatus struct {
	RequestID          string              `json:"request_id"`
	PhoneNumber        string              `json:"phone_number"`
	RequestCost        float64             `json:"request_cost"`
	IsRefunded         *bool               `json:"is_refunded,omitempty"`
	RemainingBalance   *float64            `json:"remaining_balance,omitempty"`
	DeliveryStatus     *DeliveryStatus     `json:"delivery_status,omitempty"`
	VerificationStatus *VerificationStatus `json:"verification_status,omitempty"`
	Payload            string              `json:"payload,omitempty"`
}

// DeliveryState describes message delivery. Unknown future values are preserved.
type DeliveryState string

const (
	DeliverySent      DeliveryState = "sent"
	DeliveryDelivered DeliveryState = "delivered"
	DeliveryRead      DeliveryState = "read"
	DeliveryExpired   DeliveryState = "expired"
	DeliveryRevoked   DeliveryState = "revoked"
)

// DeliveryStatus records a delivery state and its Unix timestamp in seconds.
type DeliveryStatus struct {
	Status    DeliveryState `json:"status"`
	UpdatedAt int64         `json:"updated_at"`
}

// VerificationState describes code verification. Unknown values are preserved.
type VerificationState string

const (
	VerificationCodeValid               VerificationState = "code_valid"
	VerificationCodeInvalid             VerificationState = "code_invalid"
	VerificationCodeMaxAttemptsExceeded VerificationState = "code_max_attempts_exceeded"
	VerificationExpired                 VerificationState = "expired"
)

// VerificationStatus records code verification and its Unix timestamp in seconds.
type VerificationStatus struct {
	Status      VerificationState `json:"status"`
	UpdatedAt   int64             `json:"updated_at"`
	CodeEntered string            `json:"code_entered,omitempty"`
}

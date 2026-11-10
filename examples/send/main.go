// Send a six-digit verification code using Telegram Gateway.
// Set TELEGRAM_GATEWAY_TOKEN and TELEGRAM_PHONE_NUMBER before running this example.
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
	token := os.Getenv("TELEGRAM_GATEWAY_TOKEN")
	phone := os.Getenv("TELEGRAM_PHONE_NUMBER")
	if token == "" || phone == "" {
		log.Fatal("set TELEGRAM_GATEWAY_TOKEN and TELEGRAM_PHONE_NUMBER (E.164 format) before running")
	}
	client, err := gateway.New(token)
	if err != nil {
		log.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	status, err := client.SendVerificationMessage(ctx, gateway.SendVerificationMessageParams{
		PhoneNumber: phone,
		CodeLength:  6,
		TTL:         300,
	})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("Verification request sent:", status.RequestID)
	fmt.Println("Use CheckVerificationStatus with this request ID and the code entered by the recipient.")
}

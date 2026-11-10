package gateway_test

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"time"

	gateway "github.com/unstppbl/telegram-gateway-client"
)

func ExampleClient_SendVerificationMessage() {
	client, err := gateway.New(os.Getenv("TELEGRAM_GATEWAY_TOKEN"))
	if err != nil {
		log.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	status, err := client.SendVerificationMessage(ctx, gateway.SendVerificationMessageParams{
		PhoneNumber: os.Getenv("TELEGRAM_PHONE_NUMBER"),
		CodeLength:  6,
		TTL:         300,
	})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(status.RequestID)
}

func ExampleClient_CheckVerificationStatus() {
	client, err := gateway.New(os.Getenv("TELEGRAM_GATEWAY_TOKEN"))
	if err != nil {
		log.Fatal(err)
	}
	status, err := client.CheckVerificationStatus(context.Background(), gateway.CheckVerificationStatusParams{
		RequestID: os.Getenv("TELEGRAM_REQUEST_ID"),
		Code:      os.Getenv("TELEGRAM_VERIFICATION_CODE"),
	})
	if err != nil {
		log.Fatal(err)
	}
	if status.VerificationStatus != nil && status.VerificationStatus.Status == gateway.VerificationCodeValid {
		fmt.Println("Phone number verified")
	}
}

func ExampleAPIError() {
	client, err := gateway.New(os.Getenv("TELEGRAM_GATEWAY_TOKEN"))
	if err != nil {
		log.Fatal(err)
	}
	_, err = client.CheckSendAbility(context.Background(), os.Getenv("TELEGRAM_PHONE_NUMBER"))
	var apiErr *gateway.APIError
	if errors.As(err, &apiErr) {
		fmt.Println("Telegram rejected the request:", apiErr.Code)
	} else if err != nil {
		log.Fatal(err)
	}
}

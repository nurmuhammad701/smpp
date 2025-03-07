// example/main.go

package main

import (
	"context"
	"log"
	"time"

	"github.com/nurmuhammad701/smpp"
	mongosh "github.com/nurmuhammad701/smpp/db"
)

func main() {
	db, err := mongosh.Connect(context.Background())
	if err != nil {
		log.Fatalf("Failed to connect to MongoDB: %v", err)
	}

	productsRepo := mongosh.NewProductsRepository(db)
	res, err := productsRepo.GetSMPPConfig(context.Background())
	if err != nil {
		log.Fatalf("Failed to fetch SMPP config: %v", err)
	}

	api, err := smpp.NewSMPPAPI(
		res.Host,       // Replace with your SMPP server address
		res.Port,       // Default SMPP port
		res.Username,   // System ID
		res.Password,   // Password
		res.SystemType, // System Type (often empty)
	)
	if err != nil {
		log.Fatalf("Failed to create SMPP API: %v", err)
	}

	// Configure reconnection policy and timeouts
	api.WithReconnectPolicy(
		5,              // Max reconnect attempts (0 for infinite)
		5*time.Second,  // Initial delay
		60*time.Second, // Max delay
	).WithTimeouts(
		10*time.Second, // Connect timeout
		30*time.Second, // Read timeout
		10*time.Second, // Write timeout
		60*time.Second, // Enquire link interval
	)

	// Connect to the SMPP server
	err = api.Connect(false) // Don't use TLS
	if err != nil {
		log.Fatalf("Failed to connect1: %v", err)
	}
	defer api.Disconnect()

	// Send a regular SMS
	messageID, err := api.SendSMS(&smpp.SMSMessage{
		SourceAddr:         "12345",
		DestAddr:           "9876543210",
		Message:            []byte("Hello, this is a test message!"),
		RegisteredDelivery: 1, // Request delivery receipt
	})
	if err != nil {
		log.Printf("Failed to send SMS: %v", err)
	} else {
		log.Printf("Message sent successfully! Message ID: %s", messageID)
	}

	// Send a Unicode SMS
	unicodeMessageID, err := api.SendUnicodeSMS(&smpp.SMSMessage{
		SourceAddr:         "12345",
		DestAddr:           "9876543210",
		Message:            []byte("こんにちは世界"), // "Hello world" in Japanese
		RegisteredDelivery: 1,
	})
	if err != nil {
		log.Printf("Failed to send Unicode SMS: %v", err)
	} else {
		log.Printf("Unicode message sent successfully! Message ID: %s", unicodeMessageID)
	}

	// Send a long SMS that will be automatically segmented
	longMessage := []byte("This is a very long message that will be automatically segmented " +
		"by the SMPP client. SMPP 3.4 has a limit of 140 bytes (or 160 7-bit characters) " +
		"per message, so longer messages need to be split into multiple segments. " +
		"This is handled automatically by the SendLongSMS method, which uses the " +
		"SAR (Segmentation and Reassembly) protocol to ensure the message is " +
		"displayed correctly on the recipient's device.")

	messageIDs, err := api.SendLongSMS(&smpp.SMSMessage{
		SourceAddr:         "12345",
		DestAddr:           "9876543210",
		Message:            longMessage,
		RegisteredDelivery: 1,
	})
	if err != nil {
		log.Printf("Failed to send long SMS: %v", err)
	} else {
		log.Printf("Long message sent successfully! Message IDs: %v", messageIDs)
	}

	// Send a binary SMS
	binaryData := []byte{0x05, 0x00, 0x03, 0x42, 0x03, 0x01}
	binaryMessageID, err := api.SendBinarySMS(&smpp.SMSMessage{
		SourceAddr:         "12345",
		DestAddr:           "9876543210",
		Message:            binaryData,
		RegisteredDelivery: 1,
	})
	if err != nil {
		log.Printf("Failed to send binary SMS: %v", err)
	} else {
		log.Printf("Binary message sent successfully! Message ID: %s", binaryMessageID)
	}

	// Example of using the lower-level client directly
	client := smpp.NewSMPPClient(
		res.Host,       // Replace with your SMPP server address
		res.Port,       // Default SMPP port
		res.Username,   // System ID
		res.Password,   // Password
		res.SystemType, // System Type (often empty)
	)

	err = client.Connect()
	if err != nil {
		log.Fatalf("Failed to connect2: %v", err)
	}

	_, err = client.BindTransmitter()
	if err != nil {
		log.Fatalf("Failed to bind: %v", err)
	}

	params := smpp.NewSubmitSMParams()
	params.RegisteredDelivery = 1

	resp, err := client.SubmitSM("12345", "9876543210", []byte("Direct client message"), params)
	if err != nil {
		log.Printf("Failed to send SMS via direct client: %v", err)
	} else {
		log.Printf("Message sent successfully via direct client! Response: %+v", resp)
	}

	err = client.Unbind()
	if err != nil {
		log.Printf("Failed to unbind: %v", err)
	}

	err = client.Disconnect()
	if err != nil {
		log.Printf("Failed to disconnect: %v", err)
	}
	log.Println("SMPP client example completed")
}

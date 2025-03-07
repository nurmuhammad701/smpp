# Go SMPP Client

A complete SMPP client implementation in Go that follows the SMPP 3.4 specification for sending SMS messages.

## Features

- Full implementation of SMPP 3.4 client
- Support for all basic PDU operations:
  - `bind_transmitter`
  - `bind_receiver`
  - `submit_sm`
  - `deliver_sm`
  - `unbind`
- High-level API for sending SMS messages
- Support for secure TLS connections
- Unicode and binary SMS support
- Connection failure and timeout handling
- Automatic retry mechanisms for message submission
- Support for long message segmentation (SAR)
- Persistent connection management (similar to database connection pooling)
- Comprehensive unit tests

## Installation

```bash
go get github.com/yourusername/smpp
```

## Quick Start

Here's a simple example of how to use the high-level API:

```go
package main

import (
    "log"
    "time"
    
    "github.com/yourusername/smpp"
)

func main() {
    // Create a new SMPP API client
    api, err := smpp.NewSMPPAPI(
        "smpp.example.com", // SMPP server address
        2775,               // Default SMPP port
        "your_username",    // System ID
        "your_password",    // Password
        "",                 // System Type (often empty)
    )
    if err != nil {
        log.Fatalf("Failed to create SMPP API: %v", err)
    }
    
    // Configure reconnection policy and timeouts
    api.WithReconnectPolicy(
        5,                  // Max reconnect attempts (0 for infinite)
        5*time.Second,      // Initial delay
        60*time.Second,     // Max delay
    ).WithTimeouts(
        10*time.Second,     // Connect timeout
        30*time.Second,     // Read timeout
        10*time.Second,     // Write timeout
        60*time.Second,     // Enquire link interval
    )
    
    // Connect to the SMPP server
    err = api.Connect(true) // Use TLS
    if err != nil {
        log.Fatalf("Failed to connect: %v", err)
    }
    defer api.Disconnect()
    
    // Send a regular SMS
    messageID, err := api.SendSMS(&smpp.SMSMessage{
        SourceAddr: "12345",
        DestAddr:   "9876543210",
        Message:    []byte("Hello, this is a test message!"),
        RegisteredDelivery: 1, // Request delivery receipt
    })
    if err != nil {
        log.Printf("Failed to send SMS: %v", err)
    } else {
        log.Printf("Message sent successfully! Message ID: %s", messageID)
    }
}
```

## Connection Management

The SMPP client is designed to maintain persistent connections to the SMPP server, similar to how database clients work. This approach helps prevent spamming the server with frequent connection attempts and improves performance.

Key features:

- The client will automatically try to reconnect when a connection fails
- Enquire link PDUs are sent periodically to keep the connection alive
- You can configure reconnection policies including maximum attempts and backoff delays

## Detailed Documentation

### Creating a Client

#### High-level API

```go
api, err := smpp.NewSMPPAPI(host, port, systemID, password, systemType)
```

#### Low-level Client

```go
client := smpp.NewSMPPClient(host, port, systemID, password, systemType)
```

### Configuring the Client

```go
// Configure TLS
client.WithTLS(tlsConfig)

// Configure timeouts
client.WithTimeouts(connectTimeout, readTimeout, writeTimeout, enquireTimeout)

// Configure reconnection policy
client.WithReconnectPolicy(maxAttempts, initialDelay, maxDelay)
```

### Connecting and Binding

```go
// Using high-level API
api.Connect(useTLS)

// Using low-level client
client.Connect()
client.BindTransmitter() // or client.BindReceiver()
```

### Sending Messages

#### Regular SMS

```go
messageID, err := api.SendSMS(&smpp.SMSMessage{
    SourceAddr: "12345",
    DestAddr:   "9876543210",
    Message:    []byte("Hello world!"),
})
```

#### Unicode SMS

```go
messageID, err := api.SendUnicodeSMS(&smpp.SMSMessage{
    SourceAddr: "12345",
    DestAddr:   "9876543210",
    Message:    []byte("こんにちは世界"), // Hello world in Japanese
})
```

#### Binary SMS

```go
messageID, err := api.SendBinarySMS(&smpp.SMSMessage{
    SourceAddr: "12345",
    DestAddr:   "9876543210",
    Message:    []byte{0x01, 0x02, 0x03, 0x04},
})
```

#### Long Message (Auto-segmentation)

```go
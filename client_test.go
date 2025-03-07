// package/smpp/client_test.go

package smpp

import (
	"testing"
	"time"
)

func TestNextSequenceNumber(t *testing.T) {
	client := NewSMPPClient("localhost", 2775, "test", "test", "")

	// Test sequence number increments
	seq1 := client.nextSequenceNumber()
	seq2 := client.nextSequenceNumber()
	if seq2 != seq1+1 {
		t.Errorf("Expected sequence number to increment from %d to %d, got %d", seq1, seq1+1, seq2)
	}

	// Test sequence number wraps around
	client.SequenceNumber = 0x7FFFFFFF
	seq1 = client.nextSequenceNumber()
	seq2 = client.nextSequenceNumber()
	if seq1 != 0x7FFFFFFF || seq2 != 1 {
		t.Errorf("Expected sequence to wrap from 0x7FFFFFFF to 1, got %d to %d", seq1, seq2)
	}
}

func TestPDUCreation(t *testing.T) {
	// Test creating a bind_transmitter PDU
	pdu := NewPDU(BIND_TRANSMITTER, 0, 123)
	if pdu.CommandID != BIND_TRANSMITTER {
		t.Errorf("Expected CommandID to be %d, got %d", BIND_TRANSMITTER, pdu.CommandID)
	}
	if pdu.CommandStatus != 0 {
		t.Errorf("Expected CommandStatus to be 0, got %d", pdu.CommandStatus)
	}
	if pdu.SequenceNumber != 123 {
		t.Errorf("Expected SequenceNumber to be 123, got %d", pdu.SequenceNumber)
	}
	if pdu.CommandLength != 16 {
		t.Errorf("Expected initial CommandLength to be 16, got %d", pdu.CommandLength)
	}
}

func TestSubmitSMParams(t *testing.T) {
	params := NewSubmitSMParams()

	// Test default values
	if params.DataCoding != 0 {
		t.Errorf("Expected default DataCoding to be 0, got %d", params.DataCoding)
	}
	if params.RegisteredDelivery != 0 {
		t.Errorf("Expected default RegisteredDelivery to be 0, got %d", params.RegisteredDelivery)
	}

	// Test adding a TLV
	params.AddTLV(0x1234, []byte{1, 2, 3})
	if len(params.OptionalParams) != 1 {
		t.Errorf("Expected 1 optional parameter, got %d", len(params.OptionalParams))
	}
	if params.OptionalParams[0].Tag != 0x1234 {
		t.Errorf("Expected TLV tag to be 0x1234, got 0x%X", params.OptionalParams[0].Tag)
	}
	if params.OptionalParams[0].Length != 3 {
		t.Errorf("Expected TLV length to be 3, got %d", params.OptionalParams[0].Length)
	}
	if len(params.OptionalParams[0].Value) != 3 {
		t.Errorf("Expected TLV value length to be 3, got %d", len(params.OptionalParams[0].Value))
	}

	// Test clone method
	clone := params.Clone()
	if len(clone.OptionalParams) != 1 {
		t.Errorf("Expected cloned params to have 1 optional parameter, got %d", len(clone.OptionalParams))
	}

	// Modify original and verify clone is unchanged
	params.AddTLV(0x5678, []byte{4, 5, 6})
	if len(clone.OptionalParams) != 1 {
		t.Errorf("Expected cloned params to still have 1 optional parameter, got %d", len(clone.OptionalParams))
	}
}

func TestSplitLongMessage(t *testing.T) {
	client := NewSMPPClient("localhost", 2775, "test", "test", "")

	// Create a message that's just over the max length for GSM 7-bit
	message := make([]byte, 160)
	for i := range message {
		message[i] = byte('A' + (i % 26))
	}

	// Test GSM 7-bit encoding
	segments := client.SplitLongMessage(message, 0x00)
	if len(segments) != 2 {
		t.Errorf("Expected 2 segments for GSM 7-bit message, got %d", len(segments))
	}

	// Test UCS-2 encoding
	segments = client.SplitLongMessage(message, 0x08)
	if len(segments) != 3 {
		t.Errorf("Expected 3 segments for UCS-2 message, got %d", len(segments))
	}

	// Test binary encoding
	segments = client.SplitLongMessage(message, 0x04)
	if len(segments) != 2 {
		t.Errorf("Expected 2 segments for binary message, got %d", len(segments))
	}
}

func TestSMPPAPI(t *testing.T) {
	// Test API creation
	api, err := NewSMPPAPI("localhost", 2775, "test", "test", "")
	if err != nil {
		t.Errorf("Failed to create SMPP API: %v", err)
	}

	// Test setting timeouts
	api.WithTimeouts(1*time.Second, 2*time.Second, 3*time.Second, 4*time.Second)
	if api.client.ConnectTimeout != 1*time.Second {
		t.Errorf("Expected ConnectTimeout to be 1s, got %v", api.client.ConnectTimeout)
	}
	if api.client.ReadTimeout != 2*time.Second {
		t.Errorf("Expected ReadTimeout to be 2s, got %v", api.client.ReadTimeout)
	}
	if api.client.WriteTimeout != 3*time.Second {
		t.Errorf("Expected WriteTimeout to be 3s, got %v", api.client.WriteTimeout)
	}
	if api.client.EnquireTimeout != 4*time.Second {
		t.Errorf("Expected EnquireTimeout to be 4s, got %v", api.client.EnquireTimeout)
	}

	// Test setting reconnect policy
	api.WithReconnectPolicy(5, 10*time.Second, 60*time.Second)
	if api.client.reconnectAttempts != 5 {
		t.Errorf("Expected reconnectAttempts to be 5, got %d", api.client.reconnectAttempts)
	}
	if api.client.reconnectDelay != 10*time.Second {
		t.Errorf("Expected reconnectDelay to be 10s, got %v", api.client.reconnectDelay)
	}
	if api.client.reconnectMaxDelay != 60*time.Second {
		t.Errorf("Expected reconnectMaxDelay to be 60s, got %v", api.client.reconnectMaxDelay)
	}
}

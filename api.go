// package/smpp/api.go

package smpp

import (
	"errors"
	"time"
)

// SMSMessage represents an SMS message to be sent
type SMSMessage struct {
	SourceAddr           string
	DestAddr             string
	Message              []byte
	DataCoding           byte
	RegisteredDelivery   byte
	ServiceType          string
	ValidityPeriod       string
	ScheduleDeliveryTime string
	PriorityFlag         byte
	ESMClass             byte
}

// SMPPAPI provides a simplified API for sending SMS messages
type SMPPAPI struct {
	client *SMPPClient
}

// NewSMPPAPI creates a new SMPP API
func NewSMPPAPI(host string, port int, systemID, password, systemType string) (*SMPPAPI, error) {
	client := NewSMPPClient(host, port, systemID, password, systemType)
	api := &SMPPAPI{client: client}
	return api, nil
}

// Connect connects to the SMPP server
func (api *SMPPAPI) Connect(useTLS bool) error {
	if useTLS {
		api.client.UseTLS = true
	}

	err := api.client.Connect()
	if err != nil {
		return err
	}

	_, err = api.client.BindTransmitter()
	return err
}

// Disconnect disconnects from the SMPP server
func (api *SMPPAPI) Disconnect() error {
	return api.client.Disconnect()
}

// SendSMS sends an SMS message
func (api *SMPPAPI) SendSMS(msg *SMSMessage) (string, error) {
	if !api.client.Bound || api.client.BindType != BIND_TRANSMITTER {
		return "", errors.New("not bound as transmitter")
	}

	params := NewSubmitSMParams()
	params.ServiceType = msg.ServiceType
	params.DataCoding = msg.DataCoding
	params.RegisteredDelivery = msg.RegisteredDelivery
	params.ValidityPeriod = msg.ValidityPeriod
	params.ScheduleDeliveryTime = msg.ScheduleDeliveryTime
	params.PriorityFlag = msg.PriorityFlag
	params.ESMClass = msg.ESMClass

	// Check if the message should use the message_payload TLV
	if len(msg.Message) > 254 {
		params.UseMessagePayload = true
	}

	resp, err := api.client.SubmitSMWithRetry(msg.SourceAddr, msg.DestAddr, msg.Message, params, 3, 2*time.Second)
	if err != nil {
		return "", err
	}

	// Extract the message ID from the response body
	var messageID string
	if len(resp.Body) > 0 {
		// Remove the null terminator if present
		if resp.Body[len(resp.Body)-1] == 0 {
			messageID = string(resp.Body[:len(resp.Body)-1])
		} else {
			messageID = string(resp.Body)
		}
	}

	return messageID, nil
}

// SendLongSMS sends a long SMS message (using segmentation)
func (api *SMPPAPI) SendLongSMS(msg *SMSMessage) ([]string, error) {
	if !api.client.Bound || api.client.BindType != BIND_TRANSMITTER {
		return nil, errors.New("not bound as transmitter")
	}

	params := NewSubmitSMParams()
	params.ServiceType = msg.ServiceType
	params.DataCoding = msg.DataCoding
	params.RegisteredDelivery = msg.RegisteredDelivery
	params.ValidityPeriod = msg.ValidityPeriod
	params.ScheduleDeliveryTime = msg.ScheduleDeliveryTime
	params.PriorityFlag = msg.PriorityFlag
	params.ESMClass = msg.ESMClass

	return api.client.SendLongMessage(msg.SourceAddr, msg.DestAddr, msg.Message, params)
}

// SendUnicodeSMS sends a Unicode SMS message
func (api *SMPPAPI) SendUnicodeSMS(msg *SMSMessage) (string, error) {
	// Set data coding to UCS2 (Unicode)
	msg.DataCoding = 0x08
	return api.SendSMS(msg)
}

// SendBinarySMS sends a binary SMS message
func (api *SMPPAPI) SendBinarySMS(msg *SMSMessage) (string, error) {
	// Set data coding to binary
	msg.DataCoding = 0x04
	// Set ESM class to indicate binary data
	msg.ESMClass |= 0x04
	return api.SendSMS(msg)
}

// WithReconnectPolicy sets the reconnect policy
func (api *SMPPAPI) WithReconnectPolicy(attempts int, initialDelay, maxDelay time.Duration) *SMPPAPI {
	api.client.WithReconnectPolicy(attempts, initialDelay, maxDelay)
	return api
}

// WithTimeouts sets custom timeouts
func (api *SMPPAPI) WithTimeouts(connect, read, write, enquire time.Duration) *SMPPAPI {
	api.client.WithTimeouts(connect, read, write, enquire)
	return api
}

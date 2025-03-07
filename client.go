// package/smpp/client.go

package smpp

import (
	"crypto/tls"
	"errors"
	"fmt"
	"log"
	"sync"
	"time"
)

const (
	// PDU Command IDs
	BIND_TRANSMITTER      uint32 = 0x00000002
	BIND_TRANSMITTER_RESP uint32 = 0x80000002
	BIND_RECEIVER         uint32 = 0x00000001
	BIND_RECEIVER_RESP    uint32 = 0x80000001
	SUBMIT_SM             uint32 = 0x00000004
	SUBMIT_SM_RESP        uint32 = 0x80000004
	DELIVER_SM            uint32 = 0x00000005
	DELIVER_SM_RESP       uint32 = 0x80000005
	UNBIND                uint32 = 0x00000006
	UNBIND_RESP           uint32 = 0x80000006
	ENQUIRE_LINK          uint32 = 0x00000015
	ENQUIRE_LINK_RESP     uint32 = 0x80000015

	// SMPP Error Codes
	ESME_ROK uint32 = 0x00000000

	// Default timeouts
	DefaultConnectTimeout = 10 * time.Second
	DefaultReadTimeout    = 30 * time.Second
	DefaultWriteTimeout   = 10 * time.Second
	DefaultEnquireTimeout = 60 * time.Second

	// Connection states
	DISCONNECTED = 0
	CONNECTED    = 1
	BOUND        = 2
)

// SMPPClient represents an SMPP client
type SMPPClient struct {
	Host               string
	Port               int
	SystemID           string
	Password           string
	SystemType         string
	AddrTON            byte
	AddrNPI            byte
	AddressRange       string
	conn               *Connection
	Bound              bool
	UseTLS             bool
	TLSConfig          *tls.Config
	BindType           uint32
	EnquireTimeout     time.Duration
	ConnectTimeout     time.Duration
	ReadTimeout        time.Duration
	WriteTimeout       time.Duration
	SequenceNumber     uint32
	sequenceLock       sync.Mutex
	pduResponseMap     map[uint32]chan *PDU
	pduResponseMapLock sync.Mutex
	status             int
	reconnectAttempts  int
	reconnectDelay     time.Duration
	reconnectMaxDelay  time.Duration
	stopChan           chan struct{}
	enquireTimer       *time.Timer
}

// NewSMPPClient creates a new SMPP client
func NewSMPPClient(host string, port int, systemID, password, systemType string) *SMPPClient {
	return &SMPPClient{
		Host:              host,
		Port:              port,
		SystemID:          systemID,
		Password:          password,
		SystemType:        systemType,
		AddrTON:           0,
		AddrNPI:           0,
		AddressRange:      "",
		Bound:             false,
		UseTLS:            false,
		EnquireTimeout:    DefaultEnquireTimeout,
		ConnectTimeout:    DefaultConnectTimeout,
		ReadTimeout:       DefaultReadTimeout,
		WriteTimeout:      DefaultWriteTimeout,
		SequenceNumber:    1,
		pduResponseMap:    make(map[uint32]chan *PDU),
		status:            DISCONNECTED,
		reconnectAttempts: 0,
		reconnectDelay:    5 * time.Second,
		reconnectMaxDelay: 5 * time.Minute,
		stopChan:          make(chan struct{}),
	}
}

// WithTLS configures the client to use TLS
func (c *SMPPClient) WithTLS(tlsConfig *tls.Config) *SMPPClient {
	c.UseTLS = true
	c.TLSConfig = tlsConfig
	return c
}

// WithTimeouts sets custom timeouts for the client
func (c *SMPPClient) WithTimeouts(connect, read, write, enquire time.Duration) *SMPPClient {
	c.ConnectTimeout = connect
	c.ReadTimeout = read
	c.WriteTimeout = write
	c.EnquireTimeout = enquire
	return c
}

// WithReconnectPolicy sets the reconnect policy
func (c *SMPPClient) WithReconnectPolicy(attempts int, initialDelay, maxDelay time.Duration) *SMPPClient {
	c.reconnectAttempts = attempts
	c.reconnectDelay = initialDelay
	c.reconnectMaxDelay = maxDelay
	return c
}

// Connect establishes a connection to the SMPP server
func (c *SMPPClient) Connect() error {
	if c.status != DISCONNECTED {
		return errors.New("already connected or connecting")
	}

	c.conn = NewConnection(c.Host, c.Port, c.ConnectTimeout, c.ReadTimeout, c.WriteTimeout)

	if c.UseTLS {
		err := c.conn.ConnectTLS(c.TLSConfig)
		if err != nil {
			return fmt.Errorf("TLS connection failed: %v", err)
		}
	} else {
		err := c.conn.Connect()
		if err != nil {
			return fmt.Errorf("connection failed: %v", err)
		}
	}

	c.status = CONNECTED
	go c.readPDUs()
	return nil
}

// Disconnect closes the connection to the SMPP server
func (c *SMPPClient) Disconnect() error {
	if c.status == DISCONNECTED {
		return nil
	}

	// If we're bound, we should unbind first
	if c.status == BOUND {
		err := c.Unbind()
		if err != nil {
			log.Printf("Error unbinding: %v", err)
			// Continue with disconnection anyway
		}
	}

	if c.enquireTimer != nil {
		c.enquireTimer.Stop()
	}

	err := c.conn.Close()
	c.status = DISCONNECTED
	c.Bound = false

	// Close the stop channel to terminate any goroutines
	close(c.stopChan)

	return err
}

// nextSequenceNumber generates the next sequence number
func (c *SMPPClient) nextSequenceNumber() uint32 {
	c.sequenceLock.Lock()
	defer c.sequenceLock.Unlock()

	seqNum := c.SequenceNumber
	c.SequenceNumber++
	if c.SequenceNumber > 0x7FFFFFFF {
		c.SequenceNumber = 1
	}
	return seqNum
}

// readPDUs reads PDUs from the connection
func (c *SMPPClient) readPDUs() {
	for {
		select {
		case <-c.stopChan:
			return
		default:
			pdu, err := c.conn.ReadPDU()
			if err != nil {
				log.Printf("Error reading PDU: %v", err)
				c.handleConnectionFailure()
				return
			}

			// Handle the PDU
			c.handlePDU(pdu)
		}
	}
}

// handlePDU processes a received PDU
func (c *SMPPClient) handlePDU(pdu *PDU) {
	// Check if it's a response to a previously sent PDU
	if pdu.CommandID&0x80000000 != 0 {
		c.handleResponse(pdu)
		return
	}

	// It's a request from the server
	switch pdu.CommandID {
	case DELIVER_SM:
		c.handleDeliverSM(pdu)
	case ENQUIRE_LINK:
		c.handleEnquireLink(pdu)
	default:
		log.Printf("Unhandled PDU: %+v", pdu)
	}
}

// handleResponse processes a response PDU
func (c *SMPPClient) handleResponse(pdu *PDU) {
	c.pduResponseMapLock.Lock()
	defer c.pduResponseMapLock.Unlock()

	if responseChan, exists := c.pduResponseMap[pdu.SequenceNumber]; exists {
		responseChan <- pdu
		delete(c.pduResponseMap, pdu.SequenceNumber)
	} else {
		log.Printf("Received response for unknown sequence number: %d", pdu.SequenceNumber)
	}
}

// sendPDU sends a PDU and waits for a response
func (c *SMPPClient) sendPDU(pdu *PDU) (*PDU, error) {
	if c.status == DISCONNECTED {
		return nil, errors.New("not connected")
	}

	responseChan := make(chan *PDU, 1)
	c.pduResponseMapLock.Lock()
	c.pduResponseMap[pdu.SequenceNumber] = responseChan
	c.pduResponseMapLock.Unlock()

	err := c.conn.WritePDU(pdu)
	if err != nil {
		c.pduResponseMapLock.Lock()
		delete(c.pduResponseMap, pdu.SequenceNumber)
		c.pduResponseMapLock.Unlock()
		c.handleConnectionFailure()
		return nil, fmt.Errorf("error writing PDU: %v", err)
	}

	select {
	case resp := <-responseChan:
		return resp, nil
	case <-time.After(c.ReadTimeout):
		c.pduResponseMapLock.Lock()
		delete(c.pduResponseMap, pdu.SequenceNumber)
		c.pduResponseMapLock.Unlock()
		return nil, errors.New("response timeout")
	}
}

// handleConnectionFailure handles a connection failure
func (c *SMPPClient) handleConnectionFailure() {
	if c.status == DISCONNECTED {
		return
	}

	// Stop the enquire timer
	if c.enquireTimer != nil {
		c.enquireTimer.Stop()
	}

	c.conn.Close()
	c.status = DISCONNECTED
	c.Bound = false

	// Attempt to reconnect
	go c.reconnect()
}

// reconnect attempts to reconnect to the SMPP server
func (c *SMPPClient) reconnect() {
	attempts := 0
	delay := c.reconnectDelay

	for c.reconnectAttempts == 0 || attempts < c.reconnectAttempts {
		log.Printf("Attempting to reconnect (attempt %d)", attempts+1)
		err := c.Connect()
		if err == nil {
			log.Printf("Successfully reconnected to SMPP server")
			// If we were bound before, try to bind again
			if c.BindType != 0 {
				switch c.BindType {
				case BIND_TRANSMITTER:
					_, err = c.BindTransmitter()
				case BIND_RECEIVER:
					_, err = c.BindReceiver()
				}

				if err != nil {
					log.Printf("Failed to rebind: %v", err)
					c.conn.Close()
					c.status = DISCONNECTED
				} else {
					log.Printf("Successfully rebound")
					return
				}
			} else {
				return
			}
		} else {
			log.Printf("Reconnection failed: %v", err)
		}

		attempts++

		// Exponential backoff with jitter
		select {
		case <-c.stopChan:
			return
		case <-time.After(delay):
			delay *= 2
			if delay > c.reconnectMaxDelay {
				delay = c.reconnectMaxDelay
			}
		}
	}

	log.Printf("Failed to reconnect after %d attempts", attempts)
}

// BindTransmitter binds the client as a transmitter
func (c *SMPPClient) BindTransmitter() (*PDU, error) {
	if c.status != CONNECTED {
		return nil, errors.New("not connected")
	}

	seqNum := c.nextSequenceNumber()
	pdu := NewPDU(BIND_TRANSMITTER, 0, seqNum)

	pdu.Body = append(pdu.Body, []byte(c.SystemID)...)
	pdu.Body = append(pdu.Body, 0)
	pdu.Body = append(pdu.Body, []byte(c.Password)...)
	pdu.Body = append(pdu.Body, 0)
	pdu.Body = append(pdu.Body, []byte(c.SystemType)...)
	pdu.Body = append(pdu.Body, 0)

	// Interface version
	pdu.Body = append(pdu.Body, 0x34) // 3.4

	// TON, NPI, and Address Range
	pdu.Body = append(pdu.Body, c.AddrTON)
	pdu.Body = append(pdu.Body, c.AddrNPI)
	pdu.Body = append(pdu.Body, []byte(c.AddressRange)...)
	pdu.Body = append(pdu.Body, 0)

	resp, err := c.sendPDU(pdu)
	if err != nil {
		return nil, err
	}

	if resp.CommandStatus != ESME_ROK {
		return resp, fmt.Errorf("bind failed with status: %d", resp.CommandStatus)
	}

	c.Bound = true
	c.status = BOUND
	c.BindType = BIND_TRANSMITTER

	// Start the enquire link timer
	c.startEnquireTimer()

	return resp, nil
}

// BindReceiver binds the client as a receiver
func (c *SMPPClient) BindReceiver() (*PDU, error) {
	if c.status != CONNECTED {
		return nil, errors.New("not connected")
	}

	seqNum := c.nextSequenceNumber()
	pdu := NewPDU(BIND_RECEIVER, 0, seqNum)

	pdu.Body = append(pdu.Body, []byte(c.SystemID)...)
	pdu.Body = append(pdu.Body, 0)
	pdu.Body = append(pdu.Body, []byte(c.Password)...)
	pdu.Body = append(pdu.Body, 0)
	pdu.Body = append(pdu.Body, []byte(c.SystemType)...)
	pdu.Body = append(pdu.Body, 0)

	// Interface version
	pdu.Body = append(pdu.Body, 0x34) // 3.4

	// TON, NPI, and Address Range
	pdu.Body = append(pdu.Body, c.AddrTON)
	pdu.Body = append(pdu.Body, c.AddrNPI)
	pdu.Body = append(pdu.Body, []byte(c.AddressRange)...)
	pdu.Body = append(pdu.Body, 0)

	resp, err := c.sendPDU(pdu)
	if err != nil {
		return nil, err
	}

	if resp.CommandStatus != ESME_ROK {
		return resp, fmt.Errorf("bind failed with status: %d", resp.CommandStatus)
	}

	c.Bound = true
	c.status = BOUND
	c.BindType = BIND_RECEIVER

	// Start the enquire link timer
	c.startEnquireTimer()

	return resp, nil
}

// Unbind unbinds the client from the server
func (c *SMPPClient) Unbind() error {
	if !c.Bound {
		return errors.New("not bound")
	}

	seqNum := c.nextSequenceNumber()
	pdu := NewPDU(UNBIND, 0, seqNum)

	resp, err := c.sendPDU(pdu)
	if err != nil {
		return err
	}

	if resp.CommandStatus != ESME_ROK {
		return fmt.Errorf("unbind failed with status: %d", resp.CommandStatus)
	}

	c.Bound = false
	c.status = CONNECTED

	// Stop the enquire link timer
	if c.enquireTimer != nil {
		c.enquireTimer.Stop()
	}

	return nil
}

// SubmitSM sends an SMS message
func (c *SMPPClient) SubmitSM(sourceAddr, destAddr string, shortMessage []byte, params *SubmitSMParams) (*PDU, error) {
	if !c.Bound || c.BindType != BIND_TRANSMITTER {
		return nil, errors.New("not bound as transmitter")
	}

	if params == nil {
		params = NewSubmitSMParams()
	}

	seqNum := c.nextSequenceNumber()
	pdu := NewPDU(SUBMIT_SM, 0, seqNum)

	// service_type
	pdu.Body = append(pdu.Body, []byte(params.ServiceType)...)
	pdu.Body = append(pdu.Body, 0)

	// source_addr_ton
	pdu.Body = append(pdu.Body, params.SourceAddrTON)

	// source_addr_npi
	pdu.Body = append(pdu.Body, params.SourceAddrNPI)

	// source_addr
	pdu.Body = append(pdu.Body, []byte(sourceAddr)...)
	pdu.Body = append(pdu.Body, 0)

	// dest_addr_ton
	pdu.Body = append(pdu.Body, params.DestAddrTON)

	// dest_addr_npi
	pdu.Body = append(pdu.Body, params.DestAddrNPI)

	// destination_addr
	pdu.Body = append(pdu.Body, []byte(destAddr)...)
	pdu.Body = append(pdu.Body, 0)

	// esm_class
	pdu.Body = append(pdu.Body, params.ESMClass)

	// protocol_id
	pdu.Body = append(pdu.Body, params.ProtocolID)

	// priority_flag
	pdu.Body = append(pdu.Body, params.PriorityFlag)

	// schedule_delivery_time
	pdu.Body = append(pdu.Body, []byte(params.ScheduleDeliveryTime)...)
	pdu.Body = append(pdu.Body, 0)

	// validity_period
	pdu.Body = append(pdu.Body, []byte(params.ValidityPeriod)...)
	pdu.Body = append(pdu.Body, 0)

	// registered_delivery
	pdu.Body = append(pdu.Body, params.RegisteredDelivery)

	// replace_if_present_flag
	pdu.Body = append(pdu.Body, params.ReplaceIfPresentFlag)

	// data_coding
	pdu.Body = append(pdu.Body, params.DataCoding)

	// sm_default_msg_id
	pdu.Body = append(pdu.Body, params.SMDefaultMsgID)

	// sm_length
	if len(shortMessage) > 254 && !params.UseMessagePayload {
		return nil, errors.New("message too long")
	}

	if params.UseMessagePayload {
		// Set sm_length to 0 when using message_payload TLV
		pdu.Body = append(pdu.Body, 0)

		// Add message_payload TLV
		tag := uint16(0x0424)
		length := uint16(len(shortMessage))

		pdu.Body = append(pdu.Body, byte(tag>>8), byte(tag&0xFF))
		pdu.Body = append(pdu.Body, byte(length>>8), byte(length&0xFF))
		pdu.Body = append(pdu.Body, shortMessage...)
	} else {
		// Use normal short_message field
		pdu.Body = append(pdu.Body, byte(len(shortMessage)))
		pdu.Body = append(pdu.Body, shortMessage...)
	}

	// Add optional parameters
	for _, tlv := range params.OptionalParams {
		pdu.Body = append(pdu.Body, byte(tlv.Tag>>8), byte(tlv.Tag&0xFF))
		pdu.Body = append(pdu.Body, byte(tlv.Length>>8), byte(tlv.Length&0xFF))
		pdu.Body = append(pdu.Body, tlv.Value...)
	}

	return c.sendPDU(pdu)
}

// SubmitSMWithRetry attempts to send an SMS with retries
func (c *SMPPClient) SubmitSMWithRetry(sourceAddr, destAddr string, shortMessage []byte, params *SubmitSMParams, maxRetries int, retryDelay time.Duration) (*PDU, error) {
	var resp *PDU
	var err error

	for i := 0; i <= maxRetries; i++ {
		if i > 0 {
			log.Printf("Retrying SMS submission (attempt %d/%d)", i, maxRetries)
			time.Sleep(retryDelay)
		}

		resp, err = c.SubmitSM(sourceAddr, destAddr, shortMessage, params)
		if err == nil && resp.CommandStatus == ESME_ROK {
			return resp, nil
		}

		// If we're not connected or bound, try to reconnect
		if !c.Bound || c.status != BOUND {
			log.Printf("Lost connection, attempting to reconnect...")
			err = c.Connect()
			if err != nil {
				log.Printf("Reconnection failed: %v", err)
				continue
			}

			_, err = c.BindTransmitter()
			if err != nil {
				log.Printf("Rebind failed: %v", err)
				continue
			}
		}
	}

	return resp, err
}

// SplitLongMessage splits a long message for segmentation
func (c *SMPPClient) SplitLongMessage(message []byte, encoding byte) [][]byte {
	var maxLength int

	// Determine max segment length based on encoding
	switch encoding {
	case 0x00: // GSM 7-bit
		maxLength = 160 - 7 // Reserve 7 bytes for UDH
	case 0x08: // UCS-2 (16-bit)
		maxLength = 70 - 7 // Reserve 7 bytes for UDH
	default:
		maxLength = 140 - 7 // 8-bit
	}

	segments := make([][]byte, 0)
	msgLen := len(message)

	for i := 0; i < msgLen; i += maxLength {
		end := i + maxLength
		if end > msgLen {
			end = msgLen
		}
		segments = append(segments, message[i:end])
	}

	return segments
}

// SendLongMessage sends a long message using SAR (Segmentation and Reassembly)
func (c *SMPPClient) SendLongMessage(sourceAddr, destAddr string, message []byte, params *SubmitSMParams) ([]string, error) {
	if params == nil {
		params = NewSubmitSMParams()
	}

	segments := c.SplitLongMessage(message, params.DataCoding)
	numSegments := len(segments)

	if numSegments <= 1 {
		// Message fits in a single SMS
		resp, err := c.SubmitSM(sourceAddr, destAddr, message, params)
		if err != nil {
			return nil, err
		}

		msgID := string(resp.Body)
		return []string{msgID}, nil
	}

	// Generate a reference number for this message
	refNum := uint16(c.nextSequenceNumber() & 0xFFFF)
	msgIDs := make([]string, 0, numSegments)

	for i, segment := range segments {
		// Create a copy of the parameters for this segment
		segParams := params.Clone()

		// Add SAR TLVs
		segParams.AddTLV(0x020C, []byte{byte(refNum >> 8), byte(refNum & 0xFF)}) // sar_msg_ref_num
		segParams.AddTLV(0x020E, []byte{byte(numSegments)})                      // sar_total_segments
		segParams.AddTLV(0x020F, []byte{byte(i + 1)})                            // sar_segment_seqnum

		// Send this segment
		resp, err := c.SubmitSMWithRetry(sourceAddr, destAddr, segment, segParams, 3, 2*time.Second)
		if err != nil {
			return msgIDs, fmt.Errorf("failed to send segment %d: %v", i+1, err)
		}

		msgID := string(resp.Body)
		msgIDs = append(msgIDs, msgID)
	}

	return msgIDs, nil
}

// startEnquireTimer starts the enquire link timer
func (c *SMPPClient) startEnquireTimer() {
	if c.enquireTimer != nil {
		c.enquireTimer.Stop()
	}

	c.enquireTimer = time.AfterFunc(c.EnquireTimeout, func() {
		c.sendEnquireLink()
	})
}

// sendEnquireLink sends an enquire link PDU
func (c *SMPPClient) sendEnquireLink() {
	if c.status != BOUND {
		return
	}

	seqNum := c.nextSequenceNumber()
	pdu := NewPDU(ENQUIRE_LINK, 0, seqNum)

	_, err := c.sendPDU(pdu)
	if err != nil {
		log.Printf("Enquire link failed: %v", err)
		c.handleConnectionFailure()
		return
	}

	// Restart the timer
	c.startEnquireTimer()
}

// handleEnquireLink handles an enquire link request from the server
func (c *SMPPClient) handleEnquireLink(pdu *PDU) {
	resp := NewPDU(ENQUIRE_LINK_RESP, 0, pdu.SequenceNumber)
	err := c.conn.WritePDU(resp)
	if err != nil {
		log.Printf("Failed to send enquire_link_resp: %v", err)
		c.handleConnectionFailure()
	}
}

// handleDeliverSM handles a deliver_sm PDU from the server
func (c *SMPPClient) handleDeliverSM(pdu *PDU) {
	// Parse the deliver_sm PDU
	// This is a simplified implementation

	// Send a deliver_sm_resp
	resp := NewPDU(DELIVER_SM_RESP, 0, pdu.SequenceNumber)
	resp.Body = append(resp.Body, 0) // message_id (not used in deliver_sm_resp)

	err := c.conn.WritePDU(resp)
	if err != nil {
		log.Printf("Failed to send deliver_sm_resp: %v", err)
		c.handleConnectionFailure()
		return
	}

	// TODO: Implement proper delivery receipt processing
	// In a real implementation, you would parse the deliver_sm PDU
	// and notify your application
}

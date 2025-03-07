// package/smpp/pdu.go

package smpp

// PDU represents an SMPP Protocol Data Unit
type PDU struct {
	CommandLength  uint32
	CommandID      uint32
	CommandStatus  uint32
	SequenceNumber uint32
	Body           []byte
}

// NewPDU creates a new PDU
func NewPDU(commandID, commandStatus, sequenceNumber uint32) *PDU {
	return &PDU{
		CommandLength:  16, // Initial length is just the header
		CommandID:      commandID,
		CommandStatus:  commandStatus,
		SequenceNumber: sequenceNumber,
		Body:           make([]byte, 0),
	}
}

// TLV represents a Tag-Length-Value optional parameter
type TLV struct {
	Tag    uint16
	Length uint16
	Value  []byte
}

// SubmitSMParams contains parameters for the submit_sm operation
type SubmitSMParams struct {
	ServiceType          string
	SourceAddrTON        byte
	SourceAddrNPI        byte
	DestAddrTON          byte
	DestAddrNPI          byte
	ESMClass             byte
	ProtocolID           byte
	PriorityFlag         byte
	ScheduleDeliveryTime string
	ValidityPeriod       string
	RegisteredDelivery   byte
	ReplaceIfPresentFlag byte
	DataCoding           byte
	SMDefaultMsgID       byte
	UseMessagePayload    bool
	OptionalParams       []TLV
}

// NewSubmitSMParams creates a new SubmitSMParams with default values
func NewSubmitSMParams() *SubmitSMParams {
	return &SubmitSMParams{
		ServiceType:          "",
		SourceAddrTON:        0,
		SourceAddrNPI:        0,
		DestAddrTON:          1,
		DestAddrNPI:          1,
		ESMClass:             0,
		ProtocolID:           0,
		PriorityFlag:         0,
		ScheduleDeliveryTime: "",
		ValidityPeriod:       "",
		RegisteredDelivery:   0,
		ReplaceIfPresentFlag: 0,
		DataCoding:           0,
		SMDefaultMsgID:       0,
		UseMessagePayload:    false,
		OptionalParams:       make([]TLV, 0),
	}
}

// Clone creates a copy of the SubmitSMParams
func (p *SubmitSMParams) Clone() *SubmitSMParams {
	clone := &SubmitSMParams{
		ServiceType:          p.ServiceType,
		SourceAddrTON:        p.SourceAddrTON,
		SourceAddrNPI:        p.SourceAddrNPI,
		DestAddrTON:          p.DestAddrTON,
		DestAddrNPI:          p.DestAddrNPI,
		ESMClass:             p.ESMClass,
		ProtocolID:           p.ProtocolID,
		PriorityFlag:         p.PriorityFlag,
		ScheduleDeliveryTime: p.ScheduleDeliveryTime,
		ValidityPeriod:       p.ValidityPeriod,
		RegisteredDelivery:   p.RegisteredDelivery,
		ReplaceIfPresentFlag: p.ReplaceIfPresentFlag,
		DataCoding:           p.DataCoding,
		SMDefaultMsgID:       p.SMDefaultMsgID,
		UseMessagePayload:    p.UseMessagePayload,
		OptionalParams:       make([]TLV, len(p.OptionalParams)),
	}

	// Deep copy the optional parameters
	for i, tlv := range p.OptionalParams {
		newTLV := TLV{
			Tag:    tlv.Tag,
			Length: tlv.Length,
			Value:  make([]byte, len(tlv.Value)),
		}
		copy(newTLV.Value, tlv.Value)
		clone.OptionalParams[i] = newTLV
	}

	return clone
}

// AddTLV adds a TLV optional parameter
func (p *SubmitSMParams) AddTLV(tag uint16, value []byte) {
	tlv := TLV{
		Tag:    tag,
		Length: uint16(len(value)),
		Value:  value,
	}
	p.OptionalParams = append(p.OptionalParams, tlv)
}

// SetUDH sets the User Data Header for concatenated messages
func (p *SubmitSMParams) SetUDH(refNum byte, totalParts byte, partNum byte) {
	// Set ESM class to indicate UDH is present
	p.ESMClass |= 0x40

	// Create UDH for concatenated messages
	udh := []byte{
		0x05,       // UDH Length
		0x00,       // Information Element Identifier (0 = Concatenated message)
		0x03,       // Information Element Length
		refNum,     // Reference Number
		totalParts, // Total Number of Parts
		partNum,    // Part Number
	}

	p.AddTLV(0x0424, udh) // message_payload TLV with UDH
}

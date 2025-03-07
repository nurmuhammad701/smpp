// package/smpp/connection.go

package smpp

import (
	"crypto/tls"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"time"
)

// Connection represents a connection to an SMPP server
type Connection struct {
	Host           string
	Port           int
	Conn           net.Conn
	ConnectTimeout time.Duration
	ReadTimeout    time.Duration
	WriteTimeout   time.Duration
}

// NewConnection creates a new connection
func NewConnection(host string, port int, connectTimeout, readTimeout, writeTimeout time.Duration) *Connection {
	return &Connection{
		Host:           host,
		Port:           port,
		ConnectTimeout: connectTimeout,
		ReadTimeout:    readTimeout,
		WriteTimeout:   writeTimeout,
	}
}

// Connect establishes a connection to the SMPP server
func (c *Connection) Connect() error {
	addr := fmt.Sprintf("%s:%d", c.Host, c.Port)
	dialer := net.Dialer{Timeout: c.ConnectTimeout}

	conn, err := dialer.Dial("tcp", addr)
	if err != nil {
		return err
	}

	c.Conn = conn
	return nil
}

// ConnectTLS establishes a TLS connection to the SMPP server
func (c *Connection) ConnectTLS(config *tls.Config) error {
	addr := fmt.Sprintf("%s:%d", c.Host, c.Port)
	dialer := net.Dialer{Timeout: c.ConnectTimeout}

	conn, err := tls.DialWithDialer(&dialer, "tcp", addr, config)
	if err != nil {
		return err
	}

	c.Conn = conn
	return nil
}

// Close closes the connection
func (c *Connection) Close() error {
	if c.Conn == nil {
		return nil
	}

	err := c.Conn.Close()
	c.Conn = nil
	return err
}

// WritePDU writes a PDU to the connection
func (c *Connection) WritePDU(pdu *PDU) error {
	if c.Conn == nil {
		return errors.New("not connected")
	}

	err := c.Conn.SetWriteDeadline(time.Now().Add(c.WriteTimeout))
	if err != nil {
		return err
	}

	// Update the PDU length
	pdu.CommandLength = uint32(16 + len(pdu.Body)) // 16 bytes header + body length

	// Write the PDU header
	headerBuf := make([]byte, 16)
	binary.BigEndian.PutUint32(headerBuf[0:4], pdu.CommandLength)
	binary.BigEndian.PutUint32(headerBuf[4:8], pdu.CommandID)
	binary.BigEndian.PutUint32(headerBuf[8:12], pdu.CommandStatus)
	binary.BigEndian.PutUint32(headerBuf[12:16], pdu.SequenceNumber)

	_, err = c.Conn.Write(headerBuf)
	if err != nil {
		return err
	}

	// Write the PDU body
	if len(pdu.Body) > 0 {
		_, err = c.Conn.Write(pdu.Body)
		if err != nil {
			return err
		}
	}

	return nil
}

// ReadPDU reads a PDU from the connection
func (c *Connection) ReadPDU() (*PDU, error) {
	if c.Conn == nil {
		return nil, errors.New("not connected")
	}

	err := c.Conn.SetReadDeadline(time.Now().Add(c.ReadTimeout))
	if err != nil {
		return nil, err
	}

	// Read the PDU header
	headerBuf := make([]byte, 16)
	_, err = io.ReadFull(c.Conn, headerBuf)
	if err != nil {
		return nil, err
	}

	pdu := &PDU{}
	pdu.CommandLength = binary.BigEndian.Uint32(headerBuf[0:4])
	pdu.CommandID = binary.BigEndian.Uint32(headerBuf[4:8])
	pdu.CommandStatus = binary.BigEndian.Uint32(headerBuf[8:12])
	pdu.SequenceNumber = binary.BigEndian.Uint32(headerBuf[12:16])

	// Read the PDU body
	bodyLength := pdu.CommandLength - 16
	if bodyLength > 0 {
		pdu.Body = make([]byte, bodyLength)
		_, err = io.ReadFull(c.Conn, pdu.Body)
		if err != nil {
			return nil, err
		}
	}

	return pdu, nil
}

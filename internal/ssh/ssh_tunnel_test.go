package ssh

import (
	"bytes"
	"io"
	"testing"
)

func TestTunnelMethods(t *testing.T) {
	t.Run("TestDirectAccess", func(t *testing.T) {
		proxyInput := &MockWriteCloser{Buffer: bytes.NewBuffer(nil)}
		proxyOutput := bytes.NewBuffer(nil)

		conn := &Conn{
			proxyInput:  proxyInput,
			proxyOutput: proxyOutput,
		}

		if conn.BinaryInput() != proxyInput {
			t.Error("BinaryInput() did not return the expected writer")
		}

		if conn.BinaryOutput() != proxyOutput {
			t.Error("BinaryOutput() did not return the expected reader")
		}
	})

	t.Run("TestDataTransfer", func(t *testing.T) {
		inBuffer := bytes.NewBuffer(nil)
		outBuffer := bytes.NewBuffer([]byte("test response data"))

		proxyInput := &MockWriteCloser{Buffer: inBuffer}
		proxyOutput := outBuffer

		conn := &Conn{
			proxyInput:  proxyInput,
			proxyOutput: proxyOutput,
		}

		testData := []byte("test request data")

		n, err := conn.BinaryInput().Write(testData)
		if err != nil {
			t.Errorf("Failed to write to binary input: %v", err)
		}
		if n != len(testData) {
			t.Errorf("Expected to write %d bytes, wrote %d", len(testData), n)
		}
		if !bytes.Equal(inBuffer.Bytes(), testData) {
			t.Errorf("Binary input data mismatch, got %v, expected %v", inBuffer.Bytes(), testData)
		}

		responseBuffer := make([]byte, 100)

		n, err = conn.BinaryOutput().Read(responseBuffer)
		if err != nil && err != io.EOF {
			t.Errorf("Failed to read from binary output: %v", err)
		}
		if n != len("test response data") {
			t.Errorf("Expected to read %d bytes, read %d", len("test response data"), n)
		}
		if !bytes.Equal(responseBuffer[:n], []byte("test response data")) {
			t.Errorf("Binary output data mismatch, got %v, expected %v", responseBuffer[:n], []byte("test response data"))
		}
	})
}

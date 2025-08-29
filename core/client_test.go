package core

import (
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRequestResponse(t *testing.T) {
	c := &Client{}
	c.onRequest = func(rh *RequestHeader) bool { return true }

	receiver, sender := net.Pipe()
	defer receiver.Close()
	defer sender.Close()

	r := &RequestHeader{
		nbytes: 420,
		n:      69,
	}

	go c.WriteRequest(receiver, r)

	rr, err := c.ReadRequest(sender)
	if err != nil {
		t.Error(err)
	}

	rEncoded, err := r.Encoded()
	if err != nil {
		t.Error(err)
	}

	rEncodedBytes, ok := rEncoded.(*EncodedRequestHeader)
	if !ok {
		t.Error("failed to type assert to EncodedRequestHeader")
	}

	rrEncoded, err := rr.Encoded()
	if err != nil {
		t.Error(err)
	}

	rrEncodedBytes, ok := rrEncoded.(*EncodedRequestHeader)
	if !ok {
		t.Error("failed to type assert to EncodedRequestHeader")
	}

	assert.Equal(t, *rEncodedBytes, *rrEncodedBytes)
}

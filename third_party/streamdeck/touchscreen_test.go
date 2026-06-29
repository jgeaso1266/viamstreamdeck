package streamdeck

import (
	"bytes"
	"encoding/binary"
	"testing"

	"go.viam.com/test"
)

func TestTouchscreenPacketsSinglePacket(t *testing.T) {
	payload := bytes.Repeat([]byte{0xAB}, 100)
	packets := touchscreenPackets(0, 0, TouchscreenWidth, TouchscreenHeight, payload)

	test.That(t, len(packets), test.ShouldEqual, 1)
	p := packets[0]
	test.That(t, len(p), test.ShouldEqual, touchscreenPacketLen)
	test.That(t, p[0], test.ShouldEqual, byte(0x02))
	test.That(t, p[1], test.ShouldEqual, byte(0x0c))
	test.That(t, binary.LittleEndian.Uint16(p[2:]), test.ShouldEqual, uint16(0))                 // x
	test.That(t, binary.LittleEndian.Uint16(p[4:]), test.ShouldEqual, uint16(0))                 // y
	test.That(t, binary.LittleEndian.Uint16(p[6:]), test.ShouldEqual, uint16(TouchscreenWidth))  // w
	test.That(t, binary.LittleEndian.Uint16(p[8:]), test.ShouldEqual, uint16(TouchscreenHeight)) // h
	test.That(t, p[10], test.ShouldEqual, byte(1))                                               // is-last
	test.That(t, binary.LittleEndian.Uint16(p[11:]), test.ShouldEqual, uint16(0))               // page
	test.That(t, binary.LittleEndian.Uint16(p[13:]), test.ShouldEqual, uint16(100))             // payload len
	test.That(t, p[touchscreenHeaderLen:touchscreenHeaderLen+100], test.ShouldResemble, payload)
}

func TestTouchscreenPacketsChunkingReassembles(t *testing.T) {
	// Payload spanning multiple packets: header bytes must be correct on each and
	// the concatenated payloads must reproduce the original bytes exactly.
	payload := make([]byte, touchscreenPayloadLen*2+250)
	for i := range payload {
		payload[i] = byte(i % 251)
	}

	packets := touchscreenPackets(10, 20, 30, 40, payload)
	test.That(t, len(packets), test.ShouldEqual, 3)

	var reassembled []byte
	for i, p := range packets {
		test.That(t, len(p), test.ShouldEqual, touchscreenPacketLen)
		test.That(t, p[0], test.ShouldEqual, byte(0x02))
		test.That(t, p[1], test.ShouldEqual, byte(0x0c))
		test.That(t, binary.LittleEndian.Uint16(p[2:]), test.ShouldEqual, uint16(10))
		test.That(t, binary.LittleEndian.Uint16(p[4:]), test.ShouldEqual, uint16(20))
		test.That(t, binary.LittleEndian.Uint16(p[6:]), test.ShouldEqual, uint16(30))
		test.That(t, binary.LittleEndian.Uint16(p[8:]), test.ShouldEqual, uint16(40))
		test.That(t, binary.LittleEndian.Uint16(p[11:]), test.ShouldEqual, uint16(i)) // page increments

		isLast := i == len(packets)-1
		if isLast {
			test.That(t, p[10], test.ShouldEqual, byte(1))
		} else {
			test.That(t, p[10], test.ShouldEqual, byte(0))
		}

		chunkLen := int(binary.LittleEndian.Uint16(p[13:]))
		reassembled = append(reassembled, p[touchscreenHeaderLen:touchscreenHeaderLen+chunkLen]...)
	}

	test.That(t, reassembled, test.ShouldResemble, payload)
}

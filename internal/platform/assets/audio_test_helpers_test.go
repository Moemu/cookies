package assets

import (
	"bytes"
	"context"
	"encoding/binary"
)

type fakeAudioProbe struct {
	metadata AudioMetadata
	err      error
}

func (p fakeAudioProbe) Probe(_ context.Context, _ []byte, _ string) (AudioMetadata, error) {
	return p.metadata, p.err
}

func testWAV(sampleRate, durationMS int) []byte {
	sampleCount := sampleRate * durationMS / 1000
	dataSize := sampleCount * 2
	var output bytes.Buffer
	output.WriteString("RIFF")
	_ = binary.Write(&output, binary.LittleEndian, uint32(36+dataSize))
	output.WriteString("WAVEfmt ")
	_ = binary.Write(&output, binary.LittleEndian, uint32(16))
	_ = binary.Write(&output, binary.LittleEndian, uint16(1))
	_ = binary.Write(&output, binary.LittleEndian, uint16(1))
	_ = binary.Write(&output, binary.LittleEndian, uint32(sampleRate))
	_ = binary.Write(&output, binary.LittleEndian, uint32(sampleRate*2))
	_ = binary.Write(&output, binary.LittleEndian, uint16(2))
	_ = binary.Write(&output, binary.LittleEndian, uint16(16))
	output.WriteString("data")
	_ = binary.Write(&output, binary.LittleEndian, uint32(dataSize))
	output.Write(make([]byte, dataSize))
	return output.Bytes()
}

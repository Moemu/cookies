package provider

import (
	"bytes"
	"context"
	"encoding/binary"
	"hash/fnv"
)

// FakeSpeechAdapter produces a deterministic, valid PCM WAV asset for local
// development. Its result is explicitly identified as fixture output and is
// never presented as a real provider synthesis.
type FakeSpeechAdapter struct{}

func (FakeSpeechAdapter) Synthesize(_ context.Context, input SpeechSynthesisInput) (SpeechSynthesisResult, error) {
	if err := input.Validate(); err != nil {
		return SpeechSynthesisResult{}, err
	}
	durationMS := len([]rune(input.Text)) * 80
	if durationMS < 600 {
		durationMS = 600
	}
	sampleRate := 24000
	samples := sampleRate * durationMS / 1000
	dataSize := samples * 2
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
	for index := 0; index < samples; index++ {
		_ = binary.Write(&output, binary.LittleEndian, int16(0))
	}
	hash := fnv.New64a()
	_, _ = hash.Write([]byte(input.Text + "\x00" + input.VoiceAlias))
	return SpeechSynthesisResult{Audio: output.Bytes(), Codec: "wav", SampleRate: sampleRate, DurationMS: durationMS, OriginalText: input.Text, NormalizedText: input.Text,
		WordTimings: []SpeechWordTiming{}, ProviderRequestID: "fixture", ModelAndVoiceSnapshot: "fixture/fake-speech"}, nil
}

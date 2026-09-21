package oggvorbis

import (
	"bytes"
	"os"
	"testing"

	"github.com/jfreymuth/vorbis"
)

func headersOf(t *testing.T, name string) [3][]byte {
	t.Helper()
	data, err := os.ReadFile(name)
	if err != nil {
		t.Fatal(err)
	}
	r := oggReader{source: bytes.NewReader(data)}
	var headers [3][]byte
	for i := range headers {
		p, err := r.NextPacket()
		if err != nil {
			t.Fatal(err)
		}
		headers[i] = p
	}
	return headers
}

// TestNewReaderWithSetupRefusesOtherHeaders: a Setup whose identification
// header differs from the stream's, here in a bitrate field that parses fine,
// does not open the stream.
func TestNewReaderWithSetupRefusesOtherHeaders(t *testing.T) {
	data, err := os.ReadFile("testdata/long.ogg")
	if err != nil {
		t.Fatal(err)
	}
	headers := headersOf(t, "testdata/long.ogg")
	other := append([]byte(nil), headers[0]...)
	other[7+9] ^= 1 // Bitrate.Maximum, low byte
	setup, err := vorbis.ReadSetup(other, headers[1], headers[2])
	if err != nil {
		t.Fatal(err)
	}
	if _, err := NewReaderWithSetup(bytes.NewReader(data), setup); err != ErrSetupMismatch {
		t.Fatalf("opened long.ogg with a Setup from other headers: err = %v", err)
	}
}

// TestSetupIsTheEncoderConfiguration records what the fixtures show: two
// clips from the same encoder at the same settings carry byte-identical
// identification and setup headers, so one Setup opens both. The comment
// header is what differs between them and is not part of the match.
func TestSetupIsTheEncoderConfiguration(t *testing.T) {
	long := headersOf(t, "testdata/long.ogg")
	short := headersOf(t, "testdata/test.ogg")
	if !bytes.Equal(long[0], short[0]) || !bytes.Equal(long[2], short[2]) {
		t.Skip("the fixtures no longer share an encoder configuration")
	}
	setup, err := vorbis.ReadSetup(long[0], long[1], long[2])
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile("testdata/test.ogg")
	if err != nil {
		t.Fatal(err)
	}
	r, err := NewReaderWithSetup(bytes.NewReader(data), setup)
	if err != nil {
		t.Fatalf("long.ogg's Setup does not open test.ogg, whose decoding headers are identical: %v", err)
	}
	if r.Setup() != setup {
		t.Fatal("the Reader did not keep the Setup it was given")
	}
}

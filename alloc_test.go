package oggvorbis_test

import (
	"bytes"
	"io"
	"os"
	"testing"

	"github.com/jfreymuth/oggvorbis"
)

// A streaming consumer calls Read for every voice every few milliseconds and
// SetPosition at every loop wrap, so both must allocate nothing once the
// Reader's buffers have grown to the clip's largest page. These tests pin
// that; they need a vorbis whose DecodeInto allocates nothing as well.

func openAllocClip(t *testing.T) *oggvorbis.Reader {
	t.Helper()
	data, err := os.ReadFile("testdata/test.ogg")
	if err != nil {
		t.Fatal(err)
	}
	r, err := oggvorbis.NewReader(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func TestReadAllocatesNothing(t *testing.T) {
	r := openAllocClip(t)
	block := make([]float32, r.SampleRate()/100*r.Channels())
	readClip := func() {
		if err := r.SetPosition(0); err != nil {
			t.Fatal(err)
		}
		for {
			_, err := r.Read(block)
			if err == io.EOF {
				return
			}
			if err != nil {
				t.Fatal(err)
			}
		}
	}
	readClip() // grows the page buffers to the largest page in the clip
	if allocs := testing.AllocsPerRun(10, readClip); allocs != 0 {
		t.Errorf("reading the clip allocates %v times, expected 0", allocs)
	}
}

func TestSetPositionAllocatesNothing(t *testing.T) {
	r := openAllocClip(t)
	length := r.Length()
	positions := []int64{0, length / 3, length / 2, length - 1, 0}
	seekAll := func() {
		for _, pos := range positions {
			if err := r.SetPosition(pos); err != nil {
				t.Fatal(err)
			}
		}
	}
	seekAll()
	if allocs := testing.AllocsPerRun(10, seekAll); allocs != 0 {
		t.Errorf("seeking allocates %v times, expected 0", allocs)
	}
}

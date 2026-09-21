package oggvorbis_test

import (
	"bytes"
	"io"
	"os"
	"runtime"
	"sync"
	"testing"

	"github.com/jfreymuth/oggvorbis"
)

func readLongT(t *testing.T) []byte {
	t.Helper()
	data, err := os.ReadFile("testdata/long.ogg")
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func readAllFrom(t *testing.T, r *oggvorbis.Reader) []float32 {
	t.Helper()
	var out []float32
	buf := make([]float32, 4096)
	for {
		n, err := r.Read(buf)
		out = append(out, buf[:n]...)
		if err == io.EOF {
			return out
		}
		if err != nil {
			t.Fatal(err)
		}
	}
}

// TestReadersShareSetup: readers opened over one Setup, on their own
// goroutines and seeking independently, decode exactly what a reader that
// parsed its own headers decodes.
func TestReadersShareSetup(t *testing.T) {
	data := readLongT(t)

	own, err := oggvorbis.NewReader(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	want := readAllFrom(t, own)

	setup, err := oggvorbis.ReadSetup(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	if setup.SampleRate() != own.SampleRate() || setup.Channels() != own.Channels() {
		t.Fatalf("ReadSetup reports %d Hz x%d, NewReader %d Hz x%d", setup.SampleRate(), setup.Channels(), own.SampleRate(), own.Channels())
	}

	const readers = 8
	got := make([][]float32, readers)
	var wg sync.WaitGroup
	for i := range got {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			r, err := oggvorbis.NewReaderWithSetup(bytes.NewReader(data), setup)
			if err != nil {
				t.Error(err)
				return
			}
			// Half of them start part way in, as a Voice with an offset does,
			// then rewind, so the shared Setup is also exercised across a seek.
			if i%2 == 1 {
				if err := r.SetPosition(int64(i) * 4410); err != nil {
					t.Error(err)
					return
				}
				readAllFrom(t, r)
				if err := r.SetPosition(0); err != nil {
					t.Error(err)
					return
				}
			}
			got[i] = readAllFrom(t, r)
		}(i)
	}
	wg.Wait()

	for i, samples := range got {
		if len(samples) != len(want) {
			t.Fatalf("reader %d produced %d samples, want %d", i, len(samples), len(want))
		}
		for j := range samples {
			if samples[j] != want[j] {
				t.Fatalf("reader %d differs from the reference at sample %d", i, j)
			}
		}
	}
}

// BenchmarkNewReaderWithSetup is what a Voice on a Clip already loaded pays
// to open: the header check, the per-stream buffers, and on a seekable
// source the scan for the length.
func BenchmarkNewReaderWithSetup(b *testing.B) {
	data := readLong(b)
	setup, err := oggvorbis.ReadSetup(bytes.NewReader(data))
	if err != nil {
		b.Fatal(err)
	}

	b.Run("seekable", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			if _, err := oggvorbis.NewReaderWithSetup(bytes.NewReader(data), setup); err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("unseekable", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			if _, err := oggvorbis.NewReaderWithSetup(reader{bytes.NewReader(data)}, setup); err != nil {
				b.Fatal(err)
			}
		}
	})
}

// BenchmarkReadSetup is the once-per-Clip cost that remains.
func BenchmarkReadSetup(b *testing.B) {
	data := readLong(b)
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if _, err := oggvorbis.ReadSetup(bytes.NewReader(data)); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkReaderLiveWithSetup is BenchmarkReaderLive for Readers that share
// one Setup: the heap each keeps while idle, as live-B/reader, with the Setup
// itself held outside the batch and so not counted.
func BenchmarkReaderLiveWithSetup(b *testing.B) {
	data := readLong(b)
	setup, err := oggvorbis.ReadSetup(bytes.NewReader(data))
	if err != nil {
		b.Fatal(err)
	}
	const batch = 64
	var live float64
	for i := 0; i < b.N; i++ {
		readers := make([]*oggvorbis.Reader, batch)
		var before, after runtime.MemStats
		runtime.GC()
		runtime.ReadMemStats(&before)
		for j := range readers {
			readers[j], err = oggvorbis.NewReaderWithSetup(bytes.NewReader(data), setup)
			if err != nil {
				b.Fatal(err)
			}
		}
		runtime.GC()
		runtime.ReadMemStats(&after)
		live = float64(after.HeapAlloc-before.HeapAlloc) / batch
		runtime.KeepAlive(readers)
	}
	b.ReportMetric(live, "live-B/reader")
}

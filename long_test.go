package oggvorbis_test

import (
	"bytes"
	"io"
	"os"
	"runtime"
	"testing"
	"time"

	"github.com/jfreymuth/oggvorbis"
)

// These benchmarks run against testdata/long.ogg, a 20 s mono 44.1 kHz clip,
// and separate what opening a stream costs from what decoding it costs.

func readLong(b *testing.B) []byte {
	b.Helper()
	data, err := os.ReadFile("testdata/long.ogg")
	if err != nil {
		b.Fatal(err)
	}
	return data
}

func openLong(b *testing.B, data []byte) *oggvorbis.Reader {
	b.Helper()
	r, err := oggvorbis.NewReader(bytes.NewReader(data))
	if err != nil {
		b.Fatal(err)
	}
	return r
}

// BenchmarkNewReader measures opening a stream: the headers, the decoder's
// setup and, on a seekable source, the scan for the stream's length.
func BenchmarkNewReader(b *testing.B) {
	data := readLong(b)

	b.Run("seekable", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			if _, err := oggvorbis.NewReader(bytes.NewReader(data)); err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("unseekable", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			if _, err := oggvorbis.NewReader(reader{bytes.NewReader(data)}); err != nil {
				b.Fatal(err)
			}
		}
	})
}

// BenchmarkReaderLive reports the heap an open Reader keeps while idle, as
// live-B/reader: the difference in heap after opening a batch of readers and
// collecting garbage, divided by the batch.
func BenchmarkReaderLive(b *testing.B) {
	data := readLong(b)
	const batch = 64
	var live float64
	for i := 0; i < b.N; i++ {
		readers := make([]*oggvorbis.Reader, batch)
		var before, after runtime.MemStats
		runtime.GC()
		runtime.ReadMemStats(&before)
		for j := range readers {
			readers[j] = openLong(b, data)
		}
		runtime.GC()
		runtime.ReadMemStats(&after)
		live = float64(after.HeapAlloc-before.HeapAlloc) / batch
		runtime.KeepAlive(readers)
	}
	b.ReportMetric(live, "live-B/reader")
}

// BenchmarkSetPosition seeks an open Reader back to the start, which is what a
// looping stream pays at every wrap.
func BenchmarkSetPosition(b *testing.B) {
	r := openLong(b, readLong(b))
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := r.SetPosition(0); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkRead measures decoding: one 10 ms block per iteration the way a
// streaming consumer reads, and the whole clip per iteration.
func BenchmarkRead(b *testing.B) {
	data := readLong(b)

	b.Run("block10ms", func(b *testing.B) {
		r := openLong(b, data)
		block := make([]float32, r.SampleRate()/100*r.Channels())
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, err := r.Read(block)
			if err == io.EOF {
				b.StopTimer()
				if err := r.SetPosition(0); err != nil {
					b.Fatal(err)
				}
				b.StartTimer()
				continue
			}
			if err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("clip", func(b *testing.B) {
		r := openLong(b, data)
		buffer := make([]float32, 4096*r.Channels())
		b.ReportAllocs()
		b.ResetTimer()
		var elapsed time.Duration
		samples := 0
		for i := 0; i < b.N; i++ {
			b.StopTimer()
			if err := r.SetPosition(0); err != nil {
				b.Fatal(err)
			}
			b.StartTimer()
			start := time.Now()
			samples = 0
			for {
				n, err := r.Read(buffer)
				samples += n / r.Channels()
				if err == io.EOF {
					break
				}
				if err != nil {
					b.Fatal(err)
				}
			}
			elapsed += time.Since(start)
		}
		b.StopTimer()
		audio := float64(samples) / float64(r.SampleRate())
		b.ReportMetric(audio*float64(b.N)/elapsed.Seconds(), "x-realtime")
	})
}

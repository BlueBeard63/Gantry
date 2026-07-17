package gantrytest

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"os"
	"sort"
	"sync"
)

// recorder accumulates the webview's screencast frames (JPEG stills
// with timestamps, delivered by CDP on every visual change) and muxes
// them into screencast.avi at teardown. MJPEG-in-AVI on purpose: the
// frames go into the container as-is - no video encoder, no cgo, no
// bundled ffmpeg - and every player handles it. Re-encoding to WebM
// stays an on-demand nicety (tier 3's trace viewer).
type recorder struct {
	mu     sync.Mutex
	frames []screencastFrame
}

type screencastFrame struct {
	data []byte  // one JPEG
	t    float64 // capture time, seconds
}

// add is the CDP screencast callback. Frames arrive on the CDP read
// loop; keep this cheap.
func (r *recorder) add(data []byte, t float64) {
	r.mu.Lock()
	r.frames = append(r.frames, screencastFrame{data: data, t: t})
	r.mu.Unlock()
}

// A test can flash through its UI states in well under a second, and a
// real-time video of that is unwatchable. The timeline is paced for a
// human instead: every captured state stays on screen at least
// minShowSeconds, and idle stretches are compressed to maxShowSeconds.
// Ordering is exact; only the pacing is editorial.
const (
	minShowSeconds = 0.5
	maxShowSeconds = 2.0
)

// writeAVI lays the captured frames on a paced fixed-fps timeline and
// writes the AVI. Reports whether a file was written (no frames, no
// file).
func (r *recorder) writeAVI(path string, fps int) (bool, error) {
	r.mu.Lock()
	frames := append([]screencastFrame(nil), r.frames...)
	r.mu.Unlock()
	if len(frames) == 0 {
		return false, nil
	}
	// Screencast frames and action-forced captures interleave; order by
	// capture time before building the timeline.
	sort.SliceStable(frames, func(i, j int) bool { return frames[i].t < frames[j].t })

	// Merge byte-identical neighbors (screencast bursts, an action snap
	// of an unchanged screen) so the pacing below stretches distinct
	// states, not duplicates.
	dedup := frames[:1]
	for _, f := range frames[1:] {
		if !bytes.Equal(f.data, dedup[len(dedup)-1].data) {
			dedup = append(dedup, f)
		}
	}
	frames = dedup

	w, h, err := jpegSize(frames[0].data)
	if err != nil {
		return false, fmt.Errorf("reading the first frame's dimensions: %w", err)
	}

	var timeline [][]byte
	for i, f := range frames {
		show := minShowSeconds
		if i+1 < len(frames) {
			if dt := frames[i+1].t - f.t; dt > show {
				show = dt
			}
		}
		if show > maxShowSeconds {
			show = maxShowSeconds
		}
		repeats := int(show*float64(fps) + 0.5)
		if repeats < 1 {
			repeats = 1
		}
		for n := 0; n < repeats; n++ {
			timeline = append(timeline, f.data)
		}
	}

	if err := os.WriteFile(path, muxAVI(timeline, w, h, fps), 0o644); err != nil {
		return false, err
	}
	return true, nil
}

// muxAVI builds a complete MJPEG AVI in memory: RIFF header, one video
// stream, the movi frame chunks, and the idx1 index players seek with.
func muxAVI(frames [][]byte, width, height, fps int) []byte {
	le := binary.LittleEndian
	u32 := func(b *bytes.Buffer, v uint32) { _ = binary.Write(b, le, v) }

	// movi body and index first, so the header knows every size.
	var movi bytes.Buffer
	var idx bytes.Buffer
	movi.WriteString("movi")
	maxFrame := 0
	for _, f := range frames {
		offset := movi.Len()
		movi.WriteString("00dc")
		u32(&movi, uint32(len(f)))
		movi.Write(f)
		if len(f)%2 == 1 {
			movi.WriteByte(0) // chunks are word-aligned
		}
		if len(f) > maxFrame {
			maxFrame = len(f)
		}
		idx.WriteString("00dc")
		u32(&idx, 0x10) // AVIIF_KEYFRAME - every MJPEG frame stands alone
		u32(&idx, uint32(offset))
		u32(&idx, uint32(len(f)))
	}

	list := func(kind string, body []byte) []byte {
		var b bytes.Buffer
		b.WriteString("LIST")
		u32(&b, uint32(4+len(body)))
		b.WriteString(kind)
		b.Write(body)
		return b.Bytes()
	}
	chunk := func(kind string, body []byte) []byte {
		var b bytes.Buffer
		b.WriteString(kind)
		u32(&b, uint32(len(body)))
		b.Write(body)
		return b.Bytes()
	}

	// avih - the main header.
	var avih bytes.Buffer
	u32(&avih, uint32(1_000_000/fps)) // dwMicroSecPerFrame
	u32(&avih, uint32(maxFrame*fps))  // dwMaxBytesPerSec
	u32(&avih, 0)                     // dwPaddingGranularity
	u32(&avih, 0x10)                  // dwFlags: AVIF_HASINDEX
	u32(&avih, uint32(len(frames)))   // dwTotalFrames
	u32(&avih, 0)                     // dwInitialFrames
	u32(&avih, 1)                     // dwStreams
	u32(&avih, uint32(maxFrame))      // dwSuggestedBufferSize
	u32(&avih, uint32(width))
	u32(&avih, uint32(height))
	avih.Write(make([]byte, 16)) // dwReserved

	// strh - the video stream header.
	var strh bytes.Buffer
	strh.WriteString("vids")
	strh.WriteString("MJPG")
	u32(&strh, 0)                   // dwFlags
	u32(&strh, 0)                   // wPriority + wLanguage
	u32(&strh, 0)                   // dwInitialFrames
	u32(&strh, 1)                   // dwScale
	u32(&strh, uint32(fps))         // dwRate
	u32(&strh, 0)                   // dwStart
	u32(&strh, uint32(len(frames))) // dwLength
	u32(&strh, uint32(maxFrame))    // dwSuggestedBufferSize
	u32(&strh, 0xFFFFFFFF)          // dwQuality: default
	u32(&strh, 0)                   // dwSampleSize
	_ = binary.Write(&strh, le, [4]int16{0, 0, int16(width), int16(height)})

	// strf - BITMAPINFOHEADER.
	var strf bytes.Buffer
	u32(&strf, 40) // biSize
	u32(&strf, uint32(width))
	u32(&strf, uint32(height))
	_ = binary.Write(&strf, le, uint16(1))  // biPlanes
	_ = binary.Write(&strf, le, uint16(24)) // biBitCount
	strf.WriteString("MJPG")                // biCompression
	u32(&strf, uint32(width*height*3))      // biSizeImage
	u32(&strf, 0)                           // biXPelsPerMeter
	u32(&strf, 0)                           // biYPelsPerMeter
	u32(&strf, 0)                           // biClrUsed
	u32(&strf, 0)                           // biClrImportant

	strl := list("strl", append(chunk("strh", strh.Bytes()), chunk("strf", strf.Bytes())...))
	hdrl := list("hdrl", append(chunk("avih", avih.Bytes()), strl...))

	var body bytes.Buffer
	body.WriteString("AVI ")
	body.Write(hdrl)
	body.Write(list("movi", movi.Bytes()[4:])) // movi.Bytes() already starts with "movi"
	body.Write(chunk("idx1", idx.Bytes()))

	var out bytes.Buffer
	out.WriteString("RIFF")
	u32(&out, uint32(body.Len()))
	out.Write(body.Bytes())
	return out.Bytes()
}

// jpegSize reads a JPEG's pixel dimensions from its SOF marker.
func jpegSize(data []byte) (w, h int, err error) {
	if len(data) < 4 || data[0] != 0xFF || data[1] != 0xD8 {
		return 0, 0, fmt.Errorf("not a JPEG")
	}
	i := 2
	for i+9 < len(data) {
		if data[i] != 0xFF {
			i++
			continue
		}
		marker := data[i+1]
		// SOF0..SOF15 except the tables (DHT/JPGA/DAC) carry dimensions.
		if marker >= 0xC0 && marker <= 0xCF && marker != 0xC4 && marker != 0xC8 && marker != 0xCC {
			h = int(data[i+5])<<8 | int(data[i+6])
			w = int(data[i+7])<<8 | int(data[i+8])
			return w, h, nil
		}
		if marker == 0xD8 || (marker >= 0xD0 && marker <= 0xD9) {
			i += 2
			continue
		}
		length := int(data[i+2])<<8 | int(data[i+3])
		i += 2 + length
	}
	return 0, 0, fmt.Errorf("no SOF marker found")
}

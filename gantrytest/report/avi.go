package report

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
)

// DecodeAVI reads the distinct frames back out of a screencast.avi
// (MJPEG-in-AVI, as written by gantrytest's recorder). The AVI lays
// frames on a fixed-fps timeline with repeats padding each state; this
// walks the movi chunks, collapses byte-identical neighbours back into
// distinct states, and reconstructs each state's appearance time from
// its frame index and the header fps. The result is exactly what the
// canvas player needs: distinct JPEGs with timestamps, far smaller than
// the padded stream.
//
// It is intentionally tolerant: a malformed or truncated file yields
// whatever frames were readable, never an error, so a half-written
// screencast still shows something.
func DecodeAVI(data []byte) []Frame {
	fps := aviFPS(data)
	if fps <= 0 {
		fps = 12 // the recorder's default
	}
	movi := findList(data, "movi")
	if movi == nil {
		return nil
	}

	var out []Frame
	var prev []byte
	frameIdx := 0
	i := 0
	for i+8 <= len(movi) {
		id := string(movi[i : i+4])
		size := int(binary.LittleEndian.Uint32(movi[i+4 : i+8]))
		if size < 0 || i+8+size > len(movi) {
			break
		}
		body := movi[i+8 : i+8+size]
		// "00dc" is the compressed video chunk; ignore any other stream.
		if id == "00dc" && len(body) > 0 {
			if !bytes.Equal(body, prev) {
				out = append(out, Frame{
					Data: jpegDataURI(body),
					T:    float64(frameIdx) / float64(fps),
				})
				prev = append(prev[:0], body...)
			}
			frameIdx++
		}
		// Chunks are word-aligned.
		adv := 8 + size
		if size%2 == 1 {
			adv++
		}
		i += adv
	}
	return out
}

// aviFPS recovers frames-per-second from the avih dwMicroSecPerFrame
// field, so DecodeAVI's timeline matches however the recorder paced it.
func aviFPS(data []byte) int {
	avih := findChunk(data, "avih")
	if len(avih) < 4 {
		return 0
	}
	microPerFrame := binary.LittleEndian.Uint32(avih[:4])
	if microPerFrame == 0 {
		return 0
	}
	return int(1_000_000 / microPerFrame)
}

// findList returns the body of the first LIST of the given kind
// (e.g. "movi"), excluding the 4-byte kind tag.
func findList(data []byte, kind string) []byte {
	for i := 0; i+12 <= len(data); {
		tag := string(data[i : i+4])
		size := int(binary.LittleEndian.Uint32(data[i+4 : i+8]))
		if size < 0 || i+8+size > len(data) {
			return nil
		}
		if tag == "RIFF" || tag == "LIST" {
			listKind := string(data[i+8 : i+12])
			body := data[i+12 : i+8+size]
			if tag == "LIST" && listKind == kind {
				return body
			}
			// Descend into RIFF (and other LISTs) looking for the kind.
			if r := findList(body, kind); r != nil {
				return r
			}
		}
		adv := 8 + size
		if size%2 == 1 {
			adv++
		}
		i += adv
	}
	return nil
}

// findChunk returns the body of the first non-LIST chunk of the given
// id (e.g. "avih"), searching recursively through LIST containers.
func findChunk(data []byte, id string) []byte {
	for i := 0; i+8 <= len(data); {
		tag := string(data[i : i+4])
		size := int(binary.LittleEndian.Uint32(data[i+4 : i+8]))
		if size < 0 || i+8+size > len(data) {
			return nil
		}
		body := data[i+8 : i+8+size]
		switch tag {
		case "RIFF", "LIST":
			if len(body) >= 4 {
				if r := findChunk(body[4:], id); r != nil {
					return r
				}
			}
		case id:
			return body
		}
		adv := 8 + size
		if size%2 == 1 {
			adv++
		}
		i += adv
	}
	return nil
}

func jpegDataURI(jpeg []byte) string {
	return "data:image/jpeg;base64," + base64.StdEncoding.EncodeToString(jpeg)
}

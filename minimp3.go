package minimp3

/*
#define MINIMP3_IMPLEMENTATION
#include "minimp3.h"
*/
import "C"

import (
	"errors"
	"io"
	"unsafe"
)

const maxSamplesPerFrame = 1152 * 2

// Decoder decode the mp3 stream by minimp3
type Decoder struct {
	source               io.Reader
	mp3, pcm             []byte
	mp3Length, pcmLength int
	lastError            error
	samples              C.int
	decode               C.mp3dec_t
	info                 C.mp3dec_frame_info_t
}

func NewDecoder(source io.Reader) *Decoder {
	d := &Decoder{
		source: source,
		mp3:    make([]byte, 1024*16),
		pcm:    make([]byte, maxSamplesPerFrame*2),
	}

	C.mp3dec_init(&d.decode)
	return d
}

func (d *Decoder) Read(p []byte) (int, error) {
	var n, n2, n3 int
	for {
		n3 = copy(p[n:], d.pcm[:d.pcmLength])
		n += n3

		if n3 < d.pcmLength {
			// The p is full
			d.pcmLength = copy(d.pcm, d.pcm[n3:d.pcmLength])
			return n, nil
		}

		if d.lastError == nil {
			n2, d.lastError = d.source.Read(d.mp3[d.mp3Length:])
			d.mp3Length += n2
		}

		d.samples = C.mp3dec_decode_frame(&d.decode,
			(*C.uint8_t)(unsafe.Pointer(&d.mp3[0])), C.int(d.mp3Length),
			(*C.mp3d_sample_t)(unsafe.Pointer(&d.pcm[0])), &d.info,
		)

		if d.info.frame_bytes == 0 {
			return n, d.lastError
		}

		d.mp3Length = copy(d.mp3, d.mp3[d.info.frame_bytes:d.mp3Length])
		d.pcmLength = int(d.samples*d.info.channels) * 2
	}
}

func (d *Decoder) Seek(offset int64, whence int) (int64, error) {
	seeker, ok := d.source.(io.Seeker)
	if !ok {
		panic("minimp3: d.source is not seeker")
	}

	bytesPerFrame := int64(d.samples * d.info.channels * 2)
	if bytesPerFrame == 0 {
		return 0, errors.New("no frame available")
	}

	mp3Offset := offset / bytesPerFrame * int64(d.info.frame_bytes)
	if whence == io.SeekCurrent {
		mp3Offset -= int64(d.mp3Length)
	}

	mp3Pos, err := seeker.Seek(mp3Offset, whence)
	if err != nil {
		return 0, err
	}

	d.Reset()
	pcmPos := mp3Pos / int64(d.info.frame_bytes) * bytesPerFrame
	return pcmPos, nil
}

func (d *Decoder) Info() (SampleRate, Channels, Kbps, Layer int) {
	SampleRate = int(d.info.hz)
	Channels = int(d.info.channels)
	Kbps = int(d.info.bitrate_kbps)
	Layer = int(d.info.layer)
	return
}

func (d *Decoder) Reset() {
	d.mp3Length = 0
	d.pcmLength = 0
}

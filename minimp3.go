package minimp3

/*
#define MINIMP3_IMPLEMENTATION
#include "minimp3.h"
*/
import "C"

import (
	"io"
	"sync"
	"unsafe"
)

const maxSamplesPerFrame = 1152 * 2

// Decoder decode the mp3 stream by minimp3
type Decoder struct {
	sync.Mutex
	source               io.ReadCloser
	mp3, pcm             []byte
	mp3Length, pcmLength int
	lastError            error
	decode               C.mp3dec_t
	info                 C.mp3dec_frame_info_t
	SampleRate           int
	Channels             int
	Kbps                 int
	Layer                int
}

func NewDecoder(source io.ReadCloser) (*Decoder, error) {
	d := &Decoder{
		source: source,
		mp3:    make([]byte, 1024*16),
		pcm:    make([]byte, maxSamplesPerFrame*2),
	}

	C.mp3dec_init(&d.decode)
	_, err := d.Read([]byte{})
	return d, err
}

func (d *Decoder) Read(p []byte) (int, error) {
	d.Lock()
	defer d.Unlock()

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

		samples := C.mp3dec_decode_frame(&d.decode,
			(*C.uint8_t)(unsafe.Pointer(&d.mp3[0])), C.int(d.mp3Length),
			(*C.mp3d_sample_t)(unsafe.Pointer(&d.pcm[0])), &d.info,
		)

		if d.info.frame_bytes == 0 {
			return n, d.lastError
		}

		d.mp3Length = copy(d.mp3, d.mp3[d.info.frame_bytes:d.mp3Length])
		d.pcmLength = int(samples*d.info.channels) * 2

		d.SampleRate = int(d.info.hz)
		d.Channels = int(d.info.channels)
		d.Kbps = int(d.info.bitrate_kbps)
		d.Layer = int(d.info.layer)
	}
}

func (d *Decoder) Close() error {
	d.Lock()
	defer d.Unlock()
	d.mp3Length = 0
	d.pcmLength = 0
	return d.source.Close()
}

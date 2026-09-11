package reddit

import (
	"compress/flate"
	"compress/gzip"
	"io"
	"strings"

	"github.com/andybalholm/brotli"
	http "github.com/bogdanfinn/fhttp"
	"github.com/klauspost/compress/zstd"
)

const (
	encodingGzip    = "gzip"
	encodingDeflate = "deflate"
	encodingBrotli  = "br"
	encodingZstd    = "zstd"
)

func readBody(resp *http.Response) ([]byte, error) {
	if resp.Uncompressed {
		return io.ReadAll(resp.Body)
	}

	r, err := decoder(resp)
	if err != nil {
		return nil, err
	}

	return io.ReadAll(r)
}

func decoder(resp *http.Response) (io.Reader, error) {
	switch strings.ToLower(resp.Header.Get("Content-Encoding")) {
	case encodingGzip:
		return gzip.NewReader(resp.Body)
	case encodingDeflate:
		return flate.NewReader(resp.Body), nil
	case encodingBrotli:
		return brotli.NewReader(resp.Body), nil
	case encodingZstd:
		return zstd.NewReader(resp.Body)
	default:
		return resp.Body, nil
	}
}

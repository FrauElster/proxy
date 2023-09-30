package proxy

import (
	"bytes"
	"compress/flate"
	"compress/gzip"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/andybalholm/brotli"
)

type SupportedCompression string

const (
	Gzip    SupportedCompression = "gzip"
	Deflate SupportedCompression = "deflate"
	Brotli  SupportedCompression = "br"
)

func compressionFromString(encoding string) SupportedCompression {
	if strings.Contains(encoding, string(Gzip)) {
		return Gzip
	}
	if strings.Contains(encoding, string(Deflate)) {
		return Deflate
	}
	if strings.Contains(encoding, string(Brotli)) {
		return Brotli
	}
	return ""
}

func decompressResponse(res *http.Response) (err error) {
	if res.Header.Get("Content-Encoding") == "" {
		return nil
	}

	// get correct reader
	var reader io.Reader
	encoding := compressionFromString(res.Header.Get("Content-Encoding"))
	switch encoding {
	case Gzip:
		// Decompress the response body
		gzipReader, err := gzip.NewReader(res.Body)
		if err != nil {
			return err
		}
		defer gzipReader.Close()
		reader = gzipReader
	case Deflate:
		flateReader := flate.NewReader(res.Body)
		defer flateReader.Close()
		reader = flateReader
	case Brotli:
		reader = brotli.NewReader(res.Body)
	default:
		return fmt.Errorf("unknown compression type: %s", res.Header.Get("Content-Encoding"))
	}

	var decompressedBody bytes.Buffer
	_, err = io.Copy(&decompressedBody, reader)
	if err != nil {
		return err
	}

	// Replace the response body with the decompressed data
	res.Body = io.NopCloser(&decompressedBody)
	res.Header.Del("Content-Encoding")
	res.ContentLength = int64(decompressedBody.Len())

	return nil
}

func compressBody(body []byte, encoding SupportedCompression) ([]byte, error) {
	var writer io.WriteCloser
	var compressedBodyBuffer bytes.Buffer
	switch encoding {
	case Gzip:
		writer = gzip.NewWriter(&compressedBodyBuffer)
	case Deflate:
		flateWriter, err := flate.NewWriter(&compressedBodyBuffer, flate.BestCompression)
		if err != nil {
			return nil, err
		}
		writer = flateWriter
	case Brotli:
		writer = brotli.NewWriter(&compressedBodyBuffer)
	default:
		return nil, fmt.Errorf("unknown compression type: %s", encoding)
	}

	_, err := writer.Write(body)
	if err != nil {
		return nil, err
	}
	err = writer.Close()
	if err != nil {
		return nil, err
	}
	return compressedBodyBuffer.Bytes(), nil
}

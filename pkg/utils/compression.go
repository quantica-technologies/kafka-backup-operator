package utils

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
)

// Compress compresses data using gzip
func Compress(data []byte) ([]byte, error) {
	var buf bytes.Buffer
	writer := gzip.NewWriter(&buf)

	if _, err := writer.Write(data); err != nil {
		return nil, fmt.Errorf("failed to write compressed data: %w", err)
	}

	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("failed to close gzip writer: %w", err)
	}

	return buf.Bytes(), nil
}

// Decompress decompresses gzip data
func Decompress(data []byte) ([]byte, error) {
	reader, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("failed to create gzip reader: %w", err)
	}
	defer reader.Close()

	result, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("failed to read decompressed data: %w", err)
	}

	return result, nil
}

// CompressReader wraps a reader with gzip compression
func CompressReader(r io.Reader) io.ReadCloser {
	pr, pw := io.Pipe()

	go func() {
		gw := gzip.NewWriter(pw)
		_, err := io.Copy(gw, r)
		gw.Close()
		if err != nil {
			pw.CloseWithError(err)
		} else {
			pw.Close()
		}
	}()

	return pr
}

// DecompressReader wraps a reader with gzip decompression
func DecompressReader(r io.Reader) (io.ReadCloser, error) {
	return gzip.NewReader(r)
}

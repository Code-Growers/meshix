package handlers

import (
	"fmt"
	"io"
	"strings"

	"github.com/andybalholm/brotli"
	"github.com/dsnet/compress/bzip2"
	"github.com/klauspost/compress/zstd"
	"github.com/pierrec/lz4/v4"
	"github.com/sorairolake/lzip-go"
	"github.com/ulikunitz/xz"
)

var supportedCompressions = []string{
	"br", "bz2", "lzip", "lz4", "zst", "xz",
}

func isCompressionSupported(compressionType string) bool {
	return contains(supportedCompressions, compressionType)
}

func NewCompressionWriter(compressionType string, w io.Writer) (io.WriteCloser, error) {
	compressionType = strings.TrimSpace(compressionType)
	if !contains(supportedCompressions, compressionType) {
		return nil, fmt.Errorf("Unsupported compression: %s", compressionType)
	}

	switch compressionType {
	case "br":
		return brotli.NewWriter(w), nil
	case "bz2":
		return bzip2.NewWriter(w, nil)
	case "lzip":
		return lzip.NewWriter(w), nil
	case "lz4":
		return lz4.NewWriter(w), nil
	case "zst":
		return zstd.NewWriter(w)
	case "xz":
		return xz.NewWriter(w)
	}

	return nil, fmt.Errorf("Unsupported compression: %s", compressionType)
}

func NewCompressionReader(compressionType string, r io.Reader) (io.Reader, error) {
	compressionType = strings.TrimSpace(compressionType)
	if !contains(supportedCompressions, compressionType) {
		return nil, fmt.Errorf("Unsupported compression: %s", compressionType)
	}

	switch compressionType {
	case "br":
		return brotli.NewReader(r), nil
	case "bz2":
		return bzip2.NewReader(r, nil)
	case "lzip":
		return lzip.NewReader(r)
	case "lz4":
		return lz4.NewReader(r), nil
	case "zst":
		return zstd.NewReader(r)
	case "xz":
		return xz.NewReader(r)
	}

	return nil, fmt.Errorf("Unsupported compression: %s", compressionType)
}

func contains(slice []string, value string) bool {
	for _, v := range slice {
		if v == value {
			return true
		}
	}
	return false
}

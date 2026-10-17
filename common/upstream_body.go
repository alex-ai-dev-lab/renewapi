package common

import (
	"errors"
	"io"
)

var ErrUpstreamBodyTooLarge = errors.New("upstream response exceeds size limit")

// ReadUpstreamBody bounds decompressed bytes independently of Content-Length.
func ReadUpstreamBody(reader io.Reader, limit int64) ([]byte, error) {
	if limit <= 0 {
		return nil, ErrUpstreamBodyTooLarge
	}
	body, err := io.ReadAll(io.LimitReader(reader, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(body)) > limit {
		return nil, ErrUpstreamBodyTooLarge
	}
	return body, nil
}

func UpstreamResponseBodyLimit() int64 {
	megabytes := GetEnvOrDefault("UPSTREAM_RESPONSE_MAX_MB", 64)
	if megabytes <= 0 || int64(megabytes) > (1<<43)-1 {
		megabytes = 64
	}
	return int64(megabytes) << 20
}

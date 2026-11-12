package artifacts

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"
)

const MaxGeographicBytes int64 = 1 << 30

// GeographicStore uses fresh reservation keys and never exposes a bucket URL.
// Upload accepts a seekable spool so the signed digest can be verified before
// sending bytes. A caller must retain the reservation on uncertain writes.
type GeographicStore interface {
	PutGeographic(ctx context.Context, key string, body io.ReadSeeker, size int64, digest, media string) error
	OpenGeographic(ctx context.Context, key string, size int64, part *ByteRange) (io.ReadCloser, error)
	Delete(ctx context.Context, key string) error
}

type ByteRange struct{ Start, End int64 } // inclusive

var geographicKey = regexp.MustCompile(`^emad/geographic/v1/[0-9a-f]{64}/geoasset_[0-9a-f]{64}/[0-9a-f]{64}$`)

func (s *S3) geographicClient() *http.Client {
	c := *s.client
	c.Timeout = 30 * time.Minute
	return &c
}

func (s *S3) PutGeographic(ctx context.Context, key string, body io.ReadSeeker, size int64, digest, media string) error {
	if !geographicKey.MatchString(key) || !strings.HasSuffix(key, "/"+digest) || size < 1 || size > MaxGeographicBytes {
		return errors.New("invalid geographic object identity")
	}
	if _, err := body.Seek(0, io.SeekStart); err != nil {
		return err
	}
	if err := verifyStream(ctx, body, size, digest); err != nil {
		return err
	}
	if _, err := body.Seek(0, io.SeekStart); err != nil {
		return err
	}
	// Reconcile a prior successful upload even when the response was lost.
	if prior, err := s.openGeographic(ctx, key, size, nil); err == nil {
		defer prior.Close()
		return verifyStream(ctx, prior, size, digest)
	} else if !errors.Is(err, errGeographicMissing) {
		return err
	}
	req, err := s.signedRequest(ctx, http.MethodPut, key, io.LimitReader(body, size), size, digest)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", media)
	req.Header.Set("If-None-Match", "*")
	resp, err := s.geographicClient().Do(req)
	if err != nil {
		return errors.New("geographic write outcome unknown")
	}
	resp.Body.Close()
	if resp.StatusCode/100 != 2 && resp.StatusCode != http.StatusPreconditionFailed {
		return fmt.Errorf("geographic write status %d", resp.StatusCode)
	}
	remote, err := s.openGeographic(ctx, key, size, nil)
	if err != nil {
		return err
	}
	defer remote.Close()
	return verifyStream(ctx, remote, size, digest)
}

type contextReader struct {
	ctx context.Context
	io.Reader
}

func (r contextReader) Read(p []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.Reader.Read(p)
}

func verifyStream(ctx context.Context, r io.Reader, size int64, digest string) error {
	h := sha256.New()
	n, err := io.CopyBuffer(h, io.LimitReader(contextReader{ctx, r}, size+1), make([]byte, 256<<10))
	if err != nil {
		return err
	}
	if n != size || hex.EncodeToString(h.Sum(nil)) != digest {
		return errors.New("geographic object integrity mismatch")
	}
	return nil
}

var errGeographicMissing = errors.New("geographic object missing")

func (s *S3) OpenGeographic(ctx context.Context, key string, size int64, part *ByteRange) (io.ReadCloser, error) {
	return s.openGeographic(ctx, key, size, part)
}

func (s *S3) openGeographic(ctx context.Context, key string, size int64, part *ByteRange) (io.ReadCloser, error) {
	if !geographicKey.MatchString(key) || size < 1 || size > MaxGeographicBytes {
		return nil, errors.New("invalid geographic object identity")
	}
	req, err := s.request(ctx, http.MethodGet, key, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept-Encoding", "identity")
	expected, status := size, http.StatusOK
	if part != nil {
		if part.Start < 0 || part.End < part.Start || part.End >= size {
			return nil, errors.New("invalid geographic byte range")
		}
		req.Header.Set("Range", fmt.Sprintf("bytes=%d-%d", part.Start, part.End))
		expected, status = part.End-part.Start+1, http.StatusPartialContent
	}
	r, err := s.geographicClient().Do(req)
	if err != nil {
		return nil, errors.New("geographic read unavailable")
	}
	if r.StatusCode == http.StatusNotFound {
		r.Body.Close()
		return nil, errGeographicMissing
	}
	if r.StatusCode != status || r.ContentLength != expected || part != nil && r.Header.Get("Content-Range") != fmt.Sprintf("bytes %d-%d/%d", part.Start, part.End, size) {
		r.Body.Close()
		return nil, errors.New("geographic store returned unexpected response")
	}
	return r.Body, nil
}

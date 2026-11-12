// Package artifacts stores immutable, scoped blobs behind authenticated APIs.
package artifacts

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

const MaxBytes = 16 * 1024 * 1024

func Hash(b []byte) string { h := sha256.Sum256(b); return hex.EncodeToString(h[:]) }

type Store interface {
	Put(context.Context, string, []byte, string) error
	Get(context.Context, string, int64) ([]byte, error)
	Delete(context.Context, string) error
}
type Unavailable struct{}

func (Unavailable) Put(context.Context, string, []byte, string) error {
	return errors.New("artifact storage unavailable")
}
func (Unavailable) Get(context.Context, string, int64) ([]byte, error) {
	return nil, errors.New("artifact storage unavailable")
}
func (Unavailable) Delete(context.Context, string) error {
	return errors.New("artifact storage unavailable")
}

// S3 uses a trusted, path-style endpoint. Neither object keys nor provider URLs
// are accepted from the browser. Credentials never appear in returned metadata.
type S3 struct {
	endpoint, bucket, region, key, secret string
	client                                *http.Client
}
type S3Config struct {
	Endpoint, Bucket, Region, AccessKey, SecretKey string
	AllowHTTP                                      bool
}

var bucketName = regexp.MustCompile(`^[a-z0-9][a-z0-9.-]{1,61}[a-z0-9]$`)

func NewS3(c S3Config) (*S3, error) {
	u, e := url.Parse(c.Endpoint)
	if e != nil || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || (u.Path != "" && u.Path != "/") || (u.Scheme != "https" && !(u.Scheme == "http" && c.AllowHTTP)) || !bucketName.MatchString(c.Bucket) || c.AccessKey == "" || c.SecretKey == "" {
		return nil, errors.New("invalid trusted S3 configuration")
	}
	if c.Region == "" {
		c.Region = "us-east-1"
	}
	return &S3{strings.TrimRight(c.Endpoint, "/"), c.Bucket, c.Region, c.AccessKey, c.SecretKey, &http.Client{Timeout: 30 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return errors.New("S3 redirect denied") }}}, nil
}
func mac(key []byte, s string) []byte {
	h := hmac.New(sha256.New, key)
	_, _ = h.Write([]byte(s))
	return h.Sum(nil)
}
func (s *S3) request(ctx context.Context, method, key string, body []byte) (*http.Request, error) {
	return s.signedRequest(ctx, method, key, bytes.NewReader(body), int64(len(body)), Hash(body))
}
func (s *S3) signedRequest(ctx context.Context, method, key string, body io.Reader, size int64, payload string) (*http.Request, error) {
	if (key == "" && method != "PUT") || strings.ContainsAny(key, "?\\#\r\n") || strings.Contains(key, "..") || strings.HasPrefix(key, "/") {
		return nil, errors.New("invalid object key")
	}
	parts := strings.Split(key, "/")
	for i := range parts {
		parts[i] = url.PathEscape(parts[i])
	}
	req, e := http.NewRequestWithContext(ctx, method, s.endpoint+"/"+s.bucket+func() string {
		if key == "" {
			return ""
		}
		return "/" + strings.Join(parts, "/")
	}(), body)
	if e != nil {
		return nil, errors.New("invalid S3 request")
	}
	now := time.Now().UTC()
	stamp, day := now.Format("20060102T150405Z"), now.Format("20060102")
	req.ContentLength = size
	req.Header.Set("X-Amz-Date", stamp)
	req.Header.Set("X-Amz-Content-Sha256", payload)
	headers := "host:" + req.URL.Host + "\nx-amz-content-sha256:" + payload + "\nx-amz-date:" + stamp + "\n"
	signed := "host;x-amz-content-sha256;x-amz-date"
	canonical := method + "\n" + req.URL.EscapedPath() + "\n\n" + headers + "\n" + signed + "\n" + payload
	scope := day + "/" + s.region + "/s3/aws4_request"
	digest := mac(mac(mac(mac([]byte("AWS4"+s.secret), day), s.region), "s3"), "aws4_request")
	signature := hex.EncodeToString(mac(digest, "AWS4-HMAC-SHA256\n"+stamp+"\n"+scope+"\n"+Hash([]byte(canonical))))
	req.Header.Set("Authorization", "AWS4-HMAC-SHA256 Credential="+s.key+"/"+scope+", SignedHeaders="+signed+", Signature="+signature)
	return req, nil
}
func (s *S3) Put(ctx context.Context, key string, b []byte, media string) error {
	if key == "" || len(b) < 1 || len(b) > MaxBytes {
		return errors.New("artifact size exceeds budget")
	}
	// Every write is content-addressed. Some S3-compatible implementations
	// ignore conditional PUT headers: even there concurrent writes to this
	// key can only carry identical bytes. Never repair an existing corrupt
	// object by overwriting it; retain the integrity failure for investigation.
	if !strings.HasSuffix(key, "/"+Hash(b)) {
		return errors.New("object key must end with its exact content hash")
	}
	probe, e := s.request(ctx, "GET", key, nil)
	if e != nil {
		return e
	}
	prior, e := s.client.Do(probe)
	if e != nil {
		return errors.New("S3 existence check unavailable")
	}
	if prior.StatusCode == 200 {
		old, readErr := io.ReadAll(io.LimitReader(prior.Body, int64(len(b))+1))
		prior.Body.Close()
		if readErr != nil || !bytes.Equal(old, b) {
			return errors.New("immutable object conflict")
		}
		return nil
	}
	prior.Body.Close()
	if prior.StatusCode != 404 {
		return fmt.Errorf("S3 existence status %d", prior.StatusCode)
	}
	req, e := s.request(ctx, "PUT", key, b)
	if e != nil {
		return e
	}
	req.Header.Set("Content-Type", media)
	req.Header.Set("If-None-Match", "*")
	r, e := s.client.Do(req)
	if e != nil {
		return errors.New("S3 put unavailable")
	}
	defer r.Body.Close()
	if r.StatusCode == http.StatusPreconditionFailed {
		old, e := s.Get(ctx, key, int64(len(b)))
		if e != nil || !bytes.Equal(old, b) {
			return errors.New("immutable object conflict")
		}
		return nil
	}
	if r.StatusCode/100 != 2 {
		return fmt.Errorf("S3 put status %d", r.StatusCode)
	}
	return nil
}
func (s *S3) Get(ctx context.Context, key string, size int64) ([]byte, error) {
	if size < 1 || size > MaxBytes {
		return nil, errors.New("artifact size exceeds budget")
	}
	req, e := s.request(ctx, "GET", key, nil)
	if e != nil {
		return nil, e
	}
	r, e := s.client.Do(req)
	if e != nil {
		return nil, errors.New("S3 get unavailable")
	}
	defer r.Body.Close()
	if r.StatusCode != 200 {
		return nil, fmt.Errorf("S3 get status %d", r.StatusCode)
	}
	b, e := io.ReadAll(io.LimitReader(r.Body, size+1))
	if e != nil || int64(len(b)) != size {
		return nil, errors.New("S3 object size mismatch")
	}
	return b, nil
}
func (s *S3) Delete(ctx context.Context, key string) error {
	req, e := s.request(ctx, "DELETE", key, nil)
	if e != nil {
		return e
	}
	r, e := s.client.Do(req)
	if e != nil {
		return errors.New("S3 delete unavailable")
	}
	defer r.Body.Close()
	if r.StatusCode/100 != 2 && r.StatusCode != 404 {
		return fmt.Errorf("S3 delete status %d", r.StatusCode)
	}
	return nil
}

// CreateBucket is an explicit installation helper, never called by API startup.
func (s *S3) CreateBucket(ctx context.Context) error {
	req, e := s.request(ctx, "PUT", "", nil)
	if e != nil {
		return e
	}
	r, e := s.client.Do(req)
	if e != nil {
		return e
	}
	defer r.Body.Close()
	if r.StatusCode/100 != 2 && r.StatusCode != 409 {
		return fmt.Errorf("S3 bucket setup status %d", r.StatusCode)
	}
	return nil
}

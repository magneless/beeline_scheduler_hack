package geo

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func retryProvider(t *testing.T, client *http.Client) *OSMProvider {
	t.Helper()
	p, err := NewOSMProvider(OSMOptions{Client: client, MinInterval: time.Nanosecond})
	if err != nil {
		t.Fatal(err)
	}
	return p
}
func TestGetRetriesTemporaryFailureThenSucceeds(t *testing.T) {
	var n int32
	client := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if atomic.AddInt32(&n, 1) == 1 {
			return nil, timeoutErr{}
		}
		return response(200, `{"type":"FeatureCollection"}`), nil
	})}
	p := retryProvider(t, client)
	var out map[string]any
	if err := p.get(context.Background(), "http://example.test", &out); err != nil {
		t.Fatal(err)
	}
	if n != 2 {
		t.Fatalf("attempts=%d", n)
	}
}
func TestGetRetries503TwiceAndStopsAtThree(t *testing.T) {
	var n int32
	client := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		k := atomic.AddInt32(&n, 1)
		if k < 3 {
			return response(503, "busy"), nil
		}
		return response(200, `{}`), nil
	})}
	p := retryProvider(t, client)
	if err := p.get(context.Background(), "http://x", &map[string]any{}); err != nil {
		t.Fatal(err)
	}
	if n != 3 {
		t.Fatalf("attempts=%d", n)
	}
	var m int32
	bad := retryProvider(t, &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) { atomic.AddInt32(&m, 1); return response(503, "busy"), nil })})
	if err := bad.get(context.Background(), "http://x2", &map[string]any{}); err == nil || m != 3 {
		t.Fatalf("err=%v attempts=%d", err, m)
	}
}
func TestGetDoesNotRetryClientOrJSONErrors(t *testing.T) {
	var n int32
	c := retryProvider(t, &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) { atomic.AddInt32(&n, 1); return response(400, "bad"), nil })})
	if err := c.get(context.Background(), "http://x", &map[string]any{}); err == nil || n != 1 {
		t.Fatalf("err=%v n=%d", err, n)
	}
	n = 0
	c = retryProvider(t, &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		atomic.AddInt32(&n, 1)
		return response(200, "not-json"), nil
	})})
	if err := c.get(context.Background(), "http://x", &map[string]any{}); err == nil || n != 1 {
		t.Fatalf("err=%v n=%d", err, n)
	}
}
func TestGetCancellationDuringRetryStops(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	var n int32
	p := retryProvider(t, &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		atomic.AddInt32(&n, 1)
		cancel()
		return nil, timeoutErr{}
	})})
	err := p.get(ctx, "http://x", &map[string]any{})
	if !errors.Is(err, context.Canceled) || n != 1 {
		t.Fatalf("err=%v n=%d", err, n)
	}
}
func TestGetRetryAfterTooLongStops(t *testing.T) {
	var n int32
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&n, 1)
		w.Header().Set("Retry-After", "31")
		w.WriteHeader(429)
	}))
	defer s.Close()
	p := retryProvider(t, s.Client())
	err := p.get(context.Background(), s.URL, &map[string]any{})
	if err == nil || n != 1 {
		t.Fatalf("err=%v n=%d", err, n)
	}
}

func TestParseRetryAfterHTTPDate(t *testing.T) {
	d, ok := parseRetryAfter(time.Now().Add(10 * time.Second).UTC().Format(http.TimeFormat))
	if !ok || d < 9*time.Second || d > 10*time.Second {
		t.Fatalf("d=%s ok=%v", d, ok)
	}
	d, ok = parseRetryAfter(time.Now().Add(40 * time.Second).UTC().Format(http.TimeFormat))
	if !ok || d <= 30*time.Second {
		t.Fatalf("long date d=%s ok=%v", d, ok)
	}
}

func TestGetAppliesIntervalToEveryAttempt(t *testing.T) {
	var n int32
	var times []time.Time
	c := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		times = append(times, time.Now())
		atomic.AddInt32(&n, 1)
		if n < 3 {
			return response(503, "busy"), nil
		}
		return response(200, `{}`), nil
	})}
	p, err := NewOSMProvider(OSMOptions{Client: c, MinInterval: 1100 * time.Millisecond})
	if err != nil {
		t.Fatal(err)
	}
	if err = p.get(context.Background(), "http://interval", &map[string]any{}); err != nil {
		t.Fatal(err)
	}
	for i := 1; i < len(times); i++ {
		if d := times[i].Sub(times[i-1]); d < 1100*time.Millisecond {
			t.Fatalf("interval %s", d)
		}
	}
}
func TestGetFailuresAreNotCached(t *testing.T) {
	var n int32
	dir := t.TempDir()
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { atomic.AddInt32(&n, 1); w.WriteHeader(500) }))
	defer s.Close()
	p := retryProvider(t, s.Client())
	p.o.CacheDir = dir
	if err := p.get(context.Background(), s.URL, &map[string]any{}); err == nil {
		t.Fatal("expected failure")
	}
	if _, ok := p.loadCache(s.URL); ok {
		t.Fatal("failure cached")
	}
}
func TestGetClientTimeoutIsRetried(t *testing.T) {
	var n int32
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if atomic.AddInt32(&n, 1) == 1 {
			<-r.Context().Done()
			return
		}
		_, _ = w.Write([]byte(`{}`))
	}))
	defer s.Close()
	p := retryProvider(t, &http.Client{Timeout: 200 * time.Millisecond})
	if err := p.get(context.Background(), s.URL, &map[string]any{}); err != nil {
		t.Fatal(err)
	}
	if n != 2 {
		t.Fatalf("attempts=%d", n)
	}
}
func TestGetDoesNotHonorRetryAfterAfterThirdAttempt(t *testing.T) {
	var n int32
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		k := atomic.AddInt32(&n, 1)
		if k == 3 {
			w.Header().Set("Retry-After", "30")
		}
		w.WriteHeader(503)
	}))
	defer s.Close()
	started := time.Now()
	p := retryProvider(t, s.Client())
	if err := p.get(context.Background(), s.URL, &map[string]any{}); err == nil {
		t.Fatal("expected failure")
	}
	if n != 3 {
		t.Fatalf("attempts=%d", n)
	}
	if time.Since(started) > 5*time.Second {
		t.Fatal("waited for third Retry-After")
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

type timeoutErr struct{}

func (timeoutErr) Error() string   { return "timeout" }
func (timeoutErr) Timeout() bool   { return true }
func (timeoutErr) Temporary() bool { return true }
func response(code int, body string) *http.Response {
	return &http.Response{StatusCode: code, Status: http.StatusText(code), Header: make(http.Header), Body: io.NopCloser(bytes.NewBufferString(body))}
}

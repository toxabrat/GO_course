package urls

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type getFunc func(string) (*http.Response, error)

func (f getFunc) Get(url string) (*http.Response, error) { return f(url) }

type trackedBody struct {
	io.Reader
	closed atomic.Bool
}

func (b *trackedBody) Close() error {
	b.closed.Store(true)
	return nil
}

func waitFor(t *testing.T, condition func() bool) {
	t.Helper()
	deadline := time.NewTimer(3 * time.Second)
	defer deadline.Stop()
	tick := time.NewTicker(time.Millisecond)
	defer tick.Stop()
	for !condition() {
		select {
		case <-deadline.C:
			t.Fatal("timed out waiting for condition")
		case <-tick.C:
		}
	}
}

func TestAggregatorStreamingAndSnapshots(t *testing.T) {
	gate := make(chan struct{})
	var release sync.Once
	unblock := func() { release.Do(func() { close(gate) }) }
	t.Cleanup(unblock)
	fastBody := &trackedBody{Reader: strings.NewReader("fast")}
	slowBody := &trackedBody{Reader: strings.NewReader("slow")}
	client := getFunc(func(url string) (*http.Response, error) {
		if url == "slow" {
			<-gate
			return &http.Response{StatusCode: 404, Body: slowBody}, nil
		}
		return &http.Response{StatusCode: 200, Body: fastBody}, nil
	})
	input := make(chan string, 2)
	input <- "slow"
	input <- "fast"
	close(input)
	result, err := NewURLAggregator(client).Aggregate(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}
	waitFor(t, func() bool { return result.GetResponsesCount(200) == 1 })
	if result.Done() {
		t.Fatal("aggregation completed before slow request")
	}
	snapshot := result.GetResult()
	snapshot[200] = 1000
	snapshot[500] = 1
	if result.GetResponsesCount(200) != 1 || result.GetResponsesCount(500) != 0 {
		t.Fatal("external map mutation changed the result")
	}
	unblock()
	waitFor(t, result.Done)
	if result.GetResponsesCount(404) != 1 || !fastBody.closed.Load() || !slowBody.closed.Load() {
		t.Fatal("missing responses or unclosed response body")
	}
}

func TestAggregatorConcurrentReaders(t *testing.T) {
	const requests = 200
	client := getFunc(func(url string) (*http.Response, error) {
		index, _ := strconv.Atoi(url)
		return &http.Response{StatusCode: 200 + index%2, Body: io.NopCloser(strings.NewReader(""))}, nil
	})
	input := make(chan string, requests)
	for i := 0; i < requests; i++ {
		input <- strconv.Itoa(i)
	}
	close(input)
	result, err := NewURLAggregator(client).Aggregate(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}
	var readers sync.WaitGroup
	for i := 0; i < 16; i++ {
		readers.Add(1)
		go func() {
			defer readers.Done()
			for !result.Done() {
				_ = result.GetResponsesCount(200)
				snapshot := result.GetResult()
				snapshot[500]++
			}
		}()
	}
	waitFor(t, result.Done)
	readers.Wait()
	if result.GetResponsesCount(200) != requests/2 || result.GetResponsesCount(201) != requests/2 || result.GetResponsesCount(500) != 0 {
		t.Fatalf("wrong counts: %v", result.GetResult())
	}
}

func TestAggregatorCancellationStopsHTTPRequest(t *testing.T) {
	started := make(chan struct{})
	stopped := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		close(started)
		<-r.Context().Done()
		close(stopped)
	}))
	t.Cleanup(server.Close)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	input := make(chan string, 1)
	input <- server.URL
	close(input)
	client := server.Client()
	originalTransport := client.Transport
	result, err := NewURLAggregator(client).Aggregate(ctx, input)
	if err != nil {
		t.Fatal(err)
	}
	select {
	case <-started:
	case <-time.After(3 * time.Second):
		t.Fatal("request never started")
	}
	cancel()
	waitFor(t, result.Done)
	select {
	case <-stopped:
	case <-time.After(3 * time.Second):
		t.Fatal("request context was not cancelled")
	}
	if len(result.GetResult()) != 0 || client.Transport != originalTransport {
		t.Fatal("cancelled request was counted or caller's client was modified")
	}
}

func TestAggregatorCancellationWithCustomClient(t *testing.T) {
	started := make(chan struct{})
	gate := make(chan struct{})
	var once sync.Once
	unblock := func() { once.Do(func() { close(gate) }) }
	t.Cleanup(unblock)
	body := &trackedBody{Reader: strings.NewReader("")}
	client := getFunc(func(string) (*http.Response, error) {
		close(started)
		<-gate
		return &http.Response{StatusCode: 200, Body: body}, nil
	})
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	input := make(chan string, 1)
	input <- "slow"
	result, err := NewURLAggregator(client).Aggregate(ctx, input)
	if err != nil {
		t.Fatal(err)
	}
	select {
	case <-started:
	case <-time.After(3 * time.Second):
		t.Fatal("request never started")
	}
	cancel()
	waitFor(t, result.Done)
	unblock()
	waitFor(t, body.closed.Load)
	if len(result.GetResult()) != 0 {
		t.Fatal("late response changed a completed result")
	}
}

func TestAggregatorErrorsAndEmptyInput(t *testing.T) {
	body := &trackedBody{Reader: strings.NewReader("")}
	client := getFunc(func(url string) (*http.Response, error) {
		if url == "nil" {
			return nil, nil
		}
		return &http.Response{StatusCode: 500, Body: body}, errors.New("network failure")
	})
	for _, entries := range [][]string{nil, {"error", "nil"}} {
		input := make(chan string, len(entries))
		for _, entry := range entries {
			input <- entry
		}
		close(input)
		result, err := NewURLAggregator(client).Aggregate(context.Background(), input)
		if err != nil {
			t.Fatal(err)
		}
		waitFor(t, result.Done)
		if len(result.GetResult()) != 0 {
			t.Fatalf("failed request was counted: %v", result.GetResult())
		}
	}
	if !body.closed.Load() {
		t.Fatal("error response body was not closed")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := NewURLAggregator(client).Aggregate(ctx, nil); !errors.Is(err, context.Canceled) {
		t.Fatalf("pre-cancelled context: %v", err)
	}
	for _, nilClient := range []HttpClient{nil, (*http.Client)(nil)} {
		if _, err := NewURLAggregator(nilClient).Aggregate(context.Background(), nil); err == nil {
			t.Fatal("nil client must be rejected")
		}
	}
}

func TestAggregatorCancellationWhileWaitingForInput(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	client := getFunc(func(string) (*http.Response, error) {
		t.Error("unexpected request")
		return nil, errors.New("unexpected request")
	})
	result, err := NewURLAggregator(client).Aggregate(ctx, make(chan string))
	if err != nil {
		t.Fatal(err)
	}
	cancel()
	waitFor(t, result.Done)
}

func TestAggregatorPreservesClientTimeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))
	defer server.Close()
	client := server.Client()
	client.Timeout = 50 * time.Millisecond
	input := make(chan string, 1)
	input <- server.URL
	close(input)
	result, err := NewURLAggregator(client).Aggregate(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}
	waitFor(t, result.Done)
	if len(result.GetResult()) != 0 {
		t.Fatal("timed-out request was counted")
	}
}

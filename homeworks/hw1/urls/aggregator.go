package urls

import (
	"context"
	"fmt"
	"log"
	"net/http"
)

type ResponseCodeAggregator struct {
	client HttpClient
}

func NewURLAggregator(client HttpClient) *ResponseCodeAggregator {
	return &ResponseCodeAggregator{client: client}
}

func (u *ResponseCodeAggregator) Aggregate(ctx context.Context, urls <-chan string) (AggregationResult, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if u.client == nil {
		return nil, fmt.Errorf("HTTP client must not be nil")
	}
	client := u.client
	if httpClient, ok := client.(*http.Client); ok {
		if httpClient == nil {
			return nil, fmt.Errorf("HTTP client must not be nil")
		}
		client = contextClient{ctx: ctx, client: httpClient}
	}
	result := &aggregationResult{counts: make(map[int]int)}
	const workers = 8
	go func() {
		defer result.finish()
		completed := make(chan struct{}, workers)
		for i := 0; i < workers; i++ {
			go func() {
				defer func() { completed <- struct{}{} }()
				for {
					select {
					case <-ctx.Done():
						return
					case url, ok := <-urls:
						if !ok || ctx.Err() != nil {
							return
						}
						collectResponse(ctx, client, url, result)
					}
				}
			}()
		}
		for i := 0; i < workers; i++ {
			select {
			case <-ctx.Done():
				return
			case <-completed:
			}
		}
	}()
	return result, nil
}

func collectResponse(ctx context.Context, client HttpClient, url string, result *aggregationResult) {
	response, err := client.Get(url)
	if response != nil && response.Body != nil {
		defer response.Body.Close()
	}
	if err != nil {
		log.Printf("GET %q: %v", url, err)
		return
	}
	if response == nil {
		log.Printf("GET %q: client returned a nil response", url)
		return
	}
	if ctx.Err() == nil {
		result.addResponse(response.StatusCode)
	}
}

type contextClient struct {
	ctx    context.Context
	client *http.Client
}

func (c contextClient) Get(url string) (*http.Response, error) {
	request, err := http.NewRequestWithContext(c.ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	return c.client.Do(request)
}

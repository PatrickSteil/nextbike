package client

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/PatrickSteil/nextbike/models"
)

const baseURL = "https://maps.nextbike.net"

type QueryParams struct {
	CityUIDs []int    // filter to specific cities; empty = all
	Country  []string // e.g. "de", "at"
}

type Client struct {
	http    *http.Client
	retries int
}

func New() *Client {
	return &Client{
		http:    &http.Client{Timeout: 10 * time.Second},
		retries: 3,
	}
}

func (c *Client) Fetch(ctx context.Context, p QueryParams) (*models.Response, error) {
	q := url.Values{}
	for _, uid := range p.CityUIDs {
		q.Add("city", strconv.Itoa(uid))
	}
	for _, d := range p.Country {
		q.Add("country", d)
	}

	endpoint := fmt.Sprintf("%s/maps/nextbike-official.json?%s", baseURL, q.Encode())

	var lastErr error
	for attempt := range c.retries {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(time.Duration(1<<attempt) * time.Second):
			}
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
		if err != nil {
			return nil, fmt.Errorf("build request: %w", err)
		}
		req.Header.Set("Accept", "application/json")
		req.Header.Set("User-Agent", "nextbike-go/1.0")

		res, err := c.http.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("attempt %d: %w", attempt+1, err)
			continue
		}
		defer res.Body.Close()

		if res.StatusCode != http.StatusOK {
			lastErr = fmt.Errorf("attempt %d: HTTP %d", attempt+1, res.StatusCode)
			continue
		}

		var resp models.Response
		if err := json.NewDecoder(res.Body).Decode(&resp); err != nil {
			return nil, fmt.Errorf("decode: %w", err)
		}
		return &resp, nil
	}

	return nil, fmt.Errorf("all retries failed: %w", lastErr)
}

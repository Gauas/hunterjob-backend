package crawler

import (
	"context"
	"fmt"
	"github.com/hunterjob/hunterjob/api/internal/company"
	"io"
	"net"
	"net/http"
	"net/url"
)

type Response struct {
	URL         string
	ContentType string
	Body        []byte
}

type Fetcher interface {
	Fetch(context.Context, company.Source) (Response, error)
}

type HTTPFetcher struct {
	Client    *http.Client
	UserAgent string
	MaxBody   int64
}

func (a HTTPFetcher) Fetch(ctx context.Context, s company.Source) (Response, error) {
	if err := safeURL(s.CareerURL); err != nil {
		return Response{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.CareerURL, nil)
	if err != nil {
		return Response{}, err
	}
	req.Header.Set("Accept", "application/json, text/html;q=0.9, */*;q=0.8")
	req.Header.Set("User-Agent", a.UserAgent)
	res, err := a.Client.Do(req)
	if err != nil {
		return Response{}, err
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode > 299 {
		return Response{}, fmt.Errorf("career endpoint returned %s", res.Status)
	}
	body, err := io.ReadAll(io.LimitReader(res.Body, a.MaxBody+1))
	if err != nil {
		return Response{}, err
	}
	if int64(len(body)) > a.MaxBody {
		return Response{}, fmt.Errorf("career endpoint response exceeds %d bytes", a.MaxBody)
	}
	return Response{URL: s.CareerURL, ContentType: res.Header.Get("Content-Type"), Body: body}, nil
}

func safeURL(raw string) error {
	u, e := url.Parse(raw)
	if e != nil || u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("unsafe URL")
	}
	ips, e := net.LookupIP(u.Hostname())
	if e != nil {
		return e
	}
	for _, ip := range ips {
		if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() {
			return fmt.Errorf("private URL blocked")
		}
	}
	return nil
}

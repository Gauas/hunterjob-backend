package crawler

import (
	"context"
	"crypto/sha256"
	"fmt"
	"github.com/PuerkitoBio/goquery"
	"github.com/hunterjob/hunterjob/api/internal/company"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type Candidate struct {
	CompanyID, SourceID                                    string
	SourceURL, OriginalURL, RawTitle, RawLocation, RawText string
	Confidence                                             float64
}
type Adapter interface {
	CanHandle(company.Source) bool
	DiscoverJobs(context.Context, company.Source) ([]Candidate, error)
}
type GenericHTTPAdapter struct {
	Client    *http.Client
	UserAgent string
	MaxBody   int64
}

func (a GenericHTTPAdapter) CanHandle(s company.Source) bool { return s.Provider == "generic" }
func (a GenericHTTPAdapter) DiscoverJobs(ctx context.Context, s company.Source) ([]Candidate, error) {
	if err := safeURL(s.CareerURL); err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.CareerURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", a.UserAgent)
	res, err := a.Client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode > 299 {
		return nil, fmt.Errorf("career page returned %s", res.Status)
	}
	doc, err := goquery.NewDocumentFromReader(io.LimitReader(res.Body, a.MaxBody))
	if err != nil {
		return nil, err
	}
	base, _ := url.Parse(s.CareerURL)
	seen := map[string]bool{}
	out := []Candidate{}
	doc.Find("a[href]").Each(func(_ int, sel *goquery.Selection) {
		href, ok := sel.Attr("href")
		if !ok {
			return
		}
		u, err := url.Parse(href)
		if err != nil {
			return
		}
		absolute := base.ResolveReference(u)
		if absolute.Scheme != "http" && absolute.Scheme != "https" {
			return
		}
		title := strings.TrimSpace(sel.Text())
		parent := strings.TrimSpace(sel.Parent().Text())
		text := strings.TrimSpace(title + " " + parent)
		score := scoreCandidate(absolute.Path, title, text, s.URLPatterns)
		if score < 6 || seen[absolute.String()] {
			return
		}
		seen[absolute.String()] = true
		out = append(out, Candidate{CompanyID: s.CompanyID.Hex(), SourceID: s.ID.Hex(), SourceURL: s.CareerURL, OriginalURL: absolute.String(), RawTitle: title, RawText: text, RawLocation: location(text), Confidence: float64(score) / 10})
	})
	return out, nil
}
func scoreCandidate(path, title, text string, patterns []string) int {
	l := strings.ToLower(path)
	score := 0
	for _, p := range append(patterns, "/job/", "/jobs/", "/career/", "/careers/", "/position/", "/vacancy/", "/post/") {
		if strings.Contains(l, p) {
			score += 3
			break
		}
	}
	words := []string{"engineer", "developer", "software", "backend", "frontend", "full stack", "devops", "cloud", "qa", "tester", "data", "intern", "junior", "senior"}
	for _, w := range words {
		if strings.Contains(strings.ToLower(title), w) {
			score += 3
			break
		}
	}
	if location(text) != "" {
		score += 2
	}
	if strings.Contains(strings.ToLower(text), "full-time") {
		score++
	}
	return score
}
func location(v string) string {
	for _, x := range []string{"Da Nang", "Hanoi", "Ha Noi", "Ho Chi Minh", "Remote"} {
		if strings.Contains(strings.ToLower(v), strings.ToLower(x)) {
			return x
		}
	}
	return ""
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
func Hash(s string) string { return fmt.Sprintf("%x", sha256.Sum256([]byte(s))) }

var _ = time.Second

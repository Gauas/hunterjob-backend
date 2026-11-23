package joblake

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/hunterjob/hunterjob/api/internal/geography"
	"github.com/hunterjob/hunterjob/api/internal/jobevent"
	"golang.org/x/net/html"
)

const smartRecruitersURL = "https://api.smartrecruiters.com/v1/companies"

var publicIDPattern = regexp.MustCompile(`^[0-9]{1,30}$`)
var errPostingGone = errors.New("posting no longer exists")

type SmartRecruiters struct {
	client *http.Client
	next   time.Time
}

func NewSmartRecruiters() *SmartRecruiters {
	return &SmartRecruiters{client: &http.Client{Timeout: 15 * time.Second}}
}

type location struct {
	City         string `json:"city"`
	Country      string `json:"country"`
	FullLocation string `json:"fullLocation"`
}

func (l location) inHoChiMinh() bool {
	return strings.EqualFold(l.Country, "vn") &&
		(geography.IsHoChiMinh(l.City) || geography.IsHoChiMinh(l.FullLocation))
}

type posting struct {
	ID         string     `json:"id"`
	JobID      string     `json:"jobId"`
	Title      string     `json:"name"`
	Location   location   `json:"location"`
	PostingURL string     `json:"postingUrl"`
	Active     bool       `json:"active"`
	Visibility string     `json:"visibility"`
	Released   *time.Time `json:"releasedDate"`
	Employment struct {
		Label string `json:"label"`
	} `json:"typeOfEmployment"`
	JobAd struct {
		Sections struct {
			Description struct {
				Text string `json:"text"`
			} `json:"jobDescription"`
			Qualifications struct {
				Text string `json:"text"`
			} `json:"qualifications"`
		} `json:"sections"`
	} `json:"jobAd"`
}

// Fetch returns one complete snapshot for a company's TP.HCM postings.
func (s *SmartRecruiters) Fetch(ctx context.Context, source Source) ([]jobevent.Job, error) {
	base := smartRecruitersURL + "/" + url.PathEscape(source.Board) + "/postings"
	query := url.Values{"country": {"vn"}, "destination": {"PUBLIC"}, "limit": {"100"}}
	var jobs []jobevent.Job
	seenPostings := map[string]bool{}
	seenJobs := map[string]bool{}
	for offset := 0; offset < 10000; {
		query.Set("offset", strconv.Itoa(offset))
		var page struct {
			Content []posting `json:"content"`
			Total   *int      `json:"totalFound"`
			Offset  *int      `json:"offset"`
		}
		if err := s.get(ctx, base+"?"+query.Encode(), &page); err != nil {
			return nil, fmt.Errorf("list postings at offset %d: %w", offset, err)
		}
		if page.Total == nil || page.Offset == nil || *page.Offset != offset || (len(page.Content) == 0 && offset < *page.Total) {
			return nil, fmt.Errorf("unexpected postings page at offset %d", offset)
		}
		for _, summary := range page.Content {
			if !summary.Location.inHoChiMinh() || !publicIDPattern.MatchString(summary.ID) || seenPostings[summary.ID] {
				continue
			}
			seenPostings[summary.ID] = true
			var detail posting
			err := s.get(ctx, base+"/"+summary.ID, &detail)
			if errors.Is(err, errPostingGone) {
				continue
			}
			if err != nil {
				return nil, fmt.Errorf("read posting %s: %w", summary.ID, err)
			}
			if job, ok := normalize(source, detail); ok && !seenJobs[job.ID] {
				seenJobs[job.ID] = true
				jobs = append(jobs, job)
			}
		}
		offset += len(page.Content)
		if offset >= *page.Total {
			return jobs, nil
		}
	}
	return nil, errors.New("postings pagination exceeded 10,000 records")
}

func (s *SmartRecruiters) get(ctx context.Context, endpoint string, result any) error {
	if delay := time.Until(s.next); delay > 0 {
		timer := time.NewTimer(delay)
		defer timer.Stop()
		select {
		case <-timer.C:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	s.next = time.Now().Add(500 * time.Millisecond)
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return err
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("User-Agent", "HunterJob career feeds")
	response, err := s.client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusNotFound || response.StatusCode == http.StatusGone {
		return errPostingGone
	}
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("SmartRecruiters returned HTTP %d", response.StatusCode)
	}
	return json.NewDecoder(io.LimitReader(response.Body, 4<<20)).Decode(result)
}

func normalize(source Source, p posting) (jobevent.Job, bool) {
	link, err := url.Parse(p.PostingURL)
	title := strings.TrimSpace(p.Title)
	if err != nil || link.Scheme != "https" || link.Hostname() == "" || link.User != nil ||
		title == "" || !p.Active || p.Visibility != "PUBLIC" || !p.Location.inHoChiMinh() || p.ID == "" {
		return jobevent.Job{}, false
	}
	id := p.JobID
	if id == "" {
		id = p.ID
	}
	externalID := strings.ToLower(source.Board) + ":" + id
	return jobevent.Job{
		ID:         "smartrecruiters:" + externalID,
		ExternalID: externalID,
		Source:     "smartrecruiters", Title: title, CompanyName: source.Company,
		Location: geography.HoChiMinhCity, EmploymentType: p.Employment.Label,
		Description: plainText(p.JobAd.Sections.Description.Text + " " + p.JobAd.Sections.Qualifications.Text),
		JobURL:      p.PostingURL, PublishedAt: p.Released, DiscoveredAt: time.Now().UTC(), Status: "active",
	}, true
}

func plainText(value string) string {
	tokens := html.NewTokenizer(strings.NewReader(value))
	var words []string
	for {
		switch tokens.Next() {
		case html.TextToken:
			words = append(words, strings.Fields(string(tokens.Text()))...)
		case html.ErrorToken:
			return strings.Join(words, " ")
		}
	}
}

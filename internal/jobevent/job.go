package jobevent

import "time"

// Job is the normalized job.new event contract owned by Job Lake.
type Job struct {
	ID             string     `json:"id"`
	ExternalID     string     `json:"external_id"`
	Source         string     `json:"source"`
	Title          string     `json:"title"`
	CompanyName    string     `json:"company_name"`
	CompanyLogoURL string     `json:"company_logo_url"`
	Location       string     `json:"location"`
	EmploymentType string     `json:"employment_type"`
	ExperienceMin  *int       `json:"experience_min"`
	ExperienceMax  *int       `json:"experience_max"`
	Skills         []string   `json:"skills"`
	Description    string     `json:"description"`
	JobURL         string     `json:"job_url"`
	PublishedAt    *time.Time `json:"published_at"`
	DiscoveredAt   time.Time  `json:"discovered_at"`
	Status         string     `json:"status"`
}

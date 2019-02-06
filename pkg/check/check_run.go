package check

import (
	"time"
)

const RFC8601 = `2006-01-02T15:04:05Z`

// NewRun creates a new Run with the required fields set.
//
// Note that Conclusion is set to `neutral` by default, and CompletedAt is
// set to now.
func NewRun(name, branch, sha string) *Run {
	return &Run{
		Name:       name,
		HeadBranch: branch,
		HeadSHA:    sha,
		//Conclusion:  "neutral",
		//CompletedAt: time.Now().Format(RFC8601),
		StartedAt: time.Now().Format(RFC8601),
	}
}

// Run describes the Run object
// https://developer.github.com/v3/checks/runs/#create-a-check-run
type Run struct {
	// Name is the required human-friendly name of the job
	Name string `json:"name"`
	// HeadBranch is the required branch name
	HeadBranch string `json:"head_branch"`
	// HeadSHA is the required commit ID
	HeadSHA string `json:"head_sha"`

	// Conclusion is the required conclusion. It must be one of:
	//
	//	- success
	//	- failure
	//	- neutral
	//	- cancelled
	//	- timed_out
	//	- action_required
	//
	// This is required if status is set
	//
	// If action_required is specified, you must also set details_url
	//
	// NB: I think the documentation is incorrect; I do not believe this is
	// required for a run that is in progress.
	Conclusion string `json:"conclusion,omitempty"`

	// URL for further details
	DetailsURL string `json:"details_url,omitempty"`

	// ExternalID is the ID in a correlated system.
	//
	// For example, you could set this to the Brigade BuildID for easy
	// cross linking with Kashti.
	ExternalID string `json:"external_id,omitempty"`

	// Status is one of queued, in_progress, or completed.
	// Queued is the default
	Status string `json:"status,omitempty"`

	// StartedAt is an ISO 8601 date stamp, YYYY-MM-DDTHH:MM:SSZ
	StartedAt string `json:"started_at,omitempty"`

	// CompletedAt is an ISO 8601 date stamp, YYYY-MM-DDTHH:MM:SSZ
	CompletedAt string `json:"completed_at,omitempty"`

	// Output is the output of this status message.
	Output Output `json:"output,omitempty"`
}

// Output is the rich output of a check run
type Output struct {
	// Title is the required title
	Title string `json:"title"`
	// Summary is the required summary.
	// Markdown allowed
	Summary string `json:"summary"`
	// Text is the details
	// Markdown allowed
	Text string `json:"text,omitempty"`

	// Annotations is a list of annotations
	Annotations []Annotation `json:"annotations,omitempty"`

	// Images is a list of images.
	Images []Image `json:"images,omitempty"`
}

// Annotation is a file annotation
type Annotation struct {
	Filename     string `json:"filename"`
	BlobHRef     string `json:"blob_href"`
	StartLine    int    `json:"start_line"`
	EndLine      int    `json:"end_line"`
	WarningLevel string `json:"warning_level"`
	Message      string `json:"message"`
	Title        string `json:"title,omitempty"`
	RawDetails   string `json:"raw_details,omitempty"`
}

// Image is an image attachment
type Image struct {
	ImageURL string `json:"image_url"`
	Alt      string `json:"alt"`
	Caption  string `json:"caption,omitempty"`
}

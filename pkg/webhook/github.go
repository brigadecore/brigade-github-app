package webhook

import (
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"

	"github.com/google/go-github/github"
	"gopkg.in/gin-gonic/gin.v1"

	"github.com/Azure/brigade/pkg/brigade"
	"github.com/Azure/brigade/pkg/storage"
)

const (
	brigadeJSFile      = "brigade.js"
	hubSignatureHeader = "X-Hub-Signature"
)

// EventCheckSuite is a placeholder for JSON unmarshalling.
// This will be replaced when the Go GitHub library catches up.
type EventCheckSuite struct {
	Action     string     `json:"action"`
	CheckSuite CheckSuite `json:"check_suite"`
	Repo       Repository `json:"repository"`
}

// EventCheckRun is a placeholder for JSON unmarshalling.
// This will be replaced when the Go GitHub library catches up.
type EventCheckRun struct {
	Action   string `json:"action"`
	CheckRun struct {
		HeadSHA    string     `json:"head_sha"`
		CheckSuite CheckSuite `json:"check_suite"`
	} `json:"check_run"`
	Repo Repository `json:"repository"`
}

// CheckSuite is a placeholder for JSON unmarshalling.
// This will be replaced when the Go GitHub library catches up.
type CheckSuite struct {
	HeadBranch string `json:"head_branch"`
	HeadSHA    string `json:"head_sha"`
}

// Respository is a placeholder for JSON unmarshalling.
// This will be replaced when the Go GitHub library catches up.
type Repository struct {
	FullName string `json:"full_name"`
}

type githubHook struct {
	store          storage.Store
	getFile        fileGetter
	createStatus   statusCreator
	allowedAuthors []string
}

type fileGetter func(commit, path string, proj *brigade.Project) ([]byte, error)

type statusCreator func(commit string, proj *brigade.Project, status *github.RepoStatus) error

// NewGithubHook creates a GitHub webhook handler.
func NewGithubHook(s storage.Store, authors []string) gin.HandlerFunc {
	gh := &githubHook{
		store:          s,
		getFile:        getFileFromGithub,
		createStatus:   setRepoStatus,
		allowedAuthors: authors,
	}
	return gh.Handle
}

// Handle routes a webhook to its appropriate handler.
//
// It does this by sniffing the event from the header, and routing accordingly.
func (s *githubHook) Handle(c *gin.Context) {
	event := c.Request.Header.Get("X-GitHub-Event")
	switch event {
	case "ping":
		log.Print("Received ping from GitHub")
		c.JSON(200, gin.H{"message": "OK"})
		return
	case "push", "pull_request", "create", "release", "status", "commit_comment", "pull_request_review", "deployment", "deployment_status":
		s.handleEvent(c, event)
		return
	// Added
	case "check_suite", "check_run":
		s.handleCheck(c, event)
	default:
		// Issue #127: Don't return an error for unimplemented events.
		log.Printf("Unsupported event %q", event)
		c.JSON(200, gin.H{"message": "Ignored"})
		return
	}
}

func (s *githubHook) handleCheck(c *gin.Context, eventType string) {
	body, err := ioutil.ReadAll(c.Request.Body)
	if err != nil {
		log.Printf("Failed to read body: %s", err)
		c.JSON(http.StatusBadRequest, gin.H{"status": "Malformed body"})
		return
	}
	defer c.Request.Body.Close()

	log.Print(string(body))

	// This can be further refined
	brigEvent := eventType

	var repo string
	var rev brigade.Revision
	switch eventType {
	case "check_suite":
		e := &EventCheckSuite{}
		err := json.Unmarshal(body, e)
		if err != nil {
			log.Printf("Failed to parse body: %s", err)
			c.JSON(http.StatusBadRequest, gin.H{"status": "Malformed body"})
			return
		}

		// This can be check_suite:requested, check_suite:rerequested, and check_suite:completed
		brigEvent = fmt.Sprintf("%s:%s", eventType, e.Action)
		repo = e.Repo.FullName
		rev.Commit = e.CheckSuite.HeadSHA
		rev.Ref = fmt.Sprintf("refs/heads/%s", e.CheckSuite.HeadBranch)
	case "check_run":
		e := &EventCheckRun{}
		err := json.Unmarshal(body, e)
		if err != nil {
			log.Printf("Failed to parse body: %s", err)
			c.JSON(http.StatusBadRequest, gin.H{"status": "Malformed body"})
			return
		}

		brigEvent = fmt.Sprintf("%s:%s", eventType, e.Action)
		repo = e.Repo.FullName
		rev.Commit = e.CheckRun.HeadSHA
		rev.Ref = fmt.Sprintf("refs/heads/%s", e.CheckRun.CheckSuite.HeadBranch)
	}

	proj, err := s.store.GetProject(repo)
	if err != nil {
		log.Printf("Project %q not found. No secret loaded. %s", repo, err)
		c.JSON(http.StatusBadRequest, gin.H{"status": "project not found"})
		return
	}

	s.build(brigEvent, rev, body, proj)
	c.JSON(http.StatusOK, gin.H{"status": "Complete"})
}

func (s *githubHook) handleEvent(c *gin.Context, eventType string) {
	body, err := ioutil.ReadAll(c.Request.Body)
	if err != nil {
		log.Printf("Failed to read body: %s", err)
		c.JSON(http.StatusBadRequest, gin.H{"status": "Malformed body"})
		return
	}
	defer c.Request.Body.Close()

	e, err := github.ParseWebHook(eventType, body)
	if err != nil {
		log.Printf("Failed to parse body: %s", err)
		c.JSON(http.StatusBadRequest, gin.H{"status": "Malformed body"})
		return
	}

	var repo string
	var rev brigade.Revision

	switch e := e.(type) {
	case *github.PushEvent:
		// If this is a branch deletion, skip the build.
		if e.GetDeleted() {
			c.JSON(http.StatusOK, gin.H{"status": "build skipped on branch deletion"})
			return
		}

		repo = e.Repo.GetFullName()
		rev.Commit = e.HeadCommit.GetID()
		rev.Ref = e.GetRef()
	case *github.PullRequestEvent:
		if !s.isAllowedPullRequest(e) {
			c.JSON(http.StatusOK, gin.H{"status": "build skipped"})
			return
		}

		// EXPERIMENTAL: Since labeling and unlabeling PRs doesn't really have a
		// code impact, we don't really want to fire off the same event (or require
		// the user to know the event details). So we add a pseudo-event for labeling
		// actions.
		if a := e.GetAction(); a == "labeled" || a == "unlabeled" {
			eventType = "pull_request:" + a
		}

		repo = e.Repo.GetFullName()
		rev.Commit = e.PullRequest.Head.GetSHA()
		rev.Ref = fmt.Sprintf("refs/pull/%d/head", e.PullRequest.GetNumber())
	case *github.CommitCommentEvent:
		repo = e.Repo.GetFullName()
		rev.Commit = e.Comment.GetCommitID()
	case *github.CreateEvent:
		// TODO: There are three ref_type values: tag, branch, and repo. Do we
		// want to be opinionated about how we handle these?
		repo = e.Repo.GetFullName()
		rev.Ref = e.GetRef()
	case *github.ReleaseEvent:
		repo = e.Repo.GetFullName()
		rev.Ref = e.Release.GetTagName()
	case *github.StatusEvent:
		repo = e.Repo.GetFullName()
		rev.Commit = e.Commit.GetSHA()
	case *github.PullRequestReviewEvent:
		repo = e.Repo.GetFullName()
		rev.Commit = e.PullRequest.Head.GetSHA()
		rev.Ref = fmt.Sprintf("refs/pull/%d/head", e.PullRequest.GetNumber())
	case *github.DeploymentEvent:
		repo = e.Repo.GetFullName()
		rev.Commit = e.Deployment.GetSHA()
		rev.Ref = e.Deployment.GetRef()
	case *github.DeploymentStatusEvent:
		repo = e.Repo.GetFullName()
		rev.Commit = e.Deployment.GetSHA()
		rev.Ref = e.Deployment.GetRef()
	default:
		log.Printf("Failed to parse payload")
		c.JSON(http.StatusBadRequest, gin.H{"status": "Received data is not valid JSON"})
		return
	}

	proj, err := s.store.GetProject(repo)
	if err != nil {
		log.Printf("Project %q not found. No secret loaded. %s", repo, err)
		c.JSON(http.StatusBadRequest, gin.H{"status": "project not found"})
		return
	}

	s.build(eventType, rev, body, proj)
	c.JSON(http.StatusOK, gin.H{"status": "Complete"})
}

// isAllowedPullRequest returns true if this particular pull request is allowed
// to produce an event.
func (s *githubHook) isAllowedPullRequest(e *github.PullRequestEvent) bool {

	isFork := e.PullRequest.Head.Repo.GetFork()

	// This applies the author association to forked PRs.
	// PRs sent against origin will be accepted without a check.
	// See https://developer.github.com/v4/reference/enum/commentauthorassociation/
	if assoc := e.PullRequest.GetAuthorAssociation(); isFork && !s.isAllowedAuthor(assoc) {
		log.Printf("skipping pull request for disallowed author %s", assoc)
		return false
	}
	switch e.GetAction() {
	case "opened", "synchronize", "reopened", "labeled", "unlabeled", "closed":
		return true
	}
	log.Println("unsupported pull_request action:", e.GetAction())
	return false
}

func (s *githubHook) isAllowedAuthor(author string) bool {
	for _, a := range s.allowedAuthors {
		if a == author {
			return true
		}
	}
	return false
}

func truncAt(str string, max int) string {
	if len(str) > max {
		short := str[0 : max-3]
		return short + "..."
	}
	return str
}

func getFileFromGithub(commit, path string, proj *brigade.Project) ([]byte, error) {
	return GetFileContents(proj, commit, path)
}

func (s *githubHook) build(eventType string, rev brigade.Revision, payload []byte, proj *brigade.Project) error {
	brigadeScript, err := s.getFile(rev.Commit, brigadeJSFile, proj)
	if err != nil {
		if proj.DefaultScript == "" {
			return fmt.Errorf("no brigade.js found in either project's defaultScript or the git repository: %v", err)
		}
		brigadeScript = []byte(proj.DefaultScript)
	}

	b := &brigade.Build{
		ProjectID: proj.ID,
		Type:      eventType,
		Provider:  "github",
		Revision:  &rev,
		Payload:   payload,
		Script:    brigadeScript,
	}

	return s.store.CreateBuild(b)
}

// validateSignature compares the salted digest in the header with our own computing of the body.
func validateSignature(signature, secretKey string, payload []byte) error {
	sum := SHA1HMAC([]byte(secretKey), payload)
	if subtle.ConstantTimeCompare([]byte(sum), []byte(signature)) != 1 {
		log.Printf("Expected signature %q (sum), got %q (hub-signature)", sum, signature)
		return errors.New("payload signature check failed")
	}
	return nil
}

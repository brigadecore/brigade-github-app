package webhook

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"strings"

	"github.com/google/go-github/github"
	"gopkg.in/gin-gonic/gin.v1"

	"github.com/Azure/brigade/pkg/brigade"
	"github.com/Azure/brigade/pkg/storage"
)

const hubSignatureHeader = "X-Hub-Signature"

// ErrAuthFailed indicates some part of the auth handshake failed
//
// This is usually indicative of an auth failure between the client library and GitHub
var ErrAuthFailed = errors.New("Auth Failed")

type githubHook struct {
	store          storage.Store
	getFile        fileGetter
	createStatus   statusCreator
	opts           GithubOpts
	allowedAuthors []string
	// key is the x509 certificate key as ASCII-armored (PEM) data
	key []byte
}

// GithubOpts provides options for configuring a GitHub hook
type GithubOpts struct {
	// CheckSuiteOnPR will trigger a check suite run for new PRs that pass the security params.
	CheckSuiteOnPR bool
	AppID          int
}

type fileGetter func(commit, path string, proj *brigade.Project) ([]byte, error)

type statusCreator func(commit string, proj *brigade.Project, status *github.RepoStatus) error

// NewGithubHook creates a GitHub webhook handler.
func NewGithubHook(s storage.Store, authors []string, x509Key []byte, opts GithubOpts) gin.HandlerFunc {
	gh := &githubHook{
		store:          s,
		getFile:        getFileFromGithub,
		createStatus:   setRepoStatus,
		allowedAuthors: authors,
		key:            x509Key,
		opts:           opts,
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
	var res *Payload
	switch eventType {
	case "check_suite":
		e := &github.CheckSuiteEvent{}
		err := json.Unmarshal(body, e)
		if err != nil {
			log.Printf("Failed to parse body: %s", err)
			c.JSON(http.StatusBadRequest, gin.H{"status": "Malformed body"})
			return
		}

		res = &Payload{
			Body:   e,
			AppID:  int(e.CheckSuite.App.GetID()),
			InstID: int(e.Installation.GetID()),
			Type:   "check_suite",
		}

		if res.AppID != s.opts.AppID {
			log.Printf("This was destined for app %d, not us (%d)", res.AppID, s.opts.AppID)
			return
		}

		// This can be check_suite:requested, check_suite:rerequested, and check_suite:completed
		brigEvent = fmt.Sprintf("%s:%s", eventType, e.GetAction())
		repo = e.Repo.GetFullName()
		rev.Commit = e.CheckSuite.GetHeadSHA()
		rev.Ref = e.CheckSuite.GetHeadBranch()

	case "check_run":
		e := &github.CheckRunEvent{}
		err := json.Unmarshal(body, e)
		if err != nil {
			log.Printf("Failed to parse body: %s", err)
			c.JSON(http.StatusBadRequest, gin.H{"status": "Malformed body"})
			return
		}

		res = &Payload{
			Body:   e,
			AppID:  int(e.CheckRun.App.GetID()),
			InstID: int(e.Installation.GetID()),
			Type:   "check_run",
		}

		if res.AppID == 0 {
			res.AppID = int(e.CheckRun.CheckSuite.App.GetID())
		}

		if res.AppID != s.opts.AppID {
			log.Printf("This was destined for app %d, not us (%d)", res.AppID, s.opts.AppID)
			return
		}

		brigEvent = fmt.Sprintf("%s:%s", eventType, e.GetAction())
		repo = e.Repo.GetFullName()
		rev.Commit = e.CheckRun.CheckSuite.GetHeadSHA()
		rev.Ref = e.CheckRun.CheckSuite.GetHeadBranch()
	}

	proj, err := s.store.GetProject(repo)
	if err != nil {
		log.Printf("Project %q not found. No secret loaded. %s", repo, err)
		c.JSON(http.StatusBadRequest, gin.H{"status": "project not found"})
		return
	}

	if proj.SharedSecret == "" {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "No secret is configured for this repo."})
		return
	}

	signature := c.Request.Header.Get(hubSignatureHeader)
	if err := validateSignature(signature, proj.SharedSecret, body); err != nil {
		c.JSON(http.StatusForbidden, gin.H{"status": "malformed signature"})
		return
	}

	tok, timeout, err := s.installationToken(res.AppID, res.InstID, proj.Github)
	if err != nil {
		log.Printf("Failed to negotiate a token: %s", err)
		c.JSON(http.StatusForbidden, gin.H{"status": ErrAuthFailed})
		return
	}
	res.Token = tok
	res.TokenExpires = timeout

	// Remarshal the body back into JSON
	pl := map[string]interface{}{}
	err = json.Unmarshal(body, &pl)
	res.Body = pl
	if err != nil {
		log.Printf("Failed to re-parse body: %s", err)
		c.JSON(http.StatusBadRequest, gin.H{"status": "Our parser is probably broken"})
		return
	}

	payload, err := json.Marshal(res)
	if err != nil {
		log.Print(err)
		c.JSON(http.StatusInternalServerError, gin.H{"status": "JSON encoding error"})
		return
	}
	s.build(brigEvent, rev, payload, proj)
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
	// Used only for check suite
	var pre *github.PullRequestEvent

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
		pre = e

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

	if proj.SharedSecret == "" {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "No secret is configured for this repo."})
		return
	}

	signature := c.Request.Header.Get(hubSignatureHeader)
	if err := validateSignature(signature, proj.SharedSecret, body); err != nil {
		c.JSON(http.StatusForbidden, gin.H{"status": "malformed signature"})
		return
	}

	// If s.opts.CheckSuiteOnPR is set, this will create a new check suite request.
	if eventType == "pull_request" && s.opts.CheckSuiteOnPR {
		if err := s.prToCheckSuite(c, pre, proj); err != nil {
			if err == ErrAuthFailed {
				c.JSON(http.StatusForbidden, gin.H{"status": err.Error()})
			}
			c.JSON(http.StatusInternalServerError, gin.H{"status": err.Error()})
			return
		}
		// TODO: do we return here (e.g. stop the PR hook) if we get to this point
	}

	s.build(eventType, rev, body, proj)
	c.JSON(http.StatusOK, gin.H{"status": "Complete"})
}

// prToCheckSuite creates a new check suite and rerequests it based on a pull request.
//
// The Check Suite webhook events are normally only triggered on `push` events. This function acts as an
// adapter to take a PR and trigger a check suite.
//
// The GitHub API is still evolving, so the current way we do this is...
//
//	- generate auth tokens for the instance/app combo. This is required to perform the action as a
//		GitHub app
//	- try to create a check_suite
//		- if success, run a `rerequest` on this check suite because merely creating a check suite does
// 		  not actually trigger a check_suite:requested webhook event
//		- if failure, check to see if we already have a check suite object, and merely run the rerequest
//		  on that check suite.
func (s *githubHook) prToCheckSuite(c *gin.Context, pre *github.PullRequestEvent, proj *brigade.Project) error {
	repo := pre.Repo.GetFullName()
	ref := fmt.Sprintf("refs/pull/%d/head", pre.PullRequest.GetNumber())
	sha := pre.PullRequest.Head.GetSHA()
	appID := s.opts.AppID
	instID := pre.Installation.GetID()

	if appID == 0 || instID == 0 {
		log.Printf("App ID and Installation ID must both be set. App: %d, Installation: %d", appID, instID)
		return ErrAuthFailed
	}

	tok, _, err := s.installationToken(int(appID), int(instID), proj.Github)
	if err != nil {
		log.Printf("Failed to negotiate a token: %s", err)
		return ErrAuthFailed
	}
	client, err := InstallationTokenClient(tok, proj.Github.BaseURL, proj.Github.UploadURL)
	if err != nil {
		log.Printf("Failed to create a new installation token client: %s", err)
		return ErrAuthFailed
	}

	projectNames := strings.Split(repo, "/")
	if len(projectNames) != 2 {
		log.Printf("Repo %q is invalid. Should be github.com/ORG/NAME.", repo)
		return errors.New("invalid repo name")
	}
	owner, pname := projectNames[0], projectNames[1]
	csOpts := github.CreateCheckSuiteOptions{
		HeadSHA:    sha,
		HeadBranch: &ref,
	}
	log.Printf("requesting check suite run for %s/%s, SHA: %s", owner, pname, csOpts.HeadSHA)

	cs, res, err := client.Checks.CreateCheckSuite(context.Background(), owner, pname, csOpts)
	if err != nil {
		log.Printf("Failed to create check suite: %s", err)

		// 422 means the suite already exists.
		if res.StatusCode != 422 {
			return errors.New("could not create check suite")
		}

		log.Println("rerunning the last suite")
		csl, _, err := client.Checks.ListCheckSuitesForRef(context.Background(), owner, pname, sha, &github.ListCheckSuiteOptions{
			AppID: &s.opts.AppID,
		})
		if err == nil && csl.GetTotal() > 0 {
			log.Printf("Loading check suite %d", csl.CheckSuites[0].GetID())
			_, err := client.Checks.ReRequestCheckSuite(context.Background(), owner, pname, csl.CheckSuites[0].GetID())
			if err != nil {
				log.Printf("error rerunning suite: %s", err)
			}
		} else {
			log.Printf("error fetching check suites: %s", err)
		}
		return nil
	}

	log.Printf("Created check suite for %s with ID %d. Triggering :rerequested", ref, cs.GetID())
	// It appears that merely creating the check suite does not trigger a check_suite:request.
	// So we manually trigger a rerequest.
	_, err = client.Checks.ReRequestCheckSuite(context.Background(), owner, pname, cs.GetID())
	return err
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

func getFileFromGithub(commit, path string, proj *brigade.Project) ([]byte, error) {
	return GetFileContents(proj, commit, path)
}

func (s *githubHook) build(eventType string, rev brigade.Revision, payload []byte, proj *brigade.Project) error {
	b := &brigade.Build{
		ProjectID: proj.ID,
		Type:      eventType,
		Provider:  "github",
		Revision:  &rev,
		Payload:   payload,
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

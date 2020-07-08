package webhook

import (
	"bytes"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/go-github/v32/github"
	gin "gopkg.in/gin-gonic/gin.v1"

	"github.com/brigadecore/brigade/pkg/brigade"
	"github.com/brigadecore/brigade/pkg/storage"
)

type testStore struct {
	proj   *brigade.Project
	builds []*brigade.Build
	err    error
	storage.Store
}

func (s *testStore) GetProject(name string) (*brigade.Project, error) {
	return s.proj, s.err
}

func (s *testStore) CreateBuild(build *brigade.Build) error {
	s.builds = append(s.builds, build)
	return s.err
}

func newTestStore() *testStore {
	return &testStore{
		proj: &brigade.Project{
			Name:         "baxterthehacker/public-repo",
			SharedSecret: "asdf",
		},
	}
}

func newTestGithubHandler(store storage.Store, t *testing.T) *githubHook {
	return &githubHook{
		store:          store,
		allowedAuthors: []string{"OWNER"},
		updateIssueCommentEvent: func(c *gin.Context, s *githubHook, ice *github.IssueCommentEvent, rev brigade.Revision, proj *brigade.Project, body []byte) (brigade.Revision, []byte) {
			revision := brigade.Revision{
				Commit: "0d1a26e67d8f5eaf1f6ba5c57fc3c7d91ac0fd1c",
				Ref:    "refs/pull/2/head",
			}
			return revision, []byte{}
		},
		opts: GithubOpts{
			EmittedEvents: []string{"*"},
		},
	}
}

func TestGithubHandler(t *testing.T) {

	tests := []struct {
		event          string
		commit         string
		ref            string
		payloadFile    string
		mustFail       bool
		expectedBuilds []string
	}{
		{
			event:          "commit_comment",
			commit:         "9049f1265b7d61be4a8904a9a27120d2064dab3b",
			payloadFile:    "testdata/github-commit_comment-payload.json",
			expectedBuilds: []string{"commit_comment", "commit_comment:created"},
		},
		{
			event:          "create",
			ref:            "0.0.1",
			payloadFile:    "testdata/github-create-payload.json",
			expectedBuilds: []string{"create"},
		},
		{
			event:          "deployment",
			commit:         "9049f1265b7d61be4a8904a9a27120d2064dab3b",
			ref:            "master",
			payloadFile:    "testdata/github-deployment-payload.json",
			expectedBuilds: []string{"deployment"},
		},
		{
			event:          "deployment_status",
			commit:         "9049f1265b7d61be4a8904a9a27120d2064dab3b",
			ref:            "master",
			payloadFile:    "testdata/github-deployment_status-payload.json",
			expectedBuilds: []string{"deployment_status"},
		},
		{
			event:          "issue_comment",
			commit:         "",
			ref:            "refs/heads/master",
			payloadFile:    "testdata/github-issue_comment-payload.json",
			expectedBuilds: []string{"issue_comment", "issue_comment:created"},
		},
		{
			event:          "issue_comment",
			commit:         "",
			ref:            "refs/heads/master",
			payloadFile:    "testdata/github-issue_comment_pull_request_comment_deleted-payload.json",
			expectedBuilds: []string{"issue_comment", "issue_comment:deleted"},
		},
		{
			event:          "issue_comment",
			commit:         "",
			ref:            "refs/heads/master",
			payloadFile:    "testdata/github-issue_comment_pull_request_author_not_allowed-payload.json",
			expectedBuilds: []string{"issue_comment", "issue_comment:edited"},
		},
		{
			event:          "issue_comment",
			commit:         "0d1a26e67d8f5eaf1f6ba5c57fc3c7d91ac0fd1c",
			ref:            "refs/pull/2/head",
			payloadFile:    "testdata/github-issue_comment_pull_request_author_allowed-payload.json",
			expectedBuilds: []string{"issue_comment", "issue_comment:edited"},
		},
		{
			event:       "pull_request",
			commit:      "0d1a26e67d8f5eaf1f6ba5c57fc3c7d91ac0fd1c",
			ref:         "refs/pull/1/head",
			payloadFile: "testdata/github-pull_request-payload-failed-perms.json",
			mustFail:    true,
		},
		{
			event:          "pull_request",
			ref:            "refs/pull/1/head",
			commit:         "0d1a26e67d8f5eaf1f6ba5c57fc3c7d91ac0fd1c",
			payloadFile:    "testdata/github-pull_request-payload.json",
			expectedBuilds: []string{"pull_request", "pull_request:opened"},
		},
		{
			event:          "pull_request",
			commit:         "ad0703ac08e80448764b34dc089d0f73a1242ae9",
			ref:            "refs/pull/1/head",
			payloadFile:    "testdata/github-pull_request-labeled-payload.json",
			expectedBuilds: []string{"pull_request", "pull_request:labeled"},
		},
		{
			event:          "pull_request_review",
			commit:         "b7a1f9c27caa4e03c14a88feb56e2d4f7500aa63",
			ref:            "refs/pull/8/head",
			payloadFile:    "testdata/github-pull_request_review-payload.json",
			expectedBuilds: []string{"pull_request_review", "pull_request_review:submitted"},
		},
		{
			event:          "pull_request_review_comment",
			commit:         "34c5c7793cb3b279e22454cb6750c80560547b3a",
			ref:            "refs/pull/1/head",
			payloadFile:    "testdata/github-pull_request_review_comment-payload.json",
			expectedBuilds: []string{"pull_request_review_comment", "pull_request_review_comment:created"},
		},
		{
			event:          "push",
			commit:         "0d1a26e67d8f5eaf1f6ba5c57fc3c7d91ac0fd1c",
			ref:            "refs/heads/changes",
			payloadFile:    "testdata/github-push-payload.json",
			expectedBuilds: []string{"push"},
		},
		{
			event:       "push",
			commit:      "0d1a26e67d8f5eaf1f6ba5c57fc3c7d91ac0fd1c",
			payloadFile: "testdata/github-push-delete-branch.json",
			mustFail:    true,
		},
		{
			event:          "status",
			commit:         "9049f1265b7d61be4a8904a9a27120d2064dab3b",
			payloadFile:    "testdata/github-status-payload.json",
			expectedBuilds: []string{"status"},
		},
		{
			event:          "release",
			ref:            "0.0.1",
			payloadFile:    "testdata/github-release-payload.json",
			expectedBuilds: []string{"release", "release:published"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.payloadFile, func(t *testing.T) {
			store := newTestStore()
			s := newTestGithubHandler(store, t)

			payload, err := ioutil.ReadFile(tt.payloadFile)
			if err != nil {
				t.Fatalf("failed to read testdata: %s", err)
			}

			w := httptest.NewRecorder()
			r, err := http.NewRequest("POST", "", bytes.NewReader(payload))
			if err != nil {
				t.Fatalf("failed to create request: %s", err)
			}
			r.Header.Add("X-GitHub-Event", tt.event)
			r.Header.Add("X-Hub-Signature", SHA1HMAC([]byte("asdf"), payload))

			ctx, _ := gin.CreateTestContext(w)
			ctx.Request = r

			s.Handle(ctx)

			if w.Code != http.StatusOK {
				t.Fatalf("unexpected error: %d\n%s", w.Code, w.Body.String())
			}

			// The build should not store anything if mustFail is true.
			if tt.mustFail {
				if len(store.builds) > 0 {
					t.Fatalf("expected failed hook for %s.", tt.payloadFile)
				}
				return
			}

			if len(store.builds) != len(tt.expectedBuilds) {
				t.Fatalf(
					"expected %d build(s) but %d build(s) were created",
					len(tt.expectedBuilds),
					len(store.builds),
				)
			}
			for i, build := range store.builds {
				if build.Type != tt.expectedBuilds[i] {
					t.Errorf(
						"store.builds[%d].Type is not correct. Expected %q, got %q",
						i,
						tt.expectedBuilds[i],
						build.Type,
					)
				}
				if build.Provider != "github" {
					t.Errorf("store.builds[%d].Provider is not correct", i)
				}
				if build.Revision.Commit != tt.commit {
					t.Errorf("store.builds[%d].Commit is not correct", i)
				}
				if build.Revision.Ref != tt.ref {
					t.Errorf(
						"store.builds[%d].Commit is not correct. Expected ref %q, got %q",
						i,
						tt.ref,
						build.Revision.Ref,
					)
				}
			}
		})
	}
}

func TestGithubHandler_ping(t *testing.T) {
	store := newTestStore()
	s := newTestGithubHandler(store, t)

	w := httptest.NewRecorder()
	r, err := http.NewRequest("POST", "", nil)
	if err != nil {
		t.Fatalf("failed to create request: %s", err)
	}
	r.Header.Add("X-GitHub-Event", "ping")

	ctx, _ := gin.CreateTestContext(w)
	ctx.Request = r

	s.Handle(ctx)

	if w.Code != http.StatusOK {
		t.Fatalf("unexpected error: %d\n%s", w.Code, w.Body.String())
	}
}

func TestGithubHandler_badevent(t *testing.T) {
	store := newTestStore()
	s := newTestGithubHandler(store, t)

	w := httptest.NewRecorder()
	r, err := http.NewRequest("POST", "", nil)
	if err != nil {
		t.Fatalf("failed to create request: %s", err)
	}
	r.Header.Add("X-GitHub-Event", "funzone")

	ctx, _ := gin.CreateTestContext(w)
	ctx.Request = r

	s.Handle(ctx)

	if w.Code != http.StatusOK {
		t.Fatalf("expected unsupported verb to return a 200, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "Ignored") {
		t.Fatalf("unexpected body: %d\n%s", w.Code, w.Body.String())
	}
}

func TestGithubHandler_shouldEmit(t *testing.T) {
	tests := []struct {
		event    string
		pattern  string
		expected bool
	}{
		{
			event:    "issue_comment",
			pattern:  "*",
			expected: true,
		},
		{
			event:    "issue_comment:created",
			pattern:  "*",
			expected: true,
		},
		{
			event:    "issue_comment",
			pattern:  "issue_comment",
			expected: true,
		},
		{
			event:    "issue_comment",
			pattern:  "issue_comment:created",
			expected: false,
		},
		{
			event:    "issue_comment:created",
			pattern:  "issue_comment",
			expected: true,
		},
		{
			event:    "issue_comment:created",
			pattern:  "issue_comment:created",
			expected: true,
		},
	}

	for i := range tests {
		tt := tests[i]
		t.Run(tt.event+"/"+tt.pattern, func(t *testing.T) {
			s := &githubHook{
				opts: GithubOpts{
					EmittedEvents: []string{tt.pattern},
				},
			}

			actual := s.shouldEmit(tt.event)

			if actual != tt.expected {
				t.Fatalf("unexpected result: pattern=%s, event=%s, expected=%v, actual=%v", tt.pattern, tt.event, tt.expected, actual)
			}
		})
	}
}

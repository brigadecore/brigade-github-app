package webhook

import (
	"encoding/json"
	"github.com/brigadecore/brigade/pkg/brigade"
	"github.com/google/go-github/github"
)

type buildOpts struct {
	tok     string
	issueId int
}

func (s *githubHook) icePayloadToBuildOpts(ice *github.IssueCommentEvent, proj *brigade.Project, payload []byte) buildOpts {
	opts := buildOpts{}
	if ice != nil {
		// Reuse the installation token generated for the payload if exists
		if len(payload) > 0 {
			res := Payload{}
			_ = json.Unmarshal(payload, &res)
			if res.Token != "" {
				opts.tok = res.Token
			}
		} else {
			tok, _, _ := s.iceToIntsallationToken(ice, proj)
			opts.tok = tok
		}

		opts.issueId = int(ice.GetIssue().GetID())
	}
	return opts
}

func (s *githubHook) preToBuildOpts(pre *github.PullRequestEvent, proj *brigade.Project) buildOpts {
	opts := buildOpts{}
	if pre != nil {
		tok, _, _ := s.prToInstallationToken(pre, proj)
		if tok != "" {
			opts.tok = tok
		}

		opts.issueId = int(pre.GetPullRequest().GetID())
	}
	return opts
}

func (s *githubHook) checkEventToBuildOpts(e interface{}, tok string) buildOpts {
	opts := buildOpts{
		tok: tok,
	}
	switch e := e.(type) {
	case *github.CheckSuiteEvent:
		opts.issueId = int(e.GetCheckSuite().PullRequests[0].GetID())
	case *github.CheckRunEvent:
		opts.issueId = int(e.GetCheckRun().PullRequests[0].GetID())
	}
	return opts
}

package webhook

import (
	"time"

	"github.com/brigadecore/brigade/pkg/brigade"
	"github.com/google/go-github/github"
)

func (s *githubHook) prToInstallationToken(pre *github.PullRequestEvent, proj *brigade.Project) (string, *time.Time, error) {
	appID := s.opts.AppID
	if appID == 0 {
		appID = s.opts.AppID
	}

	instID := pre.Installation.GetID()

	tok, timeout, err := s.getInstallationToken(appID, int(instID), proj)

	return tok, &timeout, err
}

func (s *githubHook) iceToIntsallationToken(ice *github.IssueCommentEvent, proj *brigade.Project) (string, *time.Time, error) {
	appID := int(ice.Installation.GetAppID())
	if appID == 0 {
		appID = s.opts.AppID
	}

	instID := ice.Installation.GetID()

	tok, timeout, err := s.getInstallationToken(appID, int(instID), proj)

	return tok, &timeout, err
}

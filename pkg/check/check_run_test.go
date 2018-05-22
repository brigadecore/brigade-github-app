package check

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

// in_progress is an example body supplied by GitHub
// https://developer.github.com/v3/checks/runs/#create-a-check-run
const in_progress = `
{
    "head_branch": "master",
    "name": "mighty_readme",
    "head_sha": "ce587453ced02b1526dfb4cb910479d431683101",
    "status": "in_progress",
    "external_id": "42",
    "started_at": "2018-05-04T01:14:52Z",
    "output": {
        "title": "Mighty Readme report",
        "summary": "",
        "text": ""
    }
}
`

func TestHelloWorld(t *testing.T) {
	is := assert.New(t)
	cr := &Run{}
	if err := json.Unmarshal([]byte(in_progress), cr); err != nil {
		t.Fatal(err)
	}

	is.Equal(cr.HeadBranch, "master")
	is.Equal(cr.Name, "mighty_readme")
	is.Equal(cr.HeadSHA, "ce587453ced02b1526dfb4cb910479d431683101")
	is.Equal(cr.Status, "in_progress")
	is.Equal(cr.ExternalID, "42")
	is.Equal(cr.StartedAt, "2018-05-04T01:14:52Z")

	is.Equal(cr.Output.Title, "Mighty Readme report")
	is.Equal(cr.Output.Summary, "")
	is.Equal(cr.Output.Text, "")
}

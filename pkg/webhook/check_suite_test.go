package webhook

import (
	"encoding/json"
	"io/ioutil"
	"testing"

	"github.com/stretchr/testify/assert"
)

const checkSuiteFile = "testdata/github-check_suite-payload.json"

func TestCheckSuite(t *testing.T) {
	// Our CheckSuite structs only carry fields we need. This is a test to make
	// sure those fields are present, based on input test data.
	data, err := ioutil.ReadFile(checkSuiteFile)
	if err != nil {
		t.Fatal(err)
	}

	cs := &EventCheckSuite{}
	if err := json.Unmarshal(data, cs); err != nil {
		t.Fatal(err)
	}

	is := assert.New(t)
	is.Equal(cs.CheckSuite.App.ID, 12345, "App ID is set to 12345")
	is.Equal(cs.Installation.ID, 777777, "Installation ID is set")

}

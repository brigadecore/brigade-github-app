package webhook

// EventCheckSuite is a placeholder for JSON unmarshalling.
// This will be replaced when the Go GitHub library catches up.
type EventCheckSuite struct {
	Action       string       `json:"action"`
	CheckSuite   CheckSuite   `json:"check_suite"`
	Repo         Repository   `json:"repository"`
	Installation Installation `json:"installation"`
}

// EventCheckRun is a placeholder for JSON unmarshalling.
// This will be replaced when the Go GitHub library catches up.
type EventCheckRun struct {
	Action   string `json:"action"`
	CheckRun struct {
		Name       string     `json:"name"`
		HeadSHA    string     `json:"head_sha"`
		CheckSuite CheckSuite `json:"check_suite"`
		App        App        `json:"app"`
	} `json:"check_run"`
	Repo         Repository   `json:"repository"`
	Installation Installation `json:"installation"`
}

// CheckSuite is a placeholder for JSON unmarshalling.
// This will be replaced when the Go GitHub library catches up.
type CheckSuite struct {
	HeadBranch string `json:"head_branch"`
	HeadSHA    string `json:"head_sha"`
	App        App    `json:"app"`
}

// Respository is a placeholder for JSON unmarshalling.
// This will be replaced when the Go GitHub library catches up.
type Repository struct {
	FullName string `json:"full_name"`
}

// App represents a GitHub App
type App struct {
	ID int `json:"id"`
}

// Installation represents a GitHub App's installation in a particular project
type Installation struct {
	ID int `json:"id"`
}

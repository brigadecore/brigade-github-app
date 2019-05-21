// This example shows how to kick off Check runs from Issue comments
// that are associated with Pull Requests.
//
// For instance, to manually trigger a Check run on a PR from a
// normally non-allowed commit author, or to re-trigger a Check run
// for any PR.

const {events, Job, Group} = require("brigadier");
const checkRunImage = "brigadecore/brigade-github-check-run:latest";

// Here are our event handlers, for handling check_suite, check_run
// and issue_comment event:action pairs
events.on("check_suite:requested", checkSuiteRequested);
events.on("check_suite:rerequested", checkSuiteRequested);
events.on("check_run:rerequested", checkRequested);
events.on("issue_comment:created", handleIssueComment);
events.on("issue_comment:edited", handleIssueComment);

// build runs our build job
function build(e, p) {
  return new Job("build", "alpine:3.7", ["sleep 10", "echo building!"]);
}

// test runs our test job
function test(e, p) {
  return new Job("test", "alpine:3.7", ["sleep 10", "echo testing!"]);
}

// Check represents a simple Check run,
// consisting of a name and an action (javascript function)
class Check {
  constructor(name, action) {
    this.name = name;
    this.action = action;
  }
}

// Checks represent a list of Checks that by default are run in the form
// of a check suite, but may be run individually
Checks = {
  "build": new Check("build", build),
  "test":  new Check("test", test)
};

// CommentChecks represent a list of mappings that match a comment
// to a corresponding Check
CommentChecks = {
  "/build": Checks["build"],
  "/test": Checks["test"]
};

// handleIssueComment handles an issue_comment event,
// parsing the comment text and determining whether or not a Check
// was requested
function handleIssueComment(e, p) {
  payload = JSON.parse(e.payload);

  comment = payload.body.comment.body;
  check = CommentChecks[comment]

  if (typeof check !== 'undefined') {
    checkRun(e, p, check);
  } else {
    console.log(`No action found for comment: ${comment}`);
  }
}

// checkRequested is the default function invoked on a check_run:* event
//
// It determines which check is being requested (from the payload body)
// and runs this particular check, or else throws an error if the check
// is not found
function checkRequested(e, p) {
  payload = JSON.parse(e.payload);

  name = payload.body.check_run.name;
  check = Checks[name];

  if (typeof check !== 'undefined') {
    checkRun(e, p, check);
  } else {
    err = new Error(`No check found with name: ${name}`);
    throw err;
  }
}

// checkSuiteRequested is the default function invoked on a check_suite:* event
//
// It loops over our standard set of checks and runs them in parallel
// such that they may report their results independently to GitHub
function checkSuiteRequested(e, p) {
  for (check of Object.values(Checks)) {
    checkRun(e, p, check);
  }
}

// checkRun is a GitHub Check Run, running the provided check,
// wrapped in notification jobs to update GitHub along the way
function checkRun(e, p, check) {
  console.log(`Check requested: ${check.name}`);

  // Common configuration
  const env = {
    CHECK_PAYLOAD: e.payload,
    CHECK_NAME: check.name,
    CHECK_TITLE: `Run ${check.name}`
  };

  // For convenience, we'll create three jobs: one for each GitHub Check stage.
  const start = new Job(`start-${check.name}`, checkRunImage);
  start.imageForcePull = true;
  start.env = env;
  start.env.CHECK_SUMMARY = `Running ${check.name} for ${e.revision.commit}`;

  const end = new Job(`end-${check.name}`, checkRunImage);
  end.imageForcePull = true;
  end.env = env;

  // Now we run the jobs in order:
  // - Notify GitHub of start
  // - Run the check
  // - Notify GitHub of completion
  //
  // On error, we catch the error and notify GitHub of a failure.
  start.run().then(() => {
    return check.action(e, p).run();
  }).then( (result) => {
    end.env.CHECK_CONCLUSION = "success";
    end.env.CHECK_SUMMARY = `${check.name} completed`;
    end.env.CHECK_TEXT = result.toString();
    return end.run();
  }).catch( (err) => {
    // In this case, we mark the ending failed.
    end.env.CHECK_CONCLUSION = "failure";
    end.env.CHECK_SUMMARY = `${check.name} failed`;
    end.env.CHECK_TEXT = `Error: ${ err }`;
    return end.run();
  })
}
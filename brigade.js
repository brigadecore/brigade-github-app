const { events, Job, Group } = require("brigadier")

const projectOrg = "brigadecore"
const projectName = "brigade-github-app"

const goImg = "golang:1.11"
const gopath = "/go"
const localPath = gopath + `/src/github.com/${projectOrg}/${projectName}`;

const images = [
  "brigade-github-app",
  "brigade-github-check-run"
]

const noop = {run: () => {return Promise.resolve()}}

function build(e, project) {
  // Create a new job to run Go tests
  var build = new Job(`${projectName}-build`, goImg);

  // Set a few environment variables.
  build.env = {
      "DEST_PATH": localPath,
      "GOPATH": gopath
  };

  // Run Go unit tests
  build.tasks = [
    // Need to move the source into GOPATH so vendor/ works as desired.
    `mkdir -p ${localPath}`,
    `cp -a /src/* ${localPath}`,
    `cp -a /src/.git ${localPath}`,
    `cd ${localPath}`,
    "make bootstrap",
    "make lint",
    "make test"
  ];

  return build
}

function goDockerBuild(project, tag) {
  // We build in a separate pod b/c AKS's Docker is too old to do multi-stage builds.
  const goBuild = new Job(`${projectName}-docker-build`, goImg);

  goBuild.storage.enabled = true;
  goBuild.env = {
    "DEST_PATH": localPath,
    "GOPATH": gopath
  };
  goBuild.tasks = [
    `cd /src && git checkout ${tag}`,
    `mkdir -p ${localPath}/bin`,
    `mv /src/* ${localPath}`,
    `cd ${localPath}`,
    "make build-docker-bins",
    // create share and copy binaries, for use by the dockerhubPublish job
    `mkdir -p /mnt/brigade/share/rootfs`,
    `cp -a ./rootfs/* /mnt/brigade/share/rootfs/`,
    "ls -lah /mnt/brigade/share"
  ];

  return goBuild;
}

function dockerhubPublish(project, tag) {
  const publisher = new Job("dockerhub-publish", "docker");
  let dockerRegistry = project.secrets.dockerhubRegistry || "docker.io";
  let dockerOrg = project.secrets.dockerhubOrg || "brigadecore";

  publisher.docker.enabled = true;
  publisher.storage.enabled = true;
  publisher.tasks = [
    "apk add --update --no-cache make",
    `docker login ${dockerRegistry} -u ${project.secrets.dockerhubUsername} -p ${project.secrets.dockerhubPassword}`,
    "cd /src"
  ];

  for (let i of images) {
      publisher.tasks.push(
        `cp -av /mnt/brigade/share/rootfs ./rootfs`,
        `SHELL=/bin/sh DOCKER_REGISTRY=${dockerOrg} VERSION=${tag} make ${i}-image ${i}-push`
      );
  }
  publisher.tasks.push(`docker logout ${dockerRegistry}`);

  return publisher;
}

// Here we can add additional Check Runs, which will run in parallel and
// report their results independently to GitHub
function runSuite(e, p) {
  // For now, this is the one-stop shop running build, lint and test targets
  runTests(e, p).catch(err => {console.error(err.toString())});
}

// runTests is a Check Run that is ran as part of a Checks Suite
function runTests(e, p) {
  console.log("Check requested")

  // Create Notification object (which is just a Job to update GH using the Checks API)
  var note = new Notification(`tests`, e, p);
  note.conclusion = "";
  note.title = "Run Tests";
  note.summary = "Running the test targets for " + e.revision.commit;
  note.text = "This test will ensure build, linting and tests all pass."

  // Send notification, then run, then send pass/fail notification
  return notificationWrap(build(e, p), note)
}

// A GitHub Check Suite notification
class Notification {
  constructor(name, e, p) {
      this.proj = p;
      this.payload = e.payload;
      this.name = name;
      this.externalID = e.buildID;
      this.detailsURL = `https://brigadecore.github.io/kashti/builds/${ e.buildID }`;
      this.title = "running check";
      this.text = "";
      this.summary = "";

      // count allows us to send the notification multiple times, with a distinct pod name
      // each time.
      this.count = 0;

      // One of: "success", "failure", "neutral", "cancelled", or "timed_out".
      this.conclusion = "neutral";
  }

  // Send a new notification, and return a Promise<result>.
  run() {
      this.count++
      var j = new Job(`${ this.name }-${ this.count }`, "brigadecore/brigade-github-check-run:latest");
      j.imageForcePull = true;
      j.env = {
          CHECK_CONCLUSION: this.conclusion,
          CHECK_NAME: this.name,
          CHECK_TITLE: this.title,
          CHECK_PAYLOAD: this.payload,
          CHECK_SUMMARY: this.summary,
          CHECK_TEXT: this.text,
          CHECK_DETAILS_URL: this.detailsURL,
          CHECK_EXTERNAL_ID: this.externalID
      }
      return j.run();
  }
}

// Helper to wrap a job execution between two notifications.
async function notificationWrap(job, note, conclusion) {
  if (conclusion == null) {
      conclusion = "success"
  }
  await note.run();
  try {
      let res = await job.run()
      const logs = await job.logs();

      note.conclusion = conclusion;
      note.summary = `Task "${ job.name }" passed`;
      note.text = note.text = "```" + res.toString() + "```\nTest Complete";
      return await note.run();
  } catch (e) {
      const logs = await job.logs();
      note.conclusion = "failure";
      note.summary = `Task "${ job.name }" failed for ${ e.buildID }`;
      note.text = "```" + logs + "```\nFailed with error: " + e.toString();
      try {
          return await note.run();
      } catch (e2) {
          console.error("failed to send notification: " + e2.toString());
          console.error("original error: " + e.toString());
          return e2;
      }
  }
}

events.on("exec", (e, p) => {
  return build(e, p).run()
})

events.on("push", (e, p) => {
  let release = false;
  let gitTag = "";
  let imageTag = "";

  if (e.revision.ref.includes("refs/heads/master")) {
    release = true;
    gitTag = "master"
    imageTag = "latest"
  } else if (e.revision.ref.startsWith("refs/tags/")) {
    release = true;
    let parts = e.revision.ref.split("/", 3)
    gitTag = parts[2]
    imageTag = gitTag
  }

  if (release) {
    return Group.runEach([
      goDockerBuild(p, gitTag),
      dockerhubPublish(p, imageTag)
    ]);
  }
})

events.on("check_suite:requested", runSuite)
events.on("check_suite:rerequested", runSuite)
events.on("check_run:rerequested", runSuite)

events.on("release_images", (e, p) => {
  /*
   * Expects JSON of the form {'tag': 'v1.2.3'}
   */
  payload = JSON.parse(e.payload)
  if (!payload.tag) {
    throw error("No tag specified")
  }

  Group.runEach([
    goDockerBuild(p, payload.tag),
    dockerhubPublish(p, payload.tag)
  ]);
})

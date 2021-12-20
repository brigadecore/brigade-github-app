import { Container, Event, events, Job, SerialGroup } from "@brigadecore/brigadier"

const goImg = "brigadecore/go-tools:v0.1.0"
const dindImg = "docker:20.10.9-dind"
const dockerClientImg = "brigadecore/docker-tools:v0.1.0"

const projectOrg = "brigadecore";
const projectName = "brigade-github-app";
const gopath = "/go"
const localPath = gopath + `/src/github.com/${projectOrg}/${projectName}`

// A map of all jobs. When a check_run:rerequested event wants to re-run a
// single job, this allows us to easily find that job by name.
const jobs: { [key: string]: (event: Event) => Job } = {}

const testJobName = "test"
const testJob = (event: Event) => {
	const job = new Job(testJobName, goImg, event)
	job.primaryContainer.environment = {
		"SKIP_DOCKER": "true"
	}
	job.primaryContainer.sourceMountPath = localPath
	job.primaryContainer.workingDirectory = localPath
	job.primaryContainer.command = ["make"]
	job.primaryContainer.arguments = ["verify-vendored-code", "lint", "test"]
	return job
}
jobs[testJobName] = testJob


const buildAndPublishImagesJobName = "build-and-publish-images"
const buildAndPublishImagesJob = (event: Event, version?: string) => {
	let dockerRegistry = event.project.secrets.dockerhubRegistry || "docker.io"
	const job = new Job(buildAndPublishImagesJobName, dockerClientImg, event)
	job.primaryContainer.sourceMountPath = localPath
	job.primaryContainer.workingDirectory = localPath
	job.primaryContainer.environment = {
		"DOCKER_HOST": "localhost:2375",
		"DOCKER_REGISTRY": dockerRegistry,
		"DOCKER_ORG": event.project.secrets.dockerhubOrg || "brigadecore",
		"DOCKER_PASSWORD": event.project.secrets.dockerhubPassword
	}
	if (version) {
		job.primaryContainer.environment["VERSION"] = version
	}
	job.primaryContainer.command = ["sh"]
	job.primaryContainer.arguments = [
		"-c",
		// The sleep is a grace period after which we assume the DinD sidecar is
		// probably up and running.
		`sleep 20 && docker login ${dockerRegistry} -u ${event.project.secrets.dockerhubUsername} -p $DOCKER_PASSWORD && make build-all-images push-all-images`
	]
	job.sidecarContainers.docker = new Container(dindImg)
	job.sidecarContainers.docker.privileged = true
	job.sidecarContainers.docker.environment.DOCKER_TLS_CERTDIR = ""
	return job
}
jobs[buildAndPublishImagesJobName] = buildAndPublishImagesJob

async function runSuite(event: Event): Promise<void> {
	await testJob(event).run()
	if (event.worker?.git?.ref == "main") {
		await buildAndPublishImagesJob(event).run()
	}
}

// Either of these events should initiate execution of the entire test suite.
events.on("brigade.sh/github", "check_suite:requested", runSuite)
events.on("brigade.sh/github", "check_suite:rerequested", runSuite)

// This event indicates a specific job is to be re-run.
events.on("brigade.sh/github", "check_run:rerequested", async event => {
	// Check run names are of the form <project name>:<job name>, so we strip
	// event.project.id.length + 1 characters off the start of the check run name
	// to find the job name.
	const jobName = JSON.parse(event.payload).check_run.name.slice(event.project.id.length + 1)
	const job = jobs[jobName]
	if (job) {
		await job(event).run()
		return
	}
	throw new Error(`No job found with name: ${jobName}`)
})

events.on("brigade.sh/github", "release:published", async event => {
	const version = JSON.parse(event.payload).release.tag_name
	await buildAndPublishImagesJob(event, version).run()
})

events.process()

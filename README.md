# Brigade Github App: Advanced GitHub Gateway for Brigade

[![Stability: Experimental](https://masterminds.github.io/stability/experimental.svg)](https://masterminds.github.io/stability/experimental.html)

**This is considered experimental and pre-Alpha. Do not use it in production.**

This is a [Brigade](https://github.com/Azure/brigade) gateway that provides a
GitHub App with deep integration to GitHub's new Check API.

## Installation

The installation for this gateway is multi-part, and not particularly easy at
the moment.

Prerequisites:

- A Kubernetes cluster running Brigade
- kubectl and Helm
- A local clone of this repository

### 1. Install the chart into your cluster

You must install this gateway into the same namespace in your cluster where
Brigade is already running

**Make sure the gateway is accessibly on a public IP address**. You can do that
either by setting the Service to be a load balancer, or setting up the Ingress. We
STRONGLY recommend setting up an ingress to use Kube-LEGO or another SSL proxy.

```
$ cd brigade-github
$ helm inspect values ./charts/brigade-github-app > values.yaml
$ # Edit values.yaml
$ helm install -n gh-app ./charts/brigade-github-app
```

On RBAC-enabled clusters, pass `--set rbac.enabled=true` to the `helm install`
command.

### 2. (RECOMMENDED) Create a DNS entry for your app

Create a public DNS entry for the public IP of your service or ingress

### 3. Create a GitHub App

A GitHub app is a special kind of trusted entity that is associated with either
your account or an orgs account.

https://developer.github.com/apps/building-github-apps/creating-a-github-app/

- Set the _Homepage URL_ to `https://brigade.sh`
- Set the _User Authorization Callback URL_ to **FIXME**
- Set the _Webhook URL_ to `https://YOUR_DOMAIN/events/github`
- Set the _Webhook Secret_ to a randomly generated string. Make note of that string
- Subscribe to the following events:
  - Repository contents: read
  - Issues: read (TODO: verify this)
  - Repository metadata: read
  - Pull requests: read
  - Repository webhooks: read (TODO: verify this)
  - Commit Statuses: Read And Write
  - Checks: Read and Write
- Subscribe to the following webhooks:
  - push (TODO: verify. Might just need checks)
  - checks suite
- Choose "Only This Account" to connect to the app.

**Once you have submitted** you will be promted to create a private key. Create
one and save it locally.

### 4. Test the App from GitHub

Go to the _Advanced_ tab and chck out the _Recent Deliveries_ section. This should
show a successful test run. If it is not successful, you will need to troubleshoot
why GitHub could not successfully contact your app.

Likely reasons:

- Your app is not listening on a public IP
- SSL certificate is invalid
- The URL you entered is wrong (Go to the _General_ tab and fix it)
- The Brigade Github App is returning incorrect data

### 5. Install the App

Go to the _Install App_ tab and enable this app for your account.

Accept the permissions it asks for. You can choose between _All repos_ and
_Only select repositories_, and click _Install_

> It is easy to change from All Repos to Only Selected, and vice versa, so we
> recommend starting with one repo, and adding the rest later.

### 6. Add Brigade projects for each GitHub project

For each GitHub project that you enabled teh app for, you will now need to
create a Project.

Remember that projects contain secret data, and should be handled with care.


```
$ helm inspect values brigade/brigade-project > values.yaml
$ # Edit values.yaml
```

You will want to make sure to set:

- `project`, `repository`, and `cloneURL`  to point to your repo
- `sharedSecret` to use the shared secret you created when creating the app
- `github.token` (aka `github: {token: }`) to the OAuth token GitHub Apps gave you

## Events Emitted by this Gateway

- `check_suite:requested`: When a new request is opened
- `check_suite:rerequested`: When checks are requested again
- `check_suite:completed`: When a check suite is completed
- `check_run:created`: When an individual test is requested
- `check_run:updated`: When an individual test is updated with new status
- `check_run:rerequested`: When an individual test is re-requested

## Building From Source

Prerequisites:

- The Go tool chain
- `dep` for Go dependency management
- `make`
- Docker

To build from source:

```console
$ dep ensure         # to install dependencies into vendor/
$ make test          # to run tests
$ make build         # to build local
$ make docker-build  # to build a Docker image
```

## TODO

- [ ] Move images to correct Docker repo

# Contributing

This project welcomes contributions and suggestions.  Most contributions require you to agree to a
Contributor License Agreement (CLA) declaring that you have the right to, and actually do, grant us
the rights to use your contribution. For details, visit https://cla.microsoft.com.

When you submit a pull request, a CLA-bot will automatically determine whether you need to provide
a CLA and decorate the PR appropriately (e.g., label, comment). Simply follow the instructions
provided by the bot. You will only need to do this once across all repos using our CLA.

This project has adopted the [Microsoft Open Source Code of Conduct](https://opensource.microsoft.com/codeofconduct/).
For more information see the [Code of Conduct FAQ](https://opensource.microsoft.com/codeofconduct/faq/) or
contact [opencode@microsoft.com](mailto:opencode@microsoft.com) with any additional questions or comments.

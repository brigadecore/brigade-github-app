package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"

	"gopkg.in/gin-gonic/gin.v1"
	v1 "k8s.io/api/core/v1"

	"github.com/brigadecore/brigade/pkg/storage/kube"

	"github.com/brigadecore/brigade-github-app/pkg/webhook"
)

var (
	kubeconfig     string
	master         string
	namespace      string
	gatewayPort    string
	keyFile        string
	allowedAuthors authors
	emittedEvents  events

	reportBuildFailures bool
)

// defaultAllowedAuthors is the default set of authors allowed to PR
// https://developer.github.com/v4/reference/enum/commentauthorassociation/
var defaultAllowedAuthors = []string{"COLLABORATOR", "OWNER", "MEMBER"}

// defaultEmittedEvents is the default set of events to be emitted by the gateway
var defaultEmittedEvents = []string{"*"}

func init() {
	flag.StringVar(&kubeconfig, "kubeconfig", "", "absolute path to the kubeconfig file")
	flag.StringVar(&master, "master", "", "master url")
	flag.StringVar(&namespace, "namespace", defaultNamespace(), "kubernetes namespace")
	flag.StringVar(&gatewayPort, "gateway-port", defaultGatewayPort(), "TCP port to use for brigade-github-gateway")
	flag.StringVar(&keyFile, "key-file", "/etc/brigade-github-app/key.pem", "path to x509 key for GitHub app")
	flag.Var(&allowedAuthors, "authors", "allowed author associations, separated by commas (COLLABORATOR, CONTRIBUTOR, FIRST_TIMER, FIRST_TIME_CONTRIBUTOR, MEMBER, OWNER, NONE)")
	flag.Var(&emittedEvents, "events", "events to be emitted and passed to worker, separated by commas (defaults to `*`, which matches everything)")
	flag.BoolVar(&reportBuildFailures, "report-build-failures", false, "report build failures via issue comments")
}

func main() {
	flag.Parse()

	if len(keyFile) == 0 {
		log.Fatal("Key file is required")
		os.Exit(1)
	}

	key, err := ioutil.ReadFile(keyFile)
	if err != nil {
		log.Fatalf("could not load key from %q: %s", keyFile, err)
		os.Exit(1)
	}

	if len(allowedAuthors) == 0 {
		if aa, ok := os.LookupEnv("BRIGADE_AUTHORS"); ok {
			(&allowedAuthors).Set(aa)
		} else {
			allowedAuthors = defaultAllowedAuthors
		}
	}

	if len(allowedAuthors) > 0 {
		log.Printf("Forked PRs will be built for roles %s", strings.Join(allowedAuthors, " | "))
	}

	if len(emittedEvents) == 0 {
		if ee, ok := os.LookupEnv("BRIGADE_EVENTS"); ok {
			(&emittedEvents).Set(ee)
		} else {
			emittedEvents = defaultEmittedEvents
		}
	}

	envOrBool := func(env string, defaultVal bool) bool {
		s, ok := os.LookupEnv(env)
		if !ok {
			return defaultVal
		}

		realVal, err := strconv.ParseBool(s)
		if err != nil {
			return defaultVal
		}

		return realVal
	}

	envOrInt := func(env string, defaultVal int) int {
		aa, ok := os.LookupEnv(env)
		if !ok {
			return defaultVal
		}

		realVal, err := strconv.Atoi(aa)
		if err != nil {
			return defaultVal
		}
		return realVal
	}

	ghOpts := webhook.GithubOpts{
		CheckSuiteOnPR:      envOrBool("CHECK_SUITE_ON_PR", true),
		AppID:               envOrInt("APP_ID", 0),
		DefaultSharedSecret: os.Getenv("DEFAULT_SHARED_SECRET"),
		EmittedEvents:       emittedEvents,
		ReportBuildFailures: reportBuildFailures,
	}

	clientset, err := kube.GetClient(master, kubeconfig)
	if err != nil {
		log.Fatal(err)
	}

	store := kube.New(clientset, namespace)

	var reporter *webhook.BuildReporter
	if ghOpts.ReportBuildFailures {
		reporter = webhook.NewBuildReporter(clientset, store, namespace)
		stop := make(chan struct{})
		defer close(stop)
		go reporter.Run(1, stop)
	}

	hookHandler := webhook.NewGithubHookHandler(store, allowedAuthors, key, reporter, ghOpts)

	router := gin.New()
	router.Use(gin.Recovery())

	events := router.Group("/events")
	{
		events.Use(gin.Logger())
		events.POST("/github", hookHandler)
		events.POST("/github/:app/:inst", hookHandler)
	}

	router.GET("/healthz", healthz)

	formattedGatewayPort := fmt.Sprintf(":%v", gatewayPort)
	router.Run(formattedGatewayPort)
}

func defaultNamespace() string {
	if ns, ok := os.LookupEnv("BRIGADE_NAMESPACE"); ok {
		return ns
	}
	return v1.NamespaceDefault
}

func defaultGatewayPort() string {
	if port, ok := os.LookupEnv("BRIGADE_GATEWAY_PORT"); ok {
		return port
	}
	return "7746"
}

func healthz(c *gin.Context) {
	c.String(http.StatusOK, http.StatusText(http.StatusOK))
}

type authors []string

func (a *authors) Set(value string) error {
	for _, aa := range strings.Split(value, ",") {
		*a = append(*a, strings.ToUpper(aa))
	}
	return nil
}

func (a *authors) String() string {
	return strings.Join(*a, ",")
}

type events []string

func (a *events) Set(value string) error {
	for _, aa := range strings.Split(value, ",") {
		*a = append(*a, aa)
	}
	return nil
}

func (a *events) String() string {
	return strings.Join(*a, ",")
}

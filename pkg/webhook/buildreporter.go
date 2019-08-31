package webhook

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"

	"github.com/brigadecore/brigade/pkg/brigade"
	"github.com/brigadecore/brigade/pkg/storage"
	"github.com/google/go-github/github"
)

type BuildReporter struct {
	indexer    cache.Indexer
	queue      workqueue.RateLimitingInterface
	informer   cache.Controller
	store      storage.Store
	ns         string
	podToBuild map[string]*commentableBuild
}

func newBuildReporter(queue workqueue.RateLimitingInterface, indexer cache.Indexer, informer cache.Controller, store storage.Store, ns string) *BuildReporter {
	return &BuildReporter{
		informer:   informer,
		indexer:    indexer,
		queue:      queue,
		ns:         ns,
		store:      store,
		podToBuild: map[string]*commentableBuild{},
	}
}

func (c *BuildReporter) processNextBuildPodUpdate() bool {
	key, quit := c.queue.Get()
	if quit {
		return false
	}

	defer c.queue.Done(key)

	err := c.processBuildPod(key.(string))

	c.completeOrRetry(err, key)

	return true
}

func (c *BuildReporter) processBuildPod(key string) error {
	obj, exists, err := c.indexer.GetByKey(key)
	if err != nil {
		c.Logf("Fetching object with key %s from store failed with %v", key, err)
		return err
	}

	if exists {
		// Note that you also have to check the uid if you have a local controlled resource, which
		// is dependent on the actual instance, to detect that a Pod was recreated with the same name
		pod := obj.(*v1.Pod)
		fmt.Printf("processing pod %s\n", pod.GetName())

		phase := pod.Status.Phase
		switch phase {
		case "Running", "Succeeded", "Unknown", "Pending":
			return nil
		}

		if phase != "Failed" {
			return fmt.Errorf("unexpected phase: %s", phase)
		}

		ctx, ok := c.podToBuild[pod.Name]
		if !ok {
			fmt.Printf("skipping non-brigade pod %s\n", pod.GetName())
			return nil
		}

		msg := fmt.Sprintf("Build failed. Please run `brig build logs --init %s` to investigate the cause.", ctx.underlying.ID)

		proj, err := c.store.GetProject(ctx.underlying.ProjectID)
		if err != nil {
			c.Logf("failed to retrieve project via %s: %v", ctx.underlying.ProjectID, err)
			return err
		}

		client, err := InstallationTokenClient(ctx.installationToken, proj.Github.BaseURL, proj.Github.UploadURL)
		if err != nil {
			c.Logf("Failed to create a new installation token client: %s", err)
			return err
		}

		ownerRepo := strings.Split(proj.Repo.Name, "/")
		_, _, err = client.Issues.CreateComment(context.Background(), ownerRepo[0], ownerRepo[1], ctx.issueNumber, &github.IssueComment{
			Body: &msg,
		})
		if err != nil {
			return err
		}
	}

	return nil
}

// completeOrRetry checks if an error happened and makes sure the reporter will retry the errored key later.
func (c *BuildReporter) completeOrRetry(err error, key interface{}) {
	if err == nil {
		// Prevent the next trial on the key from delaying because it has succeeded
		c.queue.Forget(key)
		return
	}

	// Retry at most 5 times on error.
	if c.queue.NumRequeues(key) < 5 {
		c.Logf("Error syncing %q: %v", key, err)

		// Re-enqueue the key with delay
		c.queue.AddRateLimited(key)
		return
	}

	c.queue.Forget(key)

	c.Logf("Dropping %q: %v", key, err)
}

func (c *BuildReporter) Run(threadiness int, stopCh chan struct{}) {
	defer c.queue.ShutDown()
	c.Logf("Starting build reporter")

	go c.informer.Run(stopCh)

	if !cache.WaitForCacheSync(stopCh, c.informer.HasSynced) {
		c.Logf("Timed out waiting for caches to sync")
		return
	}

	for i := 0; i < threadiness; i++ {
		go wait.Until(c.runWorker, time.Second, stopCh)
	}

	<-stopCh
	c.Logf("Stopping build reporter")
}

func (c *BuildReporter) runWorker() {
	// Note that this isn't a busy loop as it blocks on each dequeue
	for c.processNextBuildPodUpdate() {
	}
}

func NewBuildReporter(clientset *kubernetes.Clientset, store storage.Store, ns string) *BuildReporter {
	podListWatcher := cache.NewListWatchFromClient(clientset.CoreV1().RESTClient(), "pods", ns, fields.Everything())

	queue := workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter())

	indexer, informer := cache.NewIndexerInformer(podListWatcher, &v1.Pod{}, 0, cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			key, err := cache.MetaNamespaceKeyFunc(obj)
			if err == nil {
				queue.Add(key)
			}
		},
		UpdateFunc: func(old interface{}, new interface{}) {
			key, err := cache.MetaNamespaceKeyFunc(new)
			if err == nil {
				queue.Add(key)
			}
		},
	}, cache.Indexers{})

	controller := newBuildReporter(queue, indexer, informer, store, ns)

	return controller
}

func (c *BuildReporter) Logf(msg string, v ...interface{}) {
	log.Printf(msg, v...)
}

// commentableBuild is a brigade build that is run on a GitHub issue or pull request
type commentableBuild struct {
	underlying        *brigade.Build
	issueNumber       int
	installationToken string
}

func (c *BuildReporter) Add(b *brigade.Build, issueNumber int, tok string) {
	podName := fmt.Sprintf("brigade-worker-%s", b.ID)

	c.podToBuild[podName] = &commentableBuild{
		underlying:        b,
		installationToken: tok,
		issueNumber:       issueNumber,
	}

	c.indexer.Add(&v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      podName,
			Namespace: c.ns,
		},
	})
}

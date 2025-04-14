package e2e

import (
	"context"
	"net/http"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/kubernetes/test/e2e/framework"
)

func TestCCMPodRunning(t *testing.T) {
	h := NewTestHarness(t)

	ccmImages := sets.New[string]()

	var ccmPods []*corev1.Pod
	for _, pod := range h.Pods().All() {
		isCCM := false
		for _, container := range pod.Spec.Containers {
			if strings.Contains(container.Image, "/cloud-controller-manager:") {
				ccmImages.Insert(container.Image)
				isCCM = true
			}
		}
		if isCCM {
			ccmPods = append(ccmPods, pod)
		}
	}

	// Make sure we have at least one CCM pod (but it is OK to have multiple CCM pods)
	if len(ccmPods) == 0 {
		t.Fatalf("CCM pod not found (no matching images)")
	}

	// We should have exactly one image
	if ccmImages.Len() != 1 {
		t.Fatalf("found multiple CCM images: %v", ccmImages)
	}
	for ccmImage := range ccmImages {
		t.Logf("found ccm image: %v", ccmImage)
	}

	// All the pods should be running
	for _, ccmPod := range ccmPods {
		t.Logf("verify status of ccm pod %v", ccmPod.Name)

		if ccmPod.Status.Phase != corev1.PodRunning {
			t.Errorf("pod %v in unexpected phase %v", ccmPod.Name, ccmPod.Status.Phase)
		}

		for _, containerStatus := range ccmPod.Status.ContainerStatuses {
			if !containerStatus.Ready {
				t.Errorf("pod %v has container %v that was not ready", ccmPod.Name, containerStatus.Name)
			}
		}
	}
}

func NewTestHarness(t *testing.T) *TestHarness {
	h := &TestHarness{T: t}

	ctx, close := context.WithCancel(context.Background())
	t.Cleanup(func() {
		close()
	})

	h.Ctx = ctx

	return h
}

type TestHarness struct {
	T   *testing.T
	Ctx context.Context

	restConfig *rest.Config
	httpClient *http.Client

	typedClient   kubernetes.Interface
	dynamicClient dynamic.Interface
}

func (h *TestHarness) Dynamic() dynamic.Interface {
	if h.dynamicClient != nil {
		return h.dynamicClient
	}

	dynamicClient, err := dynamic.NewForConfigAndClient(h.RESTConfig(), h.HTTPClient())
	if err != nil {
		h.T.Fatalf("building dynamic client: %v", err)
	}
	h.dynamicClient = dynamicClient
	return dynamicClient
}

func (h *TestHarness) TypedClient() kubernetes.Interface {
	if h.typedClient != nil {
		return h.typedClient
	}

	// restConfig := h.RESTConfig()
	typedClient, err := kubernetes.NewForConfigAndClient(h.RESTConfig(), h.HTTPClient())
	if err != nil {
		h.T.Fatalf("building types client: %v", err)
	}
	// h.T.Logf("restConfig is %+v", restConfig)
	// h.T.Logf("restConfig.tlsClientConfig is %+v", restConfig.TLSClientConfig)
	// h.T.Logf("restConfig.tlsClientConfig.caData is %+v", restConfig.TLSClientConfig.CAData)

	h.typedClient = typedClient
	return typedClient
}

func (h *TestHarness) RESTConfig() *rest.Config {
	if h.restConfig != nil {
		return h.restConfig
	}

	p := framework.TestContext.KubeConfig
	if p == "" {
		h.T.Fatalf("KubeConfig must be specified to load client config")
	}
	c, err := clientcmd.LoadFromFile(p)
	if err != nil {
		h.T.Fatalf("error loading kubeconfig from %q: %v", p, err)
	}
	restConfig, err := clientcmd.NewDefaultClientConfig(*c, nil).ClientConfig()
	if err != nil {
		h.T.Fatalf("building restConfig from kubeconfig at %q: %v", p, err)
	}
	h.T.Logf("loaded kubeconfig from %q", p)
	h.restConfig = restConfig
	return restConfig
}

func (h *TestHarness) HTTPClient() *http.Client {
	if h.httpClient != nil {
		return h.httpClient
	}

	restConfig := h.RESTConfig()
	httpClient, err := rest.HTTPClientFor(restConfig)
	if err != nil {
		h.T.Fatalf("building HTTP client from REST config: %v", err)
	}
	h.httpClient = httpClient
	return httpClient
}

func (h *TestHarness) Pods() *Pods {
	return &Pods{h: h}
}

type Pods struct {
	h *TestHarness
}

func (p *Pods) All() []*corev1.Pod {
	h := p.h
	kube := h.TypedClient()
	pods, err := kube.CoreV1().Pods("").List(h.Ctx, metav1.ListOptions{})
	if err != nil {
		h.T.Fatalf("querying pods: %v", err)
	}

	var ret []*corev1.Pod
	for i := range pods.Items {
		ret = append(ret, &pods.Items[i])
	}
	return ret
}

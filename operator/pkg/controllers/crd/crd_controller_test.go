// Copyright (c) 2024 Red Hat, Inc.
// Copyright Contributors to the Open Cluster Management project

package crd

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer/yaml"
	yamlutil "k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/restmapper"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/stolostron/multicluster-global-hub/operator/pkg/config"
	"github.com/stolostron/multicluster-global-hub/operator/pkg/constants"
)

var (
	cfg           *rest.Config
	ctx           context.Context
	cancel        context.CancelFunc
	kubeClient    *kubernetes.Clientset
	dynamicClient *dynamic.DynamicClient
)

func TestMain(m *testing.M) {
	ctx, cancel = context.WithCancel(context.Background())

	err := os.Setenv("POD_NAMESPACE", "default")
	if err != nil {
		panic(err)
	}
	// start testenv
	testenv := &envtest.Environment{
		CRDDirectoryPaths: []string{
			filepath.Join("..", "..", "..", "..", "test", "manifest", "crd"),
		},
		ErrorIfCRDPathMissing: true,
	}

	cfg, err = testenv.Start()
	if err != nil {
		panic(err)
	}

	if cfg == nil {
		panic(fmt.Errorf("empty kubeconfig!"))
	}

	kubeClient, err = kubernetes.NewForConfig(cfg)
	if err != nil {
		panic(err)
	}

	dynamicClient, err = dynamic.NewForConfig(cfg)
	if err != nil {
		panic(err)
	}

	// run testings
	code := m.Run()

	// stop testenv
	if err := testenv.Stop(); err != nil {
		panic(err)
	}
	os.Exit(code)
}

func TestCRDCtr(t *testing.T) {
	mgr, err := ctrl.NewManager(cfg, ctrl.Options{
		Metrics: metricsserver.Options{
			BindAddress: "0", // disable the metrics serving
		},
		Scheme: config.GetRuntimeScheme(),
	})
	assert.Nil(t, err)

	instance, err := controller.New(constants.GlobalHubControllerName, mgr, controller.Options{
		Reconciler: reconcile.Func(
			func(_ context.Context, request reconcile.Request) (reconcile.Result, error) {
				return reconcile.Result{}, nil
			}),
	})

	_, err = AddCRDController(mgr, &config.OperatorConfig{}, nil, instance)
	assert.Nil(t, err)

	go func() {
		err := mgr.Start(ctx)
		assert.Nil(t, err)
	}()
	assert.True(t, mgr.GetCache().WaitForCacheSync(ctx))

	clusterResourceFile := "operator.open-cluster-management.io_multiclusterglobalhubs.yaml"

	err = applyYaml(filepath.Join("..", "..", "..", "config", "crd", "bases", clusterResourceFile))
	assert.Nil(t, err)
	time.Sleep(1 * time.Second)

	assert.True(t, config.IsACMResourceReady())
	assert.True(t, config.GetKafkaResourceReady())
	cancel()
}

func applyYaml(file string) error {
	b, err := os.ReadFile(file)
	if err != nil {
		return err
	}
	decoder := yamlutil.NewYAMLOrJSONDecoder(bytes.NewReader(b), 100)
	var rawObj runtime.RawExtension
	if err = decoder.Decode(&rawObj); err != nil {
		return err
	}
	obj, gvk, err := yaml.NewDecodingSerializer(unstructured.UnstructuredJSONScheme).Decode(rawObj.Raw, nil, nil)
	if err != nil {
		return err
	}
	unstructuredMap, err := runtime.DefaultUnstructuredConverter.ToUnstructured(obj)
	if err != nil {
		log.Fatal(err)
	}

	unstructuredObj := &unstructured.Unstructured{Object: unstructuredMap}

	gr, err := restmapper.GetAPIGroupResources(kubeClient.Discovery())
	if err != nil {
		return err
	}

	mapper := restmapper.NewDiscoveryRESTMapper(gr)
	mapping, err := mapper.RESTMapping(gvk.GroupKind(), gvk.Version)
	if err != nil {
		return err
	}

	var dri dynamic.ResourceInterface
	if mapping.Scope.Name() == meta.RESTScopeNameNamespace {
		if unstructuredObj.GetNamespace() == "" {
			unstructuredObj.SetNamespace("default")
		}
		dri = dynamicClient.Resource(mapping.Resource).Namespace(unstructuredObj.GetNamespace())
	} else {
		dri = dynamicClient.Resource(mapping.Resource)
	}

	_, err = dri.Create(ctx, unstructuredObj, metav1.CreateOptions{})
	return err
}

package apps

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
	appsv1alpha1 "open-cluster-management.io/multicloud-operators-subscription/pkg/apis/apps/v1alpha1"
	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/stolostron/hub-of-hubs/agent/pkg/status/bundle"
	"github.com/stolostron/hub-of-hubs/agent/pkg/status/controller/generic"
	"github.com/stolostron/hub-of-hubs/agent/pkg/status/controller/syncintervals"
	"github.com/stolostron/hub-of-hubs/agent/pkg/transport/producer"
	"github.com/stolostron/hub-of-hubs/pkg/constants"
)

const (
	subscriptionReportsSyncLog = "subscriptions-reports-sync"
)

// AddSubscriptionReportsController adds subscription-report controller to the manager.
func AddSubscriptionReportsController(mgr ctrl.Manager, transport producer.Producer, leafHubName string,
	incarnation uint64, _ *corev1.ConfigMap, syncIntervalsData *syncintervals.SyncIntervals,
) error {
	createObjFunction := func() bundle.Object { return &appsv1alpha1.SubscriptionReport{} }

	bundleCollection := []*generic.BundleCollectionEntry{
		generic.NewBundleCollectionEntry(fmt.Sprintf("%s.%s", leafHubName, constants.SubscriptionReportMsgKey),
			bundle.NewGenericStatusBundle(leafHubName, incarnation, nil),
			func() bool { return true }),
	} // bundle predicate - always send subscription report.

	if err := generic.NewGenericStatusSyncController(mgr, subscriptionReportsSyncLog, transport, bundleCollection,
		createObjFunction, nil, syncIntervalsData.GetPolicies); err != nil {
		return fmt.Errorf("failed to add subscription reports controller to the manager - %w", err)
	}

	return nil
}

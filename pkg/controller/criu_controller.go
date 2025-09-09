package controller

import (
	"context"
	"errors"
	"fmt"
	"iter"
	"slices"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	apiruntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog/v2"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	trainer "github.com/kubeflow/trainer/v2/pkg/apis/trainer/v1alpha1"
	"github.com/kubeflow/trainer/v2/pkg/constants"
	jobruntimes "github.com/kubeflow/trainer/v2/pkg/runtime"
)

type TrainJobWatcher interface {
	NotifyTrainJobUpdate(oldJob, newJob *trainer.TrainJob)
}

type CRIUController struct {
	log      logr.Logger
	client   client.Client // optional, if you're managing Kubernetes resources
	recorder record.EventRecorder
	watchers iter.Seq[CRIUCheckpointWatcher]
}

type CheckpointInfo struct {
    ID        string    `json:"id"`
    CreatedAt time.Time `json:"createdAt"`
    Size      int64     `json:"size"`
    Status    string    `json:"status"`
}

func WithWatchers(watchers ...TrainJobWatcher) TrainJobReconcilerOption {
	return func(o *TrainJobReconcilerOptions) {
		o.Watchers = slices.Values(watchers)
	}
}

var _ reconcile.Reconciler = (*CRIUController)(nil)
var _ predicate.TypedPredicate[*trainer.TrainJob] = (*CRIUController)(nil)

func NewCRIUController(client client.Client, recorder record.EventRecorder, runtimes map[string]jobruntimes.Runtime, opts ...TrainJobReconcilerOption) *CRIUController {
	options := &TrainJobReconcilerOptions{}
	for _, opt := range opts {
		opt(options)
	}
	return &CRIUController{
		log:      ctrl.Log.WithName("criu-controller"),
		client:   client,
		recorder: recorder,
		runtimes: runtimes,
		watchers: options.Watchers,
	}
}

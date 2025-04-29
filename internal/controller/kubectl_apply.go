package controller

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type GenericReconciler interface {
	GetLog() logr.Logger
	GetRecorder() record.EventRecorder
	GetClient() client.Client
	GetReconciledObject() client.Object
	GetContext() context.Context
	GetReconcilerName() string
}

func apply(r GenericReconciler, object client.Object) error {
	objectKind := object.GetObjectKind().GroupVersionKind().Kind
	namespaceString := fmt.Sprintf("%s.Namespace", objectKind)
	nameString := fmt.Sprintf("%s.Name", objectKind)
	r.GetLog().Info(fmt.Sprintf("Applying %s %s", r.GetReconcilerName(), objectKind), namespaceString, object.GetNamespace(), nameString, object.GetName())
	r.GetRecorder().Event(r.GetReconciledObject(), corev1.EventTypeNormal, "Applying", fmt.Sprintf("Creating %s %s", r.GetReconcilerName(), objectKind))

	if err := r.GetClient().Patch(r.GetContext(), object, client.Apply, client.ForceOwnership, client.FieldOwner("cloudflare-operator")); err != nil {
		r.GetLog().Error(err, fmt.Sprintf("Failed to apply new %s", objectKind), namespaceString, object.GetNamespace(), nameString, object.GetName())
		r.GetRecorder().Event(r.GetReconciledObject(), corev1.EventTypeWarning, "FailedApplying", fmt.Sprintf("Applying %s %s failed", r.GetReconcilerName(), objectKind))
		return err
	}

	r.GetLog().Info(fmt.Sprintf("%s applied", objectKind), namespaceString, object.GetNamespace(), nameString, object.GetName())
	r.GetRecorder().Event(r.GetReconciledObject(), corev1.EventTypeNormal, "Applied", fmt.Sprintf("Applied %s %s", r.GetReconcilerName(), objectKind))
	return nil
}

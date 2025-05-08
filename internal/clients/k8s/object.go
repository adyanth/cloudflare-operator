// Package k8s provides client abstractions to the kubernetes API
package k8s

import (
	"context"
	"errors"
	"fmt"

	"github.com/go-logr/logr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

type ObjectClient struct {
	k8sClient client.Client
	log       *logr.Logger
}

func NewObjectClient(k8sClient client.Client, log *logr.Logger) (*ObjectClient, error) {
	if k8sClient == nil {
		return nil, errors.New("k8sClient cannot be nil")
	}
	if log == nil {
		return nil, errors.New("log cannot be nil")
	}
	return &ObjectClient{
		k8sClient: k8sClient,
		log:       log,
	}, nil
}

// EnsureFinalizer adds `finalizer` to obj exactly once.
//   - It NO-OPs if the finalizer is already there.
//   - It uses a strategic-merge Patch so you don’t overwrite concurrent changes.
//   - obj must be a pointer that already contains the latest copy from the API server.
func (s *ObjectClient) EnsureFinalizer(ctx context.Context, key client.ObjectKey, obj client.Object, finalizer string) error {
	err := s.k8sClient.Get(ctx, key, obj)
	if err != nil {
		return err
	}
	// if it already contains the finalizer, there's nothing to do
	if controllerutil.ContainsFinalizer(obj, finalizer) {
		return nil
	}

	//nolint:revive // we know this will serialise, even if the compiler doesn't
	base := obj.DeepCopyObject().(client.Object)
	controllerutil.AddFinalizer(obj, finalizer)

	s.log.
		WithValues("finalizer", finalizer).
		WithValues("object", fmt.Sprintf("%s/%s", obj.GetNamespace(), obj.GetName())).
		WithValues("kind", obj.GetObjectKind().GroupVersionKind().Kind).
		Info("creating finalizer")
	if err := s.k8sClient.Patch(ctx, obj, client.MergeFrom(base)); err != nil {
		return fmt.Errorf("could not add finalizer %q: %w", finalizer, err)
	}
	return nil
}

// RemoveFinalizer deletes `finalizer` from obj exactly once.
//   - NO-OPs if the finalizer is already gone.
//   - Uses a strategic-merge Patch so you never clobber concurrent changes.
//   - obj must be a *live* copy fetched from the API server.
func (s *ObjectClient) RemoveFinalizer(ctx context.Context, key client.ObjectKey, obj client.Object, finalizer string) error {
	err := s.k8sClient.Get(ctx, key, obj)
	if err != nil {
		return err
	}
	// if there is no finalizer, there's nothing to do
	if !controllerutil.ContainsFinalizer(obj, finalizer) {
		return nil
	}

	//nolint:revive // we know this will serialise, even if the compiler doesn't
	base := obj.DeepCopyObject().(client.Object)
	controllerutil.RemoveFinalizer(obj, finalizer)

	s.log.
		WithValues("finalizer", finalizer).
		WithValues("object", fmt.Sprintf("%s/%s", obj.GetNamespace(), obj.GetName())).
		Info("removing finalizer")
	if err := s.k8sClient.Patch(ctx, obj, client.MergeFrom(base)); err != nil {
		return fmt.Errorf("could not remove finalizer %q: %w", finalizer, err)
	}
	return nil
}

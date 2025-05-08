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
func (s *ObjectClient) EnsureFinalizer(ctx context.Context, key client.ObjectKey, finalizer string) error {
	var obj client.Object
	err := s.k8sClient.Get(ctx, key, obj)
	if err != nil {
		return err
	}
	// if it already contains the finalizer, there's nothing to do
	if controllerutil.ContainsFinalizer(obj, finalizer) {
		return nil
	}

	controllerutil.AddFinalizer(obj, finalizer)

	base, err := s.deepCopyObject(obj)
	if err != nil {
		return err
	}

	s.log.
		WithValues("finalizer", finalizer).
		WithValues("object", fmt.Sprintf("%s/%s", obj.GetNamespace(), obj.GetName())).
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
func (s *ObjectClient) RemoveFinalizer(ctx context.Context, key client.ObjectKey, finalizer string) error {
	var obj client.Object
	err := s.k8sClient.Get(ctx, key, obj)
	if err != nil {
		return err
	}
	// if there is no finalizer, there's nothing to do
	if !controllerutil.ContainsFinalizer(obj, finalizer) {
		return nil
	}

	controllerutil.RemoveFinalizer(obj, finalizer)

	base, err := s.deepCopyObject(obj)
	if err != nil {
		return err
	}

	s.log.
		WithValues("finalizer", finalizer).
		WithValues("object", fmt.Sprintf("%s/%s", obj.GetNamespace(), obj.GetName())).
		Info("removing finalizer")
	if err := s.k8sClient.Patch(ctx, obj, client.MergeFrom(base)); err != nil {
		return fmt.Errorf("could not remove finalizer %q: %w", finalizer, err)
	}
	return nil
}

func (*ObjectClient) deepCopyObject(obj client.Object) (client.Object, error) {
	objDeepCopy := obj.DeepCopyObject()
	if objDeepCopy == nil {
		return nil, errors.New("received nil object from DeepCopyObject")
	}
	base, ok := objDeepCopy.(client.Object)
	if !ok {
		return nil, errors.New("failed to convert object to client.Object")
	}
	return base, nil
}

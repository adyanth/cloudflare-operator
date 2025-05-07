package k8s

import (
	"context"
	"errors"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type SecretClient struct {
	k8sClient client.Client
	log       *logr.Logger
}

func NewSecretClient(k8sClient client.Client, log *logr.Logger) (*SecretClient, error) {
	if k8sClient == nil {
		return nil, errors.New("k8sClient cannot be nil")
	}
	if log == nil {
		return nil, errors.New("logger cannot be nil")
	}
	return &SecretClient{
		k8sClient: k8sClient,
		log:       log,
	}, nil
}

// EnsureFinalizer idempotently adds the specified finalizer to the specified secret
func (s *SecretClient) EnsureFinalizer(ctx context.Context, secretName, secretNamespace, finalizer string) error {
	var secret corev1.Secret
	secretKey := client.ObjectKey{
		Name:      secretName,
		Namespace: secretNamespace,
	}
	if err := s.k8sClient.Get(ctx, secretKey, &secret); err != nil {
		return err
	}

	// if finalizer already exists, we are happy
	for _, existingFinalizer := range secret.Finalizers {
		if finalizer == existingFinalizer {
			return nil
		}
	}

	s.log.WithValues("finalizer", finalizer).Info("creating finalizer")
	secret.Finalizers = append(secret.Finalizers, finalizer)
	return s.k8sClient.Update(ctx, &secret)
}

// RemoveFinalizer removes the first instance of the finalizer from the given secret
func (s *SecretClient) RemoveFinalizer(ctx context.Context, secretName, secretNamespace, finalizer string) error {
	var secret corev1.Secret
	secretKey := client.ObjectKey{
		Name:      secretName,
		Namespace: secretNamespace,
	}
	if err := s.k8sClient.Get(ctx, secretKey, &secret); err != nil {
		if k8serrors.IsNotFound(err) {
			return nil
		}
		return err
	}

	s.log.WithValues("finalizer", finalizer).Info("deleting finalizer")
	secret.Finalizers = removeString(secret.Finalizers, finalizer)
	return s.k8sClient.Update(ctx, &secret)
}

// removeString returns a copy of list with the first (only) occurrence
// of target removed. If target is not present, the original slice is
// returned unchanged.
func removeString(list []string, target string) []string {
	for i, v := range list {
		if v == target {
			// splice out the element at index i
			return append(list[:i], list[i+1:]...)
		}
	}
	return list
}

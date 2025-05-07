package k8s

import (
	"context"
	"fmt"
	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type SecretClient struct {
	client client.Client
	log    *logr.Logger
}

func NewSecretClient(client client.Client, log *logr.Logger) (*SecretClient, error) {
	if client == nil {
		return nil, fmt.Errorf("client cannot be nil")
	}
	if log == nil {
		return nil, fmt.Errorf("logger cannot be nil")
	}
	return &SecretClient{
		client: client,
		log:    log,
	}, nil
}

// EnsureFinalizer idempotently adds the specified finalizer to the specified secret
func (s *SecretClient) EnsureFinalizer(ctx context.Context, secretName, secretNamespace, finalizer string) error {
	var secret corev1.Secret
	secretKey := client.ObjectKey{
		Name:      secretName,
		Namespace: secretNamespace,
	}
	if err := s.client.Get(ctx, secretKey, &secret); err != nil {
		return err
	}

	// if finalizer already exists, we are happy
	for _, existingFinalizer := range secret.ObjectMeta.Finalizers {
		if finalizer == existingFinalizer {
			return nil
		}
	}

	s.log.WithValues("finalizer", finalizer).Info("creating finalizer")
	secret.Finalizers = append(secret.Finalizers, finalizer)
	return s.client.Update(ctx, &secret)
}

// RemoveFinalizer removes the first instance of the finalizer from the given secret
func (s *SecretClient) RemoveFinalizer(ctx context.Context, secretName, secretNamespace, finalizer string) error {
	var secret corev1.Secret
	secretKey := client.ObjectKey{
		Name:      secretName,
		Namespace: secretNamespace,
	}
	if err := s.client.Get(ctx, secretKey, &secret); err != nil {
		if errors.IsNotFound(err) {
			return nil
		}
		return err
	}

	s.log.WithValues("finalizer", finalizer).Info("deleting finalizer")
	secret.Finalizers = removeString(secret.Finalizers, finalizer)
	return s.client.Update(ctx, &secret)
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

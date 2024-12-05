package v1alpha1

import (
	"context"
	"fmt"

	"github.com/mariadb-operator/mariadb-operator/pkg/metadata"
	"github.com/mariadb-operator/mariadb-operator/pkg/predicate"
	"github.com/mariadb-operator/mariadb-operator/pkg/watch"
	corev1 "k8s.io/api/core/v1"
	ctrlbuilder "sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

const maxScaleMetricsPasswordSecretFieldPath = ".spec.auth.metricsPasswordSecretKeyRef.name"

// IndexerFuncForFieldPath returns an indexer function for a given field path.
func (m *MaxScale) IndexerFuncForFieldPath(fieldPath string) (client.IndexerFunc, error) {
	switch fieldPath {
	case maxScaleMetricsPasswordSecretFieldPath:
		return func(obj client.Object) []string {
			maxscale, ok := obj.(*MaxScale)
			if !ok {
				return nil
			}
			if maxscale.AreMetricsEnabled() && maxscale.Spec.Auth.MetricsPasswordSecretKeyRef.Name != "" {
				return []string{maxscale.Spec.Auth.MetricsPasswordSecretKeyRef.Name}
			}
			return nil
		}, nil
	default:
		return nil, fmt.Errorf("unsupported field path: %s", fieldPath)
	}
}

// IndexMaxScale watches and indexes external resources referred by MaxScale resources.
func IndexMaxScale(ctx context.Context, mgr manager.Manager, builder *ctrlbuilder.Builder, client client.Client) error {
	watcherIndexer := watch.NewWatcherIndexer(mgr, builder, client)

	if err := watcherIndexer.Watch(
		ctx,
		&corev1.Secret{},
		&MaxScale{},
		&MaxScaleList{},
		maxScaleMetricsPasswordSecretFieldPath,
		ctrlbuilder.WithPredicates(
			predicate.PredicateWithLabel(metadata.WatchLabel),
		),
	); err != nil {
		return fmt.Errorf("error watching: %v", err)
	}

	return nil
}

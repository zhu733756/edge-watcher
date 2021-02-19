package controllers

import (
	"context"
	kubeedgev1alpha1 "kubesphere.io/edge-watcher/api/v1alpha1"
)

func (r *IPTablesReconciler) addFinalizer(ctx context.Context, instance *kubeedgev1alpha1.IPTables) error {
	instance.AddFinalizer(kubeedgev1alpha1.IPTablesFinalizerName)
	return r.Update(ctx, instance)
}

func (r *IPTablesReconciler) handleFinalizer(ctx context.Context, instance *kubeedgev1alpha1.IPTables) error {
	if !instance.HasFinalizer(kubeedgev1alpha1.IPTablesFinalizerName) {
		return nil
	}
	if err := r.delete(ctx, instance); err != nil {
		return err
	}
	instance.RemoveFinalizer(kubeedgev1alpha1.IPTablesFinalizerName)
	return r.Update(ctx, instance)
}

/*
Copyright 2020 The KubeSphere Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controllers

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/go-logr/logr"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/source"

	kubeedgev1alpha1 "kubesphere.io/edge-watcher/api/v1alpha1"
	"kubesphere.io/edge-watcher/pkg/operator"
)

// IPTablesReconciler reconciles a IPTables object
type IPTablesReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

type IPTablesRulesFile struct {
	IPTables []kubeedgev1alpha1.IPTablesRule `json:"iptables"`
}

// +kubebuilder:rbac:groups=kubeedge.kubesphere.io,resources=iptables,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=kubeedge.kubesphere.io,resources=iptables/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=kubeedge.kubesphere.io,resources=iptablesrules,verbs=get;list;watch;create;update;patch;delete;deletecollection
// +kubebuilder:rbac:groups=apps,resources=daemonsets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=secrets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=serviceaccounts,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=clusterroles,verbs=create
// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=clusterrolebindings,verbs=create
// +kubebuilder:rbac:groups=core,resources=pods,verbs=get
// +kubebuilder:rbac:groups=core,resources=nodes,verbs=get;list;watch

func (r *IPTablesReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()
	_ = r.Log.WithValues("iptables", req.NamespacedName)

	var it kubeedgev1alpha1.IPTables
	err := r.Get(ctx, req.NamespacedName, &it)
	if err == nil {
		// Reconcile IPTables
		if it.IsBeingDeleted() {
			if err := r.handleFinalizer(ctx, &it); err != nil {
				return ctrl.Result{}, fmt.Errorf("error when handling finalizer: %v", err)
			}
			return ctrl.Result{}, nil
		}

		if !it.HasFinalizer(kubeedgev1alpha1.IPTablesFinalizerName) {
			if err := r.addFinalizer(ctx, &it); err != nil {
				return ctrl.Result{}, fmt.Errorf("error when adding finalizer: %v", err)
			}
			return ctrl.Result{}, nil
		}

		var rules kubeedgev1alpha1.IPTablesRulesList
		if err := r.List(ctx, &rules); err != nil {
			if errors.IsNotFound(err) {
				return ctrl.Result{}, nil
			}
			return ctrl.Result{}, err
		}

		ipTablesRules := make([]kubeedgev1alpha1.IPTablesRule, 0)

		for _, item := range rules.Items {
			for _, rule := range item.Spec.Rules {
				ipTablesRules = append(ipTablesRules, rule)
			}
		}

		ipTablesRulesFile := &IPTablesRulesFile{
			IPTables: ipTablesRules,
		}

		ipTablesRulesFileData, err := json.Marshal(ipTablesRulesFile)
		if err != nil {
			return ctrl.Result{}, err
		}

		// Create or update the corresponding Secret
		sec := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      it.Name,
				Namespace: it.Namespace,
			},
			Data: map[string][]byte{"iptables.conf": ipTablesRulesFileData},
		}
		if err := ctrl.SetControllerReference(&it, sec, r.Scheme); err != nil {
			return ctrl.Result{}, err
		}

		if _, err := controllerutil.CreateOrUpdate(ctx, r.Client, sec, func() error {
			sec.Data = map[string][]byte{"iptables.conf": ipTablesRulesFileData}
			sec.SetOwnerReferences(nil)
			if err := ctrl.SetControllerReference(&it, sec, r.Scheme); err != nil {
				return err
			}
			return nil
		}); err != nil {
			return ctrl.Result{}, err
		}

		// Install RBAC resources for the filter plugin kubernetes
		cr, sa, crb := operator.MakeRBACObjects(it.Name, it.Namespace)
		// Set ServiceAccount's owner to this iptables
		if err := ctrl.SetControllerReference(&it, &sa, r.Scheme); err != nil {
			return ctrl.Result{}, err
		}
		if err := r.Create(ctx, &cr); err != nil && !errors.IsAlreadyExists(err) {
			return ctrl.Result{}, err
		}
		if err := r.Create(ctx, &sa); err != nil && !errors.IsAlreadyExists(err) {
			return ctrl.Result{}, err
		}
		if err := r.Create(ctx, &crb); err != nil && !errors.IsAlreadyExists(err) {
			return ctrl.Result{}, err
		}

		// Deploy IPTables DaemonSet
		ds := operator.MakeDaemonSet(it)
		if err := ctrl.SetControllerReference(&it, &ds, r.Scheme); err != nil {
			return ctrl.Result{}, err
		}

		if _, err := controllerutil.CreateOrUpdate(ctx, r.Client, &ds, r.mutate(&ds, it)); err != nil {
			return ctrl.Result{}, err
		}

		// Install default configmap of edgeNodeJoin
		edgeNodeJoinDefaultConfigMap := operator.MakeEdgeNodeJoinDefaultConfigMap(it.Namespace)
		if err := ctrl.SetControllerReference(&it, &edgeNodeJoinDefaultConfigMap, r.Scheme); err != nil {
			return ctrl.Result{}, err
		}

		if err := r.Create(ctx, &edgeNodeJoinDefaultConfigMap); err != nil && !errors.IsAlreadyExists(err) {
			return ctrl.Result{}, err
		}

		return ctrl.Result{}, nil
	} else {
		// Reconcile IPTablesRules
		var rules kubeedgev1alpha1.IPTablesRulesList
		if err := r.List(ctx, &rules); err != nil {
			if errors.IsNotFound(err) {
				return ctrl.Result{}, nil
			}
			return ctrl.Result{}, err
		}

		ipTablesRules := make([]kubeedgev1alpha1.IPTablesRule, 0)

		for _, item := range rules.Items {
			for _, rule := range item.Spec.Rules {
				ipTablesRules = append(ipTablesRules, rule)
			}
		}

		ipTablesRulesFile := &IPTablesRulesFile{
			IPTables: ipTablesRules,
		}

		ipTablesRulesFileData, err := json.Marshal(ipTablesRulesFile)
		if err != nil {
			return ctrl.Result{}, err
		}

		var its kubeedgev1alpha1.IPTablesList
		if err := r.List(ctx, &its); err != nil {
			if errors.IsNotFound(err) {
				return ctrl.Result{}, nil
			}
			return ctrl.Result{}, err
		}

		for _, it := range its.Items {
			// Create or update the corresponding Secret
			sec := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      it.Name,
					Namespace: it.Namespace,
				},
				Data: map[string][]byte{"iptables.conf": ipTablesRulesFileData},
			}
			if err := ctrl.SetControllerReference(&it, sec, r.Scheme); err != nil {
				return ctrl.Result{}, err
			}

			if _, err := controllerutil.CreateOrUpdate(ctx, r.Client, sec, func() error {
				sec.Data = map[string][]byte{"iptables.conf": ipTablesRulesFileData}
				sec.SetOwnerReferences(nil)
				if err := ctrl.SetControllerReference(&it, sec, r.Scheme); err != nil {
					return err
				}
				return nil
			}); err != nil {
				return ctrl.Result{}, err
			}
		}
	}

	return ctrl.Result{}, nil
}

func (r *IPTablesReconciler) mutate(ds *appsv1.DaemonSet, it kubeedgev1alpha1.IPTables) controllerutil.MutateFn {
	expected := operator.MakeDaemonSet(it)

	return func() error {
		ds.Labels = expected.Labels
		ds.Spec = expected.Spec
		ds.SetOwnerReferences(nil)
		if err := ctrl.SetControllerReference(&it, ds, r.Scheme); err != nil {
			return err
		}
		return nil
	}
}

func (r *IPTablesReconciler) delete(ctx context.Context, it *kubeedgev1alpha1.IPTables) error {
	var sa corev1.ServiceAccount
	err := r.Get(ctx, client.ObjectKey{Namespace: it.Namespace, Name: it.Name}, &sa)
	if err == nil {
		if err := r.Delete(ctx, &sa); err != nil && !errors.IsNotFound(err) {
			return err
		}
	} else if !errors.IsNotFound(err) {
		return err
	}

	var ds appsv1.DaemonSet
	err = r.Get(ctx, client.ObjectKey{Namespace: it.Namespace, Name: it.Name}, &ds)
	if err == nil {
		if err := r.Delete(ctx, &ds); err != nil && !errors.IsNotFound(err) {
			return err
		}
	} else if !errors.IsNotFound(err) {
		return err
	}

	var sc corev1.Secret
	err = r.Get(ctx, client.ObjectKey{Namespace: it.Namespace, Name: it.Name}, &sc)
	if err == nil {
		if err := r.Delete(ctx, &sc); err != nil && !errors.IsNotFound(err) {
			return err
		}
	} else if !errors.IsNotFound(err) {
		return err
	}

	return nil
}

func (r *IPTablesReconciler) SetupWithManager(mgr ctrl.Manager) error {
	if err := mgr.GetFieldIndexer().IndexField(&corev1.ServiceAccount{}, ownerKey, func(rawObj runtime.Object) []string {
		// grab the job object, extract the owner.
		sa := rawObj.(*corev1.ServiceAccount)
		owner := metav1.GetControllerOf(sa)
		if owner == nil {
			return nil
		}
		// Make sure it's a IPTables. If so, return it.
		if owner.APIVersion != apiGVStr || owner.Kind != "IPTables" {
			return nil
		}
		return []string{owner.Name}
	}); err != nil {
		return err
	}

	if err := mgr.GetFieldIndexer().IndexField(&appsv1.DaemonSet{}, ownerKey, func(rawObj runtime.Object) []string {
		// grab the job object, extract the owner.
		ds := rawObj.(*appsv1.DaemonSet)
		owner := metav1.GetControllerOf(ds)
		if owner == nil {
			return nil
		}
		// Make sure it's a IPTables. If so, return it.
		if owner.APIVersion != apiGVStr || owner.Kind != "IPTables" {
			return nil
		}
		return []string{owner.Name}
	}); err != nil {
		return err
	}

	if err := mgr.GetFieldIndexer().IndexField(&corev1.Secret{}, ownerKey, func(rawObj runtime.Object) []string {
		// grab the job object, extract the owner.
		sc := rawObj.(*corev1.Secret)
		owner := metav1.GetControllerOf(sc)
		if owner == nil {
			return nil
		}
		// Make sure it's a IPTables. If so, return it.
		if owner.APIVersion != apiGVStr || owner.Kind != "IPTables" {
			return nil
		}
		return []string{owner.Name}
	}); err != nil {
		return err
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&kubeedgev1alpha1.IPTables{}).
		Owns(&corev1.ServiceAccount{}).
		Owns(&appsv1.DaemonSet{}).
		Owns(&corev1.Secret{}).
		Watches(&source.Kind{Type: &kubeedgev1alpha1.IPTablesRules{}}, &handler.EnqueueRequestForObject{}).
		Complete(r)
}

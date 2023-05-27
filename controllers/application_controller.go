/*
Copyright 2023 Daniel.Hu.

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
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"reflect"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"time"

	dappsv1 "github.com/shengjiangfeng/application-operator/api/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// ApplicationReconciler reconciles a Application object
type ApplicationReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=apps.danielhu.cn,resources=applications,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=apps.danielhu.cn,resources=applications/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=apps.danielhu.cn,resources=applications/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the Application object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.12.2/pkg/reconcile
func (r *ApplicationReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var CounterReconcileApplication int64
	const GenericRequeueDuration = 1 * time.Minute
	<-time.NewTicker(100 * time.Millisecond).C
	l := log.FromContext(ctx)
	// get the application
	CounterReconcileApplication += 1
	l.Info("Starting a reconcile", "number", CounterReconcileApplication)
	app := &dappsv1.Application{}
	if err := r.Get(ctx, req.NamespacedName, app); err != nil {
		if errors.IsNotFound(err) {
			l.Info("the Application not found")
			return ctrl.Result{}, err
		}
		l.Error(err, "failed to get the Application, will requeue after a short time.")
		return ctrl.Result{RequeueAfter: GenericRequeueDuration}, err
	}
	// create pods
	//for i := 0; i < int(app.Spec.Replicas); i++ {
	//	pod := &corev1.Pod{
	//		ObjectMeta: metav1.ObjectMeta{
	//			Name:      fmt.Sprintf("%s-%d", app.Name, i),
	//			Namespace: app.Namespace,
	//			Labels:    app.Labels,
	//		},
	//		Spec: app.Spec.Template.Spec,
	//	}
	//	if err := r.Create(ctx, pod); err != nil {
	//		l.Error(err, "failed to create pod for application")
	//		return ctrl.Result{RequeueAfter: 1 * time.Minute}, err
	//	}
	//	l.Info(fmt.Sprintf("the pod %s has been created", pod.Name))
	//}
	//l.Info("all pods has created")
	var result ctrl.Result
	var err error
	result, err = r.reconcileDeployment(ctx, app)
	if err != nil {
		l.Error(err, "Failed to reconcile Deployment.")
		return result, err
	}
	result, err = r.reconcileService(ctx, app)
	if err != nil {
		l.Error(err, "Failed to reconcile Service.")
		return result, err
	}
	l.Info("All resources have been reconciled.")
	return ctrl.Result{}, nil

}

func (r *ApplicationReconciler) reconcileDeployment(ctx context.Context, app *dappsv1.Application) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	const GenericRequeueDuration = 1 * time.Minute
	var dp = &appsv1.Deployment{}
	err := r.Get(ctx, types.NamespacedName{
		Namespace: app.Namespace,
		Name:      app.Name,
	}, dp)
	if err == nil {
		logger.Info("The Deployment has already exists.")
		if reflect.DeepEqual(dp.Status, app.Status.Workflow) {
			return ctrl.Result{}, nil
		}
		app.Status.Workflow = dp.Status
		if err := r.Status().Update(ctx, app); err != nil {
			logger.Error(err, "Failed to update Application Status.")
			return ctrl.Result{RequeueAfter: GenericRequeueDuration}, err
		}
		logger.Info("The Application Status has been updated.")
		return ctrl.Result{}, nil
	}
	if !errors.IsNotFound(err) {
		logger.Error(err, "Failed to get Deployment, will requeue after a short time.")
		return ctrl.Result{RequeueAfter: GenericRequeueDuration}, err
	}
	newDp := &appsv1.Deployment{}
	newDp.SetName(app.Name)
	newDp.SetNamespace(app.Namespace)
	newDp.SetLabels(app.Labels)
	newDp.Spec = app.Spec.Deployment.DeploymentSpec
	newDp.Spec.Template.SetLabels(app.Labels)
	if err := ctrl.SetControllerReference(app, newDp, r.Scheme); err != nil {
		logger.Error(err, "Failed to SetControllerReference,will requeue after a short time.")
		return ctrl.Result{RequeueAfter: GenericRequeueDuration}, err
	}
	if err := r.Create(ctx, newDp); err != nil {
		logger.Error(err, "Failed to create Deployment, will requeue after a short time.")
		return ctrl.Result{RequeueAfter: GenericRequeueDuration}, err
	}
	logger.Info("The Deployment has been created.")
	return ctrl.Result{}, nil
}
func (r *ApplicationReconciler) reconcileService(ctx context.Context, app *dappsv1.Application) (ctrl.Result, error) {
	const GenericRequeueDuration = 1 * time.Minute
	logger := log.FromContext(ctx)
	var svc = &corev1.Service{}
	err := r.Get(ctx, types.NamespacedName{
		Namespace: app.Namespace,
		Name:      app.Name,
	}, svc)
	if err == nil {
		logger.Info("The Service has already exists.")
		if reflect.DeepEqual(svc.Status, app.Status.Network) {
			return ctrl.Result{}, nil
		}
		app.Status.Network = svc.Status
		if err := r.Status().Update(ctx, app); err != nil {
			logger.Error(err, "Failed to update Application Status.")
			return ctrl.Result{RequeueAfter: GenericRequeueDuration}, err
		}
		logger.Info("The Application status has been updated.")
		return ctrl.Result{}, nil
	}
	if !errors.IsNotFound(err) {
		logger.Error(err, "Failed to get Service, will requeue after a short time.")
		return ctrl.Result{RequeueAfter: GenericRequeueDuration}, err
	}
	newSvc := &corev1.Service{}
	newSvc.SetName(app.Name)
	newSvc.SetNamespace(app.Namespace)
	newSvc.SetLabels(app.Labels)
	newSvc.Spec = app.Spec.Service.ServiceSpec
	newSvc.Spec.Selector = app.Labels
	if err := ctrl.SetControllerReference(app, newSvc, r.Scheme); err != nil {
		logger.Error(err, "Failed to SetControllerReference,will requeue after a short time.")
		return ctrl.Result{RequeueAfter: GenericRequeueDuration}, err
	}
	if err := r.Create(ctx, newSvc); err != nil {
		logger.Error(err, "Failed to create Service, will requeue after a short time.")
		return ctrl.Result{RequeueAfter: GenericRequeueDuration}, err
	}
	logger.Info("The Service has been created.")
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ApplicationReconciler) SetupWithManager(mgr ctrl.Manager) error {
	setupLog := ctrl.Log.WithName("setup")
	return ctrl.NewControllerManagedBy(mgr).
		For(&dappsv1.Application{}, builder.WithPredicates(predicate.Funcs{
			CreateFunc: func(event event.CreateEvent) bool {
				setupLog.Info("The Application will be create.")
				return true
			},
			DeleteFunc: func(deleteEvent event.DeleteEvent) bool {
				setupLog.Info("The Application has been deleted.", " name", deleteEvent.Object.GetName())
				return false
			},
			UpdateFunc: func(updateEvent event.UpdateEvent) bool {
				if updateEvent.ObjectNew.GetResourceVersion() == updateEvent.ObjectOld.GetResourceVersion() {
					return false
				}
				if reflect.DeepEqual(updateEvent.ObjectNew.(*dappsv1.Application).Spec, updateEvent.ObjectOld.(*dappsv1.Application).Spec) {
					return false
				}
				return true
			},
		})).
		Owns(&appsv1.Deployment{}, builder.WithPredicates(predicate.Funcs{
			CreateFunc: func(createEvent event.CreateEvent) bool {
				return false
			},
			DeleteFunc: func(deleteEvent event.DeleteEvent) bool {
				setupLog.Info("")
				return true
			},
			UpdateFunc: func(updateEvent event.UpdateEvent) bool {
				if updateEvent.ObjectNew.GetResourceVersion() == updateEvent.ObjectOld.GetResourceVersion() {
					return false
				}
				if reflect.DeepEqual(updateEvent.ObjectNew.(*appsv1.Deployment).Spec, updateEvent.ObjectOld.(*appsv1.Deployment).Spec) {

					return false
				}
				return true
			},
			GenericFunc: nil,
		})).
		Owns(&corev1.Service{}, builder.WithPredicates(predicate.Funcs{
			CreateFunc: func(createEvent event.CreateEvent) bool {
				return false
			},
			DeleteFunc: func(deleteEvent event.DeleteEvent) bool {
				setupLog.Info("The Service has been deleted", "name", deleteEvent.Object.GetName())
				return true
			},
			UpdateFunc: func(updateEvent event.UpdateEvent) bool {
				if updateEvent.ObjectNew.GetResourceVersion() == updateEvent.ObjectOld.GetResourceVersion() {
					return false
				}
				if reflect.DeepEqual(updateEvent.ObjectNew.(*dappsv1.Application).Spec, updateEvent.ObjectOld.(*dappsv1.Application).Spec) {
					return false
				}
				return true
			},
		})).
		Complete(r)
}

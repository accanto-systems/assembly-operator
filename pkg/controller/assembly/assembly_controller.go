package assembly

import (
	"context"
	"crypto/tls"
	"fmt"
	"time"

	"io/ioutil"

	"gopkg.in/yaml.v2"

	resty "github.com/go-resty/resty/v2"
	comv1alpha1 "github.com/orgs/accanto-systems/assembly-operator/pkg/apis/com/v1alpha1"

	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"

	// "k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"

	// "sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

var log = logf.Log.WithName("controller_service")

/**
* USER ACTION REQUIRED: This is a scaffold file intended for the user to modify with their own Controller
* business logic.  Delete these comments after modifying this file.*
 */

// Add creates a new Service Controller and adds it to the Manager. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

type LMConfiguration struct {
	LMUsername string `yaml:"lmUsername"`
	LMPassword string `yaml:"lmPassword"`
	LMBase     string `yaml:"lmBase"`
}

func getLMConfiguration() *LMConfiguration {
	yamlFile, err := ioutil.ReadFile("/var/assembly-operator/config.yaml")
	if err != nil {
		log.Info(fmt.Sprintf("yamlFile.Get err   #%v ", err))
	}
	configuration := LMConfiguration{}
	err = yaml.Unmarshal(yamlFile, &configuration)
	if err != nil {
		log.Info(fmt.Sprintf("Unmarshal: %v", err))
	}

	return &configuration
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	client := resty.New()
	client.SetTLSClientConfig(&tls.Config{InsecureSkipVerify: true})
	client.SetTimeout(2 * time.Minute)

	configuration := getLMConfiguration()
	log.Info(fmt.Sprintf("configuration: #%v", *configuration))

	return &ReconcileService{client: mgr.GetClient(), scheme: mgr.GetScheme(), restClient: client, ishtar: NewIshtar(client, configuration)}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New("service-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to primary resource Service
	err = c.Watch(&source.Kind{Type: &comv1alpha1.Assembly{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	// TODO(user): Modify this to be the types you create that are owned by the primary resource
	// Watch for changes to secondary resource Pods and requeue the owner Service
	err = c.Watch(&source.Kind{Type: &corev1.Pod{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &comv1alpha1.Assembly{},
	})
	if err != nil {
		return err
	}

	return nil
}

// blank assignment to verify that ReconcileService implements reconcile.Reconciler
var _ reconcile.Reconciler = &ReconcileService{}

// ReconcileService reconciles a Service object
type ReconcileService struct {
	// This client, initialized using mgr.Client() above, is a split client
	// that reads objects from the cache and writes to the apiserver
	client     client.Client
	scheme     *runtime.Scheme
	restClient *resty.Client
	ishtar     *Ishtar
}

func (r *ReconcileService) getAssembly(name types.NamespacedName) (*comv1alpha1.Assembly, error) {
	// Fetch the Service instance
	instance := &comv1alpha1.Assembly{}
	err := r.client.Get(context.TODO(), name, instance)
	if err != nil {
		// Error reading the object - requeue the request.
		return nil, err
	}

	return instance, nil
}

// Reconcile reads that state of the cluster for a Service object and makes changes based on the state read
// and what is in the Service.Spec
// TODO(user): Modify this Reconcile function to implement your Controller logic.  This example creates
// a Pod as an example
// Note:
// The Controller will requeue the Request to be processed again if the returned error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (r *ReconcileService) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	reqLogger := log.WithValues("Request.Namespace", request.Namespace, "Request.Name", request.Name)
	reqLogger.Info("Reconciling Service")

	// Fetch the Service instance
	instance, err := r.getAssembly(request.NamespacedName)
	// instance := &comv1alpha1.Assembly{}
	// err := r.client.Get(context.TODO(), request.NamespacedName, instance)
	if err != nil {
		if k8serrors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return reconcile.Result{}, err
	}

	reqLogger.Info(fmt.Sprintf("Assembly name: %s, ProcessID %s, Status %s", instance.Spec.AssemblyName, instance.Status.ProcessID, instance.Status.Status))

	if instance.Status.Status != "" {
		reqLogger.Info(fmt.Sprintf("Assembly %s created with status %s", instance.Spec.AssemblyName, instance.Status.Status))

		if instance.Status.Status == "Pending" || instance.Status.Status == "In Progress" {
			assemblyStatus, err := r.ishtar.GetAssemblyStatus(reqLogger, instance.Status.ProcessID)
			if err != nil {
				reqLogger.Error(err, "Failed to get service assembly status.")
				return reconcile.Result{}, err
			}

			instance1, err := r.getAssembly(request.NamespacedName)
			if err != nil {
				if k8serrors.IsNotFound(err) {
					// Request object not found, could have been deleted after reconcile request.
					// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
					// Return and don't requeue
					return reconcile.Result{}, nil
				}
				// Error reading the object - requeue the request.
				return reconcile.Result{}, err
			}

			instance1.Status.Status = assemblyStatus
			err = r.client.Status().Update(context.TODO(), instance1)
			if err != nil {
				reqLogger.Error(err, "Failed to update Service assembly status.")
				return reconcile.Result{}, err
			}
			log.Info(fmt.Sprintf("Service assembly status updated %s", assemblyStatus))

			if assemblyStatus == "Completed" || assemblyStatus == "Failed" || assemblyStatus == "Cancelled" {
				return reconcile.Result{}, nil
			}

			return reconcile.Result{Requeue: true}, nil
		}

		if instance.Status.Status == "Completed" || instance.Status.Status == "Failed" || instance.Status.Status == "Cancelled" {
			// done
			return reconcile.Result{}, nil
		}

		// re-queue
		return reconcile.Result{Requeue: true}, nil
	} else if instance.Status.ProcessID == "" {
		reqLogger.Info(fmt.Sprintf("No ProcessID, creating assembly with name %s", instance.Spec.AssemblyName))

		processID, err := r.ishtar.CreateAssembly(reqLogger, CreateAssemblyBody{
			AssemblyName:   instance.Spec.AssemblyName,
			DescriptorName: instance.Spec.DescriptorName,
			IntendedState:  instance.Spec.IntendedState,
			Properties:     instance.Spec.Properties,
		})
		if err != nil {
			reqLogger.Error(err, "Failed to create assembly.")

			instance.Status.Status = err.Error()
			err = r.client.Status().Update(context.TODO(), instance)
			if err != nil {
				reqLogger.Error(err, "Failed to update Service assembly status.")
				return reconcile.Result{}, err
			}
			log.Info("Service assembly status updated")

			return reconcile.Result{}, nil
		}
		instance.Status.ProcessID = processID
		err = r.client.Status().Update(context.TODO(), instance)
		if err != nil {
			reqLogger.Error(err, "Failed to update Service status.")
			return reconcile.Result{}, err
		}
		log.Info("Service status updated")

		// requeue
		return reconcile.Result{Requeue: true}, nil
	} else {
		reqLogger.Info(fmt.Sprintf("Assembly name %s", instance.Spec.AssemblyName))
		reqLogger.Info(fmt.Sprintf("ProcessID %s", instance.Status.ProcessID))
		reqLogger.Info(fmt.Sprintf("Get process %s", instance.Status.ProcessID))

		assemblyStatus, err := r.ishtar.GetAssemblyStatus(reqLogger, instance.Status.ProcessID)
		if err != nil {
			reqLogger.Error(err, "Failed to get service assembly status.")
			return reconcile.Result{}, err
		}

		instance1, err := r.getAssembly(request.NamespacedName)
		if err != nil {
			if k8serrors.IsNotFound(err) {
				// Request object not found, could have been deleted after reconcile request.
				// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
				// Return and don't requeue
				return reconcile.Result{}, nil
			}
			// Error reading the object - requeue the request.
			return reconcile.Result{}, err
		}

		instance1.Status.Status = assemblyStatus
		err = r.client.Status().Update(context.TODO(), instance1)
		if err != nil {
			reqLogger.Error(err, "Failed to update Service assembly status.")
			return reconcile.Result{}, err
		}
		log.Info("Service assembly status updated")

		if assemblyStatus == "Completed" || assemblyStatus == "Cancelled" || assemblyStatus == "Failed" {
			return reconcile.Result{}, nil
		}

		// requeue
		return reconcile.Result{Requeue: true}, nil
	}
}

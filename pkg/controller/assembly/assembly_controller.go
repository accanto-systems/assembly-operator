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

const assemblyFinalizer = "finalizer.com.accantosystems.stratoss"

// Reconcile reads that state of the cluster for a Service object and makes changes based on the state read
// and what is in the Service.Spec
// TODO(user): Modify this Reconcile function to implement your Controller logic.  This example creates
// a Pod as an example
// Note:
// The Controller will requeue the Request to be processed again if the returned error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (r *ReconcileService) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	reqLogger := log.WithValues("Request.Namespace", request.Namespace, "Request.Name", request.Name)
	reqLogger.Info(fmt.Sprintf("Reconciling Assembly %s", request.Name))

	// Fetch the Assembly resource instance from k8s
	instance, err := r.getAssembly(request.NamespacedName)
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

	reqLogger.Info(fmt.Sprintf("Processing Assembly %s [Transition %s, ProcessID %s, ProcessStatus %s, State %s, StateReason %s]", instance.Spec.AssemblyName, instance.Status.Transition, instance.Status.ProcessID, instance.Status.ProcessStatus, instance.Status.State, instance.Status.StateReason))

	//Does Assembly exist?
	if instance.Status.ID != "" || instance.Status.ProcessID != "" {
		// It should exist, make sure we have the ID and continue with the reconciliation
		if instance.Status.ID == "" {
			// Request to create Assembly passed but we failed to get the Assembly ID back
			reqLogger.Info(fmt.Sprintf("Fetching Process %s for Assembly %s (to determine lost ID)", instance.Status.ProcessID, instance.Spec.AssemblyName))
			process, err := r.ishtar.GetProcess(reqLogger, instance.Status.ProcessID)
			if err != nil {
				reqLogger.Error(err, fmt.Sprintf("Failed to get Process %s - will requeue reconcile request for %s", instance.Status.ProcessID, instance.Spec.AssemblyName))
				return reconcile.Result{}, err
			}
			instance.Status.ID = process.AssemblyID
			err = r.client.Status().Update(context.TODO(), instance)
			if err != nil {
				reqLogger.Error(err, fmt.Sprintf("Failed to update Assembly ID for %s", instance.Spec.AssemblyName))
				return reconcile.Result{}, err
			}
		}
	}else{
		//No ID or Process, must create Assembly (TODO: add logic to check if Assembly exists in LM, then re-create if not)
		reqLogger.Info(fmt.Sprintf("Requesting creation of Assembly %s", instance.Spec.AssemblyName))
		processID, err := r.ishtar.CreateAssembly(reqLogger, CreateAssemblyBody{
			AssemblyName:   instance.Spec.AssemblyName,
			DescriptorName: instance.Spec.DescriptorName,
			IntendedState:  instance.Spec.IntendedState,
			Properties:     instance.Spec.Properties,
		})
		if err != nil {
			reqLogger.Error(err, fmt.Sprintf("Failed to request creation of Assembly %s", instance.Spec.AssemblyName))
			instance.Status.State = "ERROR"
			instance.Status.StateReason = err.Error()
			err = r.client.Status().Update(context.TODO(), instance)
			if err != nil {
				reqLogger.Error(err, fmt.Sprintf("Failed to update Assembly status - will requeue reconcile request for %s", instance.Spec.AssemblyName))
				return reconcile.Result{}, err
			}
			//Ends reconciliation
			return reconcile.Result{}, nil
		}

		// Ensure finalizer for this Instance (so we can perform cleanup tasks on Delete)
		if !contains(instance.GetFinalizers(), assemblyFinalizer) {
			reqLogger.Info(fmt.Sprintf("Adding Finalizer for Assembly %s", instance.Spec.AssemblyName))
			instance.SetFinalizers(append(instance.GetFinalizers(), assemblyFinalizer))
			// Update CR
			err := r.client.Update(context.TODO(), instance)
			if err != nil {
				reqLogger.Error(err, fmt.Sprintf("Failed to update Assembly %s with finalizer", instance.Spec.AssemblyName))
				return reconcile.Result{}, err
			}
		}
		instance.Status.Transition = "Create"
		instance.Status.ProcessID = processID
		instance.Status.ProcessStatus = "Pending"
		instance.Status.State = "Pending"
		instance.Status.StateReason = ""
		err = r.client.Status().Update(context.TODO(), instance)
		if err != nil {
			//TODO that will attempt to re-create, which will fail as it has the same name - need to handle that.
			reqLogger.Error(err, fmt.Sprintf("Failed to update Assembly status - will requeue reconcile request for %s", instance.Spec.AssemblyName))
			return reconcile.Result{}, err
		}
		reqLogger.Info(fmt.Sprintf("Fetching Process %s for Assembly %s (to determine ID)", instance.Status.ProcessID, instance.Spec.AssemblyName))
		process, err := r.ishtar.GetProcess(reqLogger, instance.Status.ProcessID)
		if err != nil {
			reqLogger.Error(err, fmt.Sprintf("Failed to get Process %s - will requeue reconcile request for %s", instance.Status.ProcessID, instance.Spec.AssemblyName))
			return reconcile.Result{}, err
		}
		instance.Status.ID = process.AssemblyID
		err = r.client.Status().Update(context.TODO(), instance)
		if err != nil {
			reqLogger.Error(err, fmt.Sprintf("Failed to update Assembly status - will requeue reconcile request for %s", instance.Spec.AssemblyName))
			return reconcile.Result{}, err
		}
		// requeue to check on progress
		return reconcile.Result{Requeue: true}, nil
	}
	
	//Check for existing transition
	if instance.Status.ProcessStatus == "Pending" || instance.Status.ProcessStatus == "In Progress" {
		reqLogger.Info(fmt.Sprintf("Fetching Process %s for Assembly %s", instance.Status.ProcessID, instance.Spec.AssemblyName))
		process, err := r.ishtar.GetProcess(reqLogger, instance.Status.ProcessID)
		if err != nil {
			reqLogger.Error(err, fmt.Sprintf("Failed to get process %s - will requeue reconcile request for %s", instance.Status.ProcessID, instance.Spec.AssemblyName))
			return reconcile.Result{}, err
		}

		// Update based on process information
		instance.Status.ProcessStatus = process.Status
		instance.Status.StateReason = ""
		transitionFinished := false
		assemblyShouldExist := instance.Status.Transition != "Remove" || (process.Status == "Pending" || process.Status == "In Progress")
		if process.Status == "Completed" || process.Status == "Cancelled" || process.Status == "Failed" {
			transitionFinished = true
		}
		if process.Status == "Failed" {
			instance.Status.StateReason = process.StatusReason
		}

		//Update Assembly State
		if assemblyShouldExist {
			reqLogger.Info(fmt.Sprintf("Fetching Assembly instance %s", instance.Spec.AssemblyName))
			assembly, err := r.ishtar.GetAssembly(reqLogger, instance.Status.ID)
			if err != nil {
				reqLogger.Error(err, fmt.Sprintf("Failed to get assembly %s", instance.Spec.AssemblyName))
				instance.Status.State = "ERROR"
				instance.Status.StateReason = err.Error()
				err = r.client.Status().Update(context.TODO(), instance)
				if err != nil {
					reqLogger.Error(err, fmt.Sprintf("Failed to update Assembly status - will requeue reconcile request for %s", instance.Spec.AssemblyName))
					return reconcile.Result{}, err
				}
				//Ends reconciliation
				return reconcile.Result{}, nil
			}
			instance.Status.State = assembly.State
		}

		err = r.client.Status().Update(context.TODO(), instance)
		if err != nil {
			reqLogger.Error(err, fmt.Sprintf("Failed to update Assembly status - will requeue reconcile request for %s", instance.Spec.AssemblyName))
			return reconcile.Result{}, err
		}

		//Process not finished so requeue 
		if !transitionFinished {
			return reconcile.Result{Requeue: true}, nil
		}
	}

	//Is the Assembly resource being deleted?
	toBeDeleted := instance.GetDeletionTimestamp() != nil
	if toBeDeleted {
		reqLogger.Info(fmt.Sprintf("Assembly %s has deletion timestamp %s", instance.Spec.AssemblyName, instance.GetDeletionTimestamp()))
		if contains(instance.GetFinalizers(), assemblyFinalizer) {
			//Need to delete the Assembly in LM
			reqLogger.Info(fmt.Sprintf("Assembly %s includes finalizer, needs to be deleted (or check existing delete request)", instance.Spec.AssemblyName))

			if instance.Status.Transition == "Remove" && instance.Status.ProcessStatus == "Completed" {
				reqLogger.Info(fmt.Sprintf("Assembly %s removed, clearing finalizer", instance.Spec.AssemblyName))
				// We have already deleted the Assembly, remove the finalizer
				instance.SetFinalizers(remove(instance.GetFinalizers(), assemblyFinalizer))
				err = r.client.Update(context.TODO(), instance)
				if err != nil {
					reqLogger.Error(err, fmt.Sprintf("Failed to update Assembly finalizers - will requeue reconcile request for %s", instance.Spec.AssemblyName))
					return reconcile.Result{}, err
				}
				// Ends reconciliation
				return reconcile.Result{Requeue: true}, nil
			}else{
				// Trigger delete
				reqLogger.Info(fmt.Sprintf("Requesting removal of Assembly %s", instance.Spec.AssemblyName))
				reqLogger.Info(fmt.Sprintf("Fetching Assembly instance %s", instance.Spec.AssemblyName))
				assembly, err := r.ishtar.GetAssembly(reqLogger, instance.Status.ID)
				if err != nil {
					reqLogger.Error(err, fmt.Sprintf("Failed to get assembly %s", instance.Spec.AssemblyName))
					instance.Status.State = "ERROR"
					instance.Status.StateReason = err.Error()
					err = r.client.Status().Update(context.TODO(), instance)
					if err != nil {
						reqLogger.Error(err, fmt.Sprintf("Failed to update Assembly status - will requeue reconcile request for %s", instance.Spec.AssemblyName))
						return reconcile.Result{}, err
					}
					// Ends reconciliation
					return reconcile.Result{}, nil
				}

				processID, err := r.ishtar.DeleteAssembly(reqLogger, DeleteAssemblyBody{
					AssemblyName: instance.Spec.AssemblyName,
				})
				if err != nil {
					reqLogger.Error(err, fmt.Sprintf("Failed to request delete of Assembly %s", instance.Spec.AssemblyName))
					instance.Status.State = "ERROR"
					instance.Status.StateReason = err.Error()
					err = r.client.Status().Update(context.TODO(), instance)
					if err != nil {
						reqLogger.Error(err, fmt.Sprintf("Failed to update Assembly status - will requeue reconcile request for %s", instance.Spec.AssemblyName))
						return reconcile.Result{}, err
					}
					// Ends reconciliation
					return reconcile.Result{}, nil
				}
				instance.Status.Transition = "Remove"
				instance.Status.ProcessID = processID
				instance.Status.ProcessStatus = "Pending"
				instance.Status.State = assembly.State
				instance.Status.StateReason = ""
				err = r.client.Status().Update(context.TODO(), instance)
				if err != nil {
					reqLogger.Error(err, fmt.Sprintf("Failed to update Assembly status - will requeue reconcile request for %s", instance.Spec.AssemblyName))
					return reconcile.Result{}, err
				}
				// requeue to check on progress
				return reconcile.Result{Requeue: true}, nil
			}
		}
	}

	// Look for differences
	reqLogger.Info(fmt.Sprintf("Fetching Assembly instance %s", instance.Spec.AssemblyName))
	assembly, err := r.ishtar.GetAssembly(reqLogger, instance.Status.ID)
	if err != nil {
		reqLogger.Error(err, fmt.Sprintf("Failed to get assembly %s", instance.Spec.AssemblyName))
		instance.Status.State = "ERROR"
		instance.Status.StateReason = err.Error()
		err = r.client.Status().Update(context.TODO(), instance)
		if err != nil {
			reqLogger.Error(err, fmt.Sprintf("Failed to update Assembly status - will requeue reconcile request for %s", instance.Spec.AssemblyName))
			return reconcile.Result{}, err
		}
		// Ends reconciliation
		return reconcile.Result{}, nil
	}

	hasDifference := false
	hasStateChange := false
	if assembly.Name != instance.Spec.AssemblyName {
		//TODO - error on name change
	}
	if assembly.State != instance.Spec.IntendedState {
		hasStateChange = true
	}
	if assembly.DescriptorName != instance.Spec.DescriptorName {
		hasDifference = true
	}
	for k, v := range instance.Spec.Properties {
		for _, element := range assembly.Properties {
			if element.Name == k {
				if element.Value != v {
					hasDifference = true
				}
			}
		}
	}

	//Handle property differences before state changes
	if hasDifference {
		//Ready to apply differences
		reqLogger.Info(fmt.Sprintf("Requesting upgrade of Assembly %s", instance.Spec.AssemblyName))
		processID, err := r.ishtar.UpgradeAssembly(reqLogger, UpgradeAssemblyBody{
			AssemblyName:   assembly.Name,
			DescriptorName: instance.Spec.DescriptorName,
			Properties:     instance.Spec.Properties,
		})
		if err != nil {
			reqLogger.Error(err, fmt.Sprintf("Failed to request upgrade of Assembly %s", instance.Spec.AssemblyName))
			instance.Status.State = "ERROR"
			instance.Status.StateReason = err.Error()
			err = r.client.Status().Update(context.TODO(), instance)
			if err != nil {
				reqLogger.Error(err, fmt.Sprintf("Failed to update Assembly status - will requeue reconcile request for %s", instance.Spec.AssemblyName))
				return reconcile.Result{}, err
			}
			return reconcile.Result{}, nil
		}
		instance.Status.Transition = "Update"
		instance.Status.ProcessID = processID
		instance.Status.ProcessStatus = "Pending"
		instance.Status.State = assembly.State
		instance.Status.StateReason = ""
		err = r.client.Status().Update(context.TODO(), instance)
		if err != nil {
			reqLogger.Error(err, fmt.Sprintf("Failed to update Assembly status - will requeue reconcile request for %s", instance.Spec.AssemblyName))
			return reconcile.Result{}, err
		}
		// requeue to check on progress
		return reconcile.Result{Requeue: true}, nil
	}

	if hasStateChange {
		//Must change state
		reqLogger.Info(fmt.Sprintf("Requesting state change of Assembly %s to %s", instance.Spec.AssemblyName, instance.Spec.IntendedState))
		processID, err := r.ishtar.ChangeAssemblyState(reqLogger, ChangeAssemblyStateBody{
			AssemblyName: assembly.Name,
			IntendedState: instance.Spec.IntendedState,
		})
		if err != nil {
			reqLogger.Error(err, fmt.Sprintf("Failed to request state change of Assembly %s", instance.Spec.AssemblyName))
			instance.Status.State = "ERROR"
			instance.Status.StateReason = err.Error()
			err = r.client.Status().Update(context.TODO(), instance)
			if err != nil {
				reqLogger.Error(err, fmt.Sprintf("Failed to update Assembly status - will requeue reconcile request for %s", instance.Spec.AssemblyName))
				return reconcile.Result{}, err
			}
			return reconcile.Result{}, nil
		}
		instance.Status.Transition = "ChangeState"
		instance.Status.ProcessID = processID
		instance.Status.ProcessStatus = "Pending"
		instance.Status.State = assembly.State
		instance.Status.StateReason = ""
		err = r.client.Status().Update(context.TODO(), instance)
		if err != nil {
			reqLogger.Error(err, fmt.Sprintf("Failed to update Assembly status - will requeue reconcile request for %s", instance.Spec.AssemblyName))
			return reconcile.Result{}, err
		}
		// requeue to check on progress
		return reconcile.Result{Requeue: true}, nil
	}
	
	// Finally, ends reconciliation as there is nothing to check
	return reconcile.Result{}, nil
}

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

func remove(list []string, s string) []string {
	for i, v := range list {
		if v == s {
			list = append(list[:i], list[i+1:]...)
		}
	}
	return list
}
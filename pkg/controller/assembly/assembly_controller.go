package assembly

import (
	"context"
	"fmt"
	"time"

	lm "github.com/accanto/assembly-operator/internal/lm"
	stratossv1alpha1 "github.com/accanto/assembly-operator/pkg/apis/stratoss/v1alpha1"
	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

var log = logf.Log.WithName("controller_assembly")

const assemblyFinalizer = "finalizer.assemblies.stratoss.accantosystems.com"
const stateError = "ERROR"

type logKeys struct {
	AssemblyName          string
	AssemblyID            string
	ProcessID             string
	ProcessStatus         string
	IntendedState         string
	NumberOfErrors        string
	PropertyName          string
	DesiredPropertyValue  string
	ObservedPropertyName  string
	ObservedPropertyValue string
}

var LogKeys = &logKeys{
	AssemblyName:          "assemblyName",
	AssemblyID:            "assemblyId",
	ProcessID:             "processId",
	ProcessStatus:         "processStatus",
	IntendedState:         "intendedState",
	NumberOfErrors:        "errorCount",
	PropertyName:          "propertyName",
	DesiredPropertyValue:  "desiredPropertyValue",
	ObservedPropertyValue: "observedPropertyValue",
}

// Add creates a new Assembly Controller and adds it to the Manager. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	reconciler, err := newReconciler(mgr)
	if err != nil {
		return err
	}
	return add(mgr, reconciler)
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) (reconcile.Reconciler, error) {
	lmConfiguration, err := lm.ReadLMConfiguration()
	if err != nil {
		return &AssemblyReconciler{}, err
	}
	return &AssemblyReconciler{
		k8sClient: mgr.GetClient(),
		scheme:    mgr.GetScheme(),
		lmClient:  lm.BuildClient(lmConfiguration),
	}, nil
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New("assembly-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to primary resource Assembly
	err = c.Watch(&source.Kind{Type: &stratossv1alpha1.Assembly{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	// TODO(user): Modify this to be the types you create that are owned by the primary resource
	// Watch for changes to secondary resource Pods and requeue the owner Assembly
	err = c.Watch(&source.Kind{Type: &corev1.Pod{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &stratossv1alpha1.Assembly{},
	})
	if err != nil {
		return err
	}

	return nil
}

// blank assignment to verify that AssemblyReconciler implements reconcile.Reconciler
var _ reconcile.Reconciler = &AssemblyReconciler{}

// AssemblyReconciler reconciles a Assembly object
type AssemblyReconciler struct {
	// This client, initialized using mgr.Client() above, is a split client
	// that reads objects from the cache and writes to the apiserver
	k8sClient client.Client
	scheme    *runtime.Scheme
	lmClient  *lm.LMClient
}

// AssemblySynchronizer carries the state of a single reconcile call
type AssemblySynchronizer struct {
	k8sClient           client.Client
	k8sInstance         *stratossv1alpha1.Assembly
	lmClient            lm.LMClient
	logger              logr.Logger
	reconcileRequest    reconcile.Request
	stopSync            bool
	requeue             bool
	requeueDelay        int
	errors              []error
	updateError         bool
	hasFinalizerChanges bool
	needsStatusUpdate   bool
	newProcessStarted   bool
	isDeleted           bool
}

type LMSourceOfTruth struct {
	assemblyInstance      *lm.Assembly
	assemblyInstanceFound bool
	latestProcess         *lm.Process
	latestProcessFound    bool
}

func (sync *AssemblySynchronizer) onUpdateError(err error) (stopSync bool) {
	sync.errors = append(sync.errors, err)
	sync.stopSync = true
	sync.requeue = true
	sync.updateError = true
	return sync.stopSync
}

func (sync *AssemblySynchronizer) onLMError(err error) (stopSync bool) {
	sync.errors = append(sync.errors, err)
	sync.stopSync = true
	sync.requeue = true
	return sync.stopSync
}

func (sync *AssemblySynchronizer) fetchK8sInstance() (found bool, stopSync bool) {
	instance := &stratossv1alpha1.Assembly{}
	err := sync.k8sClient.Get(context.TODO(), sync.reconcileRequest.NamespacedName, instance)
	if err != nil {
		return false, sync.onUpdateError(err)
	}
	*sync.k8sInstance = (*instance)
	return true, false
}

func (sync *AssemblySynchronizer) updateK8sInstance() (stopSync bool) {
	err := sync.k8sClient.Update(context.TODO(), sync.k8sInstance)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			return true
		}
		sync.logger.Error(err, "Failed to update Assembly (CR)")
		return sync.onUpdateError(err)
	}
	return false
}

func (sync *AssemblySynchronizer) updateK8sInstanceStatus() (stopSync bool) {
	err := sync.k8sClient.Status().Update(context.TODO(), sync.k8sInstance)
	if err != nil {
		sync.logger.Error(err, "Failed to update Assembly (CR) status")
		return sync.onUpdateError(err)
	}
	return false
}

func (sync *AssemblySynchronizer) getLatestProcess() (process *lm.Process, found bool, stopSync bool) {
	sync.logger.Info("Fetching latest Process for Assembly")
	process, found, err := sync.lmClient.GetLatestProcess(sync.k8sInstance.Name)
	if err != nil {
		sync.logger.Error(err, "Failed to fetch latest Process for Assembly")
		return nil, false, sync.onLMError(err)
	}
	return process, found, false
}

func (sync *AssemblySynchronizer) getAssemblyByID(assemblyID string) (lmAssembly *lm.Assembly, found bool, stopSync bool) {
	sync.logger.Info("Fetching Assembly from LM")
	lmAssembly, found, err := sync.lmClient.GetAssemblyByID(assemblyID)
	if err != nil {
		sync.logger.Error(err, "Failed to fetch Assembly")
		return nil, false, sync.onLMError(err)
	}
	return lmAssembly, found, false
}

func (sync *AssemblySynchronizer) getAssemblyByName() (lmAssembly *lm.Assembly, found bool, stopSync bool) {
	sync.logger.Info("Fetching Assembly from LM")
	lmAssembly, found, err := sync.lmClient.GetAssemblyByName(sync.k8sInstance.Name)
	if err != nil {
		sync.logger.Error(err, "Failed to fetch Assembly")
		return nil, false, sync.onLMError(err)
	}
	return lmAssembly, found, false
}

func (sync *AssemblySynchronizer) fetchLMSourceOfTruth() (lmSourceOfTruth *LMSourceOfTruth, stopSync bool) {
	k8sInstance := sync.k8sInstance
	var assemblyInstance *lm.Assembly
	var assemblyInstanceFound bool
	var latestProcess *lm.Process
	var latestProcessFound bool
	// Fetch information about the Assembly
	if k8sInstance.Status.ID == "" {
		// No ID known, find by name
		assemblyInstance, assemblyInstanceFound, stopSync = sync.getAssemblyByName()
		if stopSync {
			return nil, stopSync
		} else if !assemblyInstanceFound {
			sync.logger.Info("Assembly not found by name in LM")
		} else {
			sync.logger.Info("Assembly found by name in LM")
		}
	} else {
		// Find by ID
		idLogger := sync.logger.WithValues(LogKeys.AssemblyID, k8sInstance.Status.ID)
		assemblyInstance, assemblyInstanceFound, stopSync = sync.getAssemblyByID(k8sInstance.Status.ID)
		if stopSync {
			return &LMSourceOfTruth{}, stopSync
		} else if !assemblyInstanceFound {
			idLogger.Info("Assembly not found by ID in LM")
		} else {
			idLogger.Info("Assembly found by ID in LM")
		}
	}

	// Fetch information about the latest Process
	latestProcess, latestProcessFound, stopSync = sync.getLatestProcess()
	if stopSync {
		return nil, stopSync
	} else if !latestProcessFound {
		sync.logger.Info("Latest process not found in LM")
	} else {
		//Ensure the process is for this Assembly (can be a previous one with the same name if recreating)
		if latestProcess.AssemblyID == assemblyInstance.ID {
			sync.logger.Info("Latest process found in LM")
		} else {
			sync.logger.Info("Latest process not found in LM")
		}

	}

	return &LMSourceOfTruth{
		assemblyInstance:      assemblyInstance,
		assemblyInstanceFound: assemblyInstanceFound,
		latestProcess:         latestProcess,
		latestProcessFound:    latestProcessFound,
	}, false
}

func (sync *AssemblySynchronizer) syncStatusWithLM() (stopSync bool) {
	lmSourceOfTruth, stopSync := sync.fetchLMSourceOfTruth()
	if stopSync {
		return stopSync
	}
	sync.logger.Info("Syncing state with data from LM")
	k8sInstance := sync.k8sInstance
	assemblyInstance := lmSourceOfTruth.assemblyInstance
	if !lmSourceOfTruth.assemblyInstanceFound {
		k8sInstance.Status.State = "NotFound"
		k8sInstance.Status.Properties = make(map[string]string)
	} else {
		k8sInstance.Status.ID = assemblyInstance.ID
		k8sInstance.Status.DescriptorName = assemblyInstance.DescriptorName
		if assemblyInstance.State != "" {
			k8sInstance.Status.State = assemblyInstance.State
		} else {
			k8sInstance.Status.State = "None"
		}
		k8sInstance.Status.Properties = make(map[string]string)
		for _, property := range assemblyInstance.Properties {
			k8sInstance.Status.Properties[property.Name] = property.Value
		}
	}

	latestProcess := lmSourceOfTruth.latestProcess

	k8sInstance.Status.LastProcess = stratossv1alpha1.Process{Status: "None", IntentType: "None"}
	if lmSourceOfTruth.latestProcessFound {
		k8sInstance.Status.LastProcess.ID = latestProcess.ID
		k8sInstance.Status.LastProcess.IntentType = translateIntentType(latestProcess.IntentType)
		k8sInstance.Status.LastProcess.Status = latestProcess.Status
		k8sInstance.Status.LastProcess.StatusReason = latestProcess.StatusReason
	}

	sync.needsStatusUpdate = true
	//stopSync = sync.updateK8sInstanceStatus()
	return false
}

func translateIntentType(incomingIntentType string) (outputIntentType string) {
	switch incomingIntentType {
	case "ChangeAssemblyState":
		return "ChangeState"
	case "CreateAssembly":
		return "Create"
	case "DeleteAssembly":
		return "Delete"
	case "HealAssembly":
		return "Heal"
	case "ScaleInAssembly":
		return "ScaleIn"
	case "ScaleOutAssembly":
		return "ScaleOut"
	case "UpgradeAssembly":
		return "Update"
	}
	return incomingIntentType
}

func processIsOngoing(processStatus string) bool {
	return processStatus == "Planned" || processStatus == "Pending" || processStatus == "In Progress"
}

func (sync *AssemblySynchronizer) checkForOngoingProcess() (stopSync bool) {
	sync.logger.Info("Checking for ongoing process")
	if sync.k8sInstance.Status.LastProcess.ID != "" {
		processLogger := sync.logger.WithValues(LogKeys.ProcessID, sync.k8sInstance.Status.LastProcess.ID)
		if processIsOngoing(sync.k8sInstance.Status.LastProcess.Status) {
			processLogger.Info("Process has not completed yet, will requeue reconcile", LogKeys.ProcessStatus, sync.k8sInstance.Status.LastProcess.Status)
			sync.requeue = true
			sync.requeueDelay = 5
			sync.stopSync = true
			return sync.stopSync
		} else {
			processLogger.Info("Process complete!", LogKeys.ProcessStatus, sync.k8sInstance.Status.LastProcess.Status)
		}
	} else {
		sync.logger.Info("No process associated to Assembly")
	}
	return false
}

func (sync *AssemblySynchronizer) syncExistence() (stopSync bool) {
	k8sInstance := sync.k8sInstance
	isDeleting := k8sInstance.GetDeletionTimestamp() != nil
	if isDeleting {
		sync.logger.Info("Assembly (CR) has deletion timestamp")
		if finalizerContains(k8sInstance.GetFinalizers(), assemblyFinalizer) {
			if k8sInstance.Status.State == "NotFound" {
				sync.logger.Info("Assembly no longer exists in LM, safe to remove finalizer and delete K8s instance")
				k8sInstance.SetFinalizers(finalizerRemove(k8sInstance.GetFinalizers(), assemblyFinalizer))
				sync.hasFinalizerChanges = true
				sync.isDeleted = true
				sync.stopSync = true
				return sync.stopSync
			} else {
				//Trigger delete
				processID, err := sync.lmClient.DeleteAssembly(lm.DeleteAssemblyRequest{
					AssemblyName: k8sInstance.Name,
				})
				if err != nil {
					sync.logger.Error(err, "Failed to request deletion of Assembly")
					return sync.onLMError(err)
				} else {
					sync.logger.Info("Delete Assembly request accepted", LogKeys.ProcessID, processID)
					sync.needsStatusUpdate = true
					sync.newProcessStarted = true
					// Requeue request to check progress
					sync.requeue = true
					sync.requeueDelay = 5
					sync.stopSync = true
					return sync.stopSync
				}
			}
		} else {
			//Finaliser already removed, allow deletion of k8s instance
			sync.stopSync = true
			sync.isDeleted = true
			return sync.stopSync
		}
	} else {
		// Ensure the finalizer is set on K8s instance
		if !finalizerContains(k8sInstance.GetFinalizers(), assemblyFinalizer) {
			sync.logger.Info("Adding Finalizer to Assembly")
			k8sInstance.SetFinalizers(append(k8sInstance.GetFinalizers(), assemblyFinalizer))
			sync.hasFinalizerChanges = true
		}

		// Create if not found
		if k8sInstance.Status.State == "NotFound" {
			sync.logger.Info("Requesting creation of Assembly")
			processID, err := sync.lmClient.CreateAssembly(lm.CreateAssemblyRequest{
				AssemblyName:   k8sInstance.Name,
				DescriptorName: k8sInstance.Spec.DescriptorName,
				IntendedState:  k8sInstance.Spec.IntendedState,
				Properties:     k8sInstance.Spec.Properties,
			})
			if err != nil {
				sync.logger.Error(err, "Failed to request creation of Assembly")
				return sync.onLMError(err)
			} else {
				sync.logger.Info("Create Assembly request accepted", LogKeys.ProcessID, processID)
				sync.needsStatusUpdate = true
				sync.newProcessStarted = true
				// Requeue request to check progress
				sync.requeue = true
				sync.requeueDelay = 5
				sync.stopSync = true
				return sync.stopSync
			}
		}
	}
	return false
}

func (sync *AssemblySynchronizer) syncAssemblyState() (stopSync bool) {
	k8sInstance := sync.k8sInstance
	if k8sInstance.Status.State != k8sInstance.Spec.IntendedState {
		//State change
		sync.logger.Info("Requesting state change of Assembly", LogKeys.IntendedState, k8sInstance.Spec.IntendedState)
		processID, err := sync.lmClient.ChangeAssemblyState(lm.ChangeAssemblyStateRequest{
			AssemblyName:  k8sInstance.Name,
			IntendedState: k8sInstance.Spec.IntendedState,
		})
		if err != nil {
			sync.logger.Error(err, "Failed to request change state for Assembly")
			return sync.onLMError(err)
		} else {
			sync.logger.Info("Change Assembly state request accepted", LogKeys.ProcessID, processID, LogKeys.IntendedState, k8sInstance.Spec.IntendedState)
			sync.needsStatusUpdate = true
			sync.newProcessStarted = true
			// Requeue request to check progress
			sync.requeue = true
			sync.requeueDelay = 5
			sync.stopSync = true
			return sync.stopSync
		}
	}
	return false
}

func (sync *AssemblySynchronizer) syncAssemblyUpdateableState() (stopSync bool) {
	k8sInstance := sync.k8sInstance
	hasDifference := false
	if k8sInstance.Status.DescriptorName != k8sInstance.Spec.DescriptorName {
		sync.logger.Info("Desired Assembly descriptorName differs from current state")
		hasDifference = true
	}
	if !hasDifference {
		for propName, specPropValue := range k8sInstance.Spec.Properties {
			statusPropValue := k8sInstance.Status.Properties[propName]
			if statusPropValue != specPropValue {
				sync.logger.Info("Desired Assembly property values differ from current state", LogKeys.PropertyName, propName, LogKeys.DesiredPropertyValue, specPropValue, LogKeys.ObservedPropertyValue, statusPropValue)
				hasDifference = true
				break
			}
		}
	}
	if hasDifference {
		// Upgrade
		sync.logger.Info("Requesting update of Assembly")
		processID, err := sync.lmClient.UpgradeAssembly(lm.UpgradeAssemblyRequest{
			AssemblyName:   k8sInstance.Name,
			DescriptorName: k8sInstance.Spec.DescriptorName,
			Properties:     k8sInstance.Spec.Properties,
		})
		if err != nil {
			sync.logger.Error(err, "Failed to request update for Assembly")
			return sync.onLMError(err)
		} else {
			sync.logger.Info("Update Assembly request accepted", LogKeys.ProcessID, processID)
			sync.needsStatusUpdate = true
			sync.newProcessStarted = true
			// Requeue request to check progress
			sync.requeue = true
			sync.requeueDelay = 5
			sync.stopSync = true
			return sync.stopSync
		}
	}
	return false
}

func (sync *AssemblySynchronizer) endReconcile() (reconcile.Result, error) {
	if !sync.isDeleted {
		if sync.newProcessStarted {
			sync.logger.Info("New process started on this Assembly, synchronizing with latest LM state")
			if stopSync := sync.syncStatusWithLM(); stopSync {
				sync.logger.Info("Failed to read latest LM state")
			} else {
				sync.needsStatusUpdate = true
			}
		}
	}

	numberOfErrors := len(sync.errors)
	var lastError error = nil
	if numberOfErrors > 0 {
		sync.logger.Info("Reconcile encountered errors", LogKeys.NumberOfErrors, numberOfErrors)
		lastError = sync.errors[numberOfErrors-1]
	} else if sync.requeue {
		sync.logger.Info("Reconile had no errors but it must be requeued")
	}

	previousStatus := sync.k8sInstance.Status.SyncState.Status
	previousAttempts := sync.k8sInstance.Status.SyncState.Attempts
	previousError := sync.k8sInstance.Status.SyncState.Error
	//Reset SyncState
	sync.k8sInstance.Status.SyncState = stratossv1alpha1.SyncState{Status: "OK"}
	if lastError != nil {
		errStr := lastError.Error()
		sync.k8sInstance.Status.SyncState.Status = "ERROR"
		sync.k8sInstance.Status.SyncState.Error = errStr
		sync.needsStatusUpdate = true
		if errStr == previousError {
			//Same error as last time
			sync.k8sInstance.Status.SyncState.Attempts = previousAttempts + 1
		} else {
			//Different error
			sync.k8sInstance.Status.SyncState.Attempts = 1
		}
	} else if previousStatus != "OK" {
		// No errors, only update if the last sync status reported an error
		sync.needsStatusUpdate = true
	}

	//Now update the K8s instance with all the changes from this reconcile
	updateReportedStop := false
	//Make a copy of the finalizers now incase update returns data from server
	finalizers := make([]string, len(sync.k8sInstance.GetFinalizers()))
	copy(finalizers, sync.k8sInstance.GetFinalizers())
	if sync.needsStatusUpdate && !sync.isDeleted {
		sync.logger.Info("Updating Assembly (CR) status")
		updateReportedStop = sync.updateK8sInstanceStatus()
	}
	if sync.hasFinalizerChanges {
		sync.logger.Info("Updating Assembly (CR) instance finalizers")
		sync.k8sInstance.SetFinalizers(finalizers)
		updateInstanceReportedStop := sync.updateK8sInstance()
		updateReportedStop = updateReportedStop || updateInstanceReportedStop
	}

	if updateReportedStop && len(sync.errors) != numberOfErrors {
		numberOfErrors = len(sync.errors)
		//New update errors occurred, requeue
		sync.logger.Info("Reconcile encountered errors whilst trying to update the K8s instance/status", LogKeys.NumberOfErrors, numberOfErrors)
		if lastError == nil {
			lastError = sync.errors[numberOfErrors-1]
		}
	}

	res := reconcile.Result{Requeue: sync.requeue, RequeueAfter: time.Duration(sync.requeueDelay) * time.Second}
	sync.logger.Info(fmt.Sprintf("Reconcile result: %+v, Reconcile error: %+v", res, lastError))
	return res, lastError
}

// Reconcile reads that state of the cluster for a Assembly object and makes changes based on the state read
// and what is in the Assembly.Spec
// Note:
// The Controller will requeue the Request to be processed again if the returned error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (r *AssemblyReconciler) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	reqLogger := log.WithValues("Request.Namespace", request.Namespace, "Request.Name", request.Name)
	reqLogger.Info("Reconciling Assembly")

	// Fetch the Assembly instance
	instance := &stratossv1alpha1.Assembly{}
	err := r.k8sClient.Get(context.TODO(), request.NamespacedName, instance)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return reconcile.Result{}, err
	}

	syncLogger := reqLogger.WithValues(LogKeys.AssemblyName, instance.Name)

	sync := &AssemblySynchronizer{
		k8sClient:        r.k8sClient,
		k8sInstance:      instance,
		lmClient:         *r.lmClient,
		logger:           syncLogger,
		reconcileRequest: request,
		stopSync:         false,
		requeue:          false,
	}

	if stopSync := sync.syncStatusWithLM(); stopSync {
		return sync.endReconcile()
	}

	if stopSync := sync.checkForOngoingProcess(); stopSync {
		return sync.endReconcile()
	}

	if stopSync := sync.syncExistence(); stopSync {
		return sync.endReconcile()
	}

	if stopSync := sync.syncAssemblyState(); stopSync {
		return sync.endReconcile()
	}

	if stopSync := sync.syncAssemblyUpdateableState(); stopSync {
		return sync.endReconcile()
	}

	return sync.endReconcile()
}

func finalizerContains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

func finalizerRemove(list []string, s string) []string {
	for i, v := range list {
		if v == s {
			list = append(list[:i], list[i+1:]...)
		}
	}
	return list
}

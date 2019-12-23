package lm

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	logf "sigs.k8s.io/controller-runtime/pkg/log"

	resty "github.com/go-resty/resty/v2"
)

var clientLog = logf.Log.WithName("lm_client")

// --LM Client--
type LMClient struct {
	restClient      *resty.Client
	lmConfiguration *LMConfiguration
	securityCtrl    *LMSecurityCtrl
}

func BuildClient(lmConfiguration *LMConfiguration) *LMClient {
	clientLog.Info("Building LM client", LogKeys.URL, lmConfiguration.Base, LogKeys.Client, lmConfiguration.Client, LogKeys.Secure, lmConfiguration.Secure)
	lmSecurityCtrl := BuildCtrl(lmConfiguration)
	restClient := resty.New()
	restClient.SetTLSClientConfig(&tls.Config{InsecureSkipVerify: true})
	restClient.SetTimeout(2 * time.Minute)
	return &LMClient{
		restClient:      restClient,
		lmConfiguration: lmConfiguration,
		securityCtrl:    lmSecurityCtrl,
	}
}

func (client *LMClient) addAuthenticationHeaders(request *resty.Request) error {
	accessToken, err := client.securityCtrl.getAccessToken()
	if err != nil {
		clientLog.Error(err, "Unable to get access token")
		return err
	}
	request.SetHeader("Authorization", fmt.Sprintf("Bearer %s", accessToken))
	return nil
}

func (client *LMClient) startRequest() (*resty.Request, error) {
	request := client.restClient.R()
	err := client.addAuthenticationHeaders(request)
	return request, err
}

// API methods
const createAssemblyAPI = "/api/intent/createAssembly"
const changeAssemblyStateAPI = "/api/intent/changeAssemblyState"
const deleteAssemblyAPI = "/api/intent/deleteAssembly"
const upgradeAssemblyAPI = "/api/intent/upgradeAssembly"
const assemblyTopologyAPI = "/api/topology/assemblies"
const processAPI = "/api/processes"

func (client *LMClient) executeProcess(requestJSON string, processAPI string, processType string) (processID string, err error) {
	url := fmt.Sprintf("%s%s", client.lmConfiguration.Base, processAPI)
	requestLogger := clientLog.WithValues(LogKeys.URL, url, LogKeys.Body, requestJSON)
	requestLogger.Info(fmt.Sprintf("Sending request: %s", processType))
	req, err := client.startRequest()
	if err != nil {
		requestLogger.Error(err, fmt.Sprintf("Unable to build request"))
		return "", err
	}
	resp, err := req.
		EnableTrace().
		SetBody(requestJSON).
		SetHeader("Content-Type", "application/json").
		Post(url)
	if err != nil {
		requestLogger.Error(err, fmt.Sprintf("Unable to %s", processType))
		return "", err
	}
	requestLogger.Info(fmt.Sprintf("%s request returned", processType), LogKeys.ResponseStatusCode, resp.StatusCode())
	if resp.StatusCode() != http.StatusCreated {
		return "", &LMClientError{
			prefix:       fmt.Sprintf("%s request returned an unexpected result", processType),
			ResponseBody: string(resp.Body()),
			StatusCode:   resp.StatusCode(),
		}
	}
	location := resp.Header().Get(http.CanonicalHeaderKey("Location"))
	split := strings.Split(location, "/")
	processID = split[len(split)-1]
	return processID, nil
}

func (client *LMClient) CreateAssembly(createRequest CreateAssemblyRequest) (processID string, err error) {
	bytes, err := json.Marshal(createRequest)
	if err != nil {
		clientLog.Error(err, "Unable to parse JSON for Create Assembly request")
		return "", err
	}
	requestJSON := string(bytes)
	return client.executeProcess(requestJSON, createAssemblyAPI, "Create Assembly")
}

func (client *LMClient) UpgradeAssembly(upgradeRequest UpgradeAssemblyRequest) (processID string, err error) {
	bytes, err := json.Marshal(upgradeRequest)
	if err != nil {
		clientLog.Error(err, "Unable to parse JSON for Upgrade Assembly request")
		return "", err
	}
	requestJSON := string(bytes)
	return client.executeProcess(requestJSON, upgradeAssemblyAPI, "Upgrade Assembly")
}

func (client *LMClient) ChangeAssemblyState(changeStateRequest ChangeAssemblyStateRequest) (processID string, err error) {
	bytes, err := json.Marshal(changeStateRequest)
	if err != nil {
		clientLog.Error(err, "Unable to parse JSON for Change Assembly State request")
		return "", err
	}
	requestJSON := string(bytes)
	return client.executeProcess(requestJSON, changeAssemblyStateAPI, "Change Assembly State")
}

func (client *LMClient) DeleteAssembly(deleteRequest DeleteAssemblyRequest) (processID string, err error) {
	bytes, err := json.Marshal(deleteRequest)
	if err != nil {
		clientLog.Error(err, "Unable to parse JSON for Delete Assembly request")
		return "", err
	}
	requestJSON := string(bytes)
	return client.executeProcess(requestJSON, deleteAssemblyAPI, "Delete Assembly")
}

func (client *LMClient) GetAssemblyByID(assemblyID string) (*Assembly, bool, error) {
	url := fmt.Sprintf("%s%s/%s", client.lmConfiguration.Base, assemblyTopologyAPI, assemblyID)
	requestLogger := clientLog.WithValues(LogKeys.URL, url, LogKeys.AssemblyID, assemblyID)
	requestLogger.Info("Sending request to retrieve Assembly instance by ID")
	result := &Assembly{}
	req, err := client.startRequest()
	if err != nil {
		requestLogger.Error(err, fmt.Sprintf("Unable to build request"))
		return result, false, err
	}
	resp, err := req.
		EnableTrace().
		SetResult(result).
		SetHeader("Content-Type", "application/json").
		Get(url)
	if err != nil {
		requestLogger.Error(err, "Unable to retrieve Assembly")
		return result, false, err
	}
	requestLogger.Info("Retrieve Assembly request returned", LogKeys.ResponseStatusCode, resp.StatusCode())
	if resp.StatusCode() == http.StatusNotFound {
		return result, false, nil
	} else if resp.StatusCode() != http.StatusOK {
		return result, false, &LMClientError{
			prefix:       fmt.Sprintf("Retrieve Assembly (ID=%s) request returned an unexpected result", assemblyID),
			ResponseBody: string(resp.Body()),
			StatusCode:   resp.StatusCode(),
		}
	} else {
		return &(*resp.Result().(*Assembly)), true, nil
	}
}

func (client *LMClient) GetAssemblyByName(assemblyName string) (*Assembly, bool, error) {
	url := fmt.Sprintf("%s%s?name=%s", client.lmConfiguration.Base, assemblyTopologyAPI, assemblyName)
	requestLogger := clientLog.WithValues(LogKeys.URL, url, LogKeys.AssemblyName, assemblyName)
	requestLogger.Info("Sending request to retrieve Assembly instance by name")
	result := make([]Assembly, 1)
	req, err := client.startRequest()
	if err != nil {
		requestLogger.Error(err, fmt.Sprintf("Unable to build request"))
		return &Assembly{}, false, err
	}
	resp, err := req.
		EnableTrace().
		SetResult(result).
		SetHeader("Content-Type", "application/json").
		Get(url)
	if err != nil {
		requestLogger.Error(err, "Unable to retrieve Assembly")
		return &Assembly{}, false, err
	}
	requestLogger.Info("Retrieve Assembly request returned", LogKeys.ResponseStatusCode, resp.StatusCode())
	if resp.StatusCode() != http.StatusOK {
		return &Assembly{}, false, &LMClientError{
			prefix:       fmt.Sprintf("Retrieve Assembly (Name=%s) request returned an unexpected result", assemblyName),
			ResponseBody: string(resp.Body()),
			StatusCode:   resp.StatusCode(),
		}
	} else {
		listOfAssemblies := (*resp.Result().(*[]Assembly))
		if len(listOfAssemblies) == 0 {
			return &Assembly{}, false, nil
		}
		assembly := listOfAssemblies[0]
		return &assembly, true, nil
	}
}

func (client *LMClient) GetLatestProcess(assemblyName string) (*Process, bool, error) {
	url := fmt.Sprintf("%s%s?assemblyName=%s&limit=1", client.lmConfiguration.Base, processAPI, assemblyName)
	requestLogger := clientLog.WithValues(LogKeys.URL, url, LogKeys.AssemblyName, assemblyName)
	requestLogger.Info("Sending request to retrieve latest Process instance for Assembly")
	result := make([]Process, 1)
	req, err := client.startRequest()
	if err != nil {
		requestLogger.Error(err, "Unable to build request")
		return &Process{}, false, err
	}
	resp, err := req.
		EnableTrace().
		SetResult(result).
		SetHeader("Content-Type", "application/json").
		Get(url)
	if err != nil {
		requestLogger.Error(err, "Unable to retrieve latest Process")
		return &Process{}, false, err
	}
	requestLogger.Info("Retrieve latest Process request returned", LogKeys.ResponseStatusCode, resp.StatusCode())
	if resp.StatusCode() != http.StatusOK {
		return &Process{}, false, &LMClientError{
			prefix:       fmt.Sprintf("Retrieve latest Process (AssemblyName=%s) request returned an unexpected result", assemblyName),
			ResponseBody: string(resp.Body()),
			StatusCode:   resp.StatusCode(),
		}
	} else {
		listOfProcesses := (*resp.Result().(*[]Process))
		if len(listOfProcesses) == 0 {
			return &Process{}, false, nil
		}
		process := listOfProcesses[0]
		return &process, true, nil
	}
}

func (client *LMClient) GetProcessByID(processID string) (*Process, bool, error) {
	url := fmt.Sprintf("%s%s/%s", client.lmConfiguration.Base, processAPI, processID)
	requestLogger := clientLog.WithValues(LogKeys.URL, url, LogKeys.ProcessID, processID)
	requestLogger.Info("Sending request to retrieve Process by ID")
	result := &Process{}
	req, err := client.startRequest()
	if err != nil {
		requestLogger.Error(err, fmt.Sprintf("Unable to build request"))
		return result, false, err
	}
	resp, err := req.
		EnableTrace().
		SetResult(result).
		SetHeader("Content-Type", "application/json").
		Get(url)
	if err != nil {
		requestLogger.Error(err, "Unable to retrieve Process by ID")
		return result, false, err
	}
	requestLogger.Info("Retrieve Process by ID request returned", LogKeys.ResponseStatusCode, resp.StatusCode())
	if resp.StatusCode() == http.StatusNotFound {
		return result, false, nil
	} else if resp.StatusCode() != http.StatusOK {
		return result, false, &LMClientError{
			prefix:       fmt.Sprintf("Retrieve Process (ID=%s) request returned an unexpected result", processID),
			ResponseBody: string(resp.Body()),
			StatusCode:   resp.StatusCode(),
		}
	} else {
		return &(*resp.Result().(*Process)), true, nil
	}
}

// DTOs
type CreateAssemblyRequest struct {
	AssemblyName   string            `json:"assemblyName"`
	DescriptorName string            `json:"descriptorName"`
	IntendedState  string            `json:"intendedState"`
	Properties     map[string]string `json:"properties"`
}

type UpgradeAssemblyRequest struct {
	AssemblyName   string            `json:"assemblyName"`
	DescriptorName string            `json:"descriptorName"`
	Properties     map[string]string `json:"properties"`
}

type ChangeAssemblyStateRequest struct {
	AssemblyName  string `json:"assemblyName"`
	IntendedState string `json:"intendedState"`
}

type DeleteAssemblyRequest struct {
	AssemblyName string `json:"assemblyName"`
}

type Process struct {
	ID           string `json:"id"`
	AssemblyID   string `json:"assemblyId"`
	IntentType   string `json:"intentType"`
	Status       string `json:"status"`
	StatusReason string `json:"statusReason"`
}

type Assembly struct {
	ID             string             `json:"id"`
	Name           string             `json:"name"`
	State          string             `json:"state"`
	DescriptorName string             `json:"descriptorName"`
	Properties     []AssemblyProperty `json:"properties"`
}

type AssemblyProperty struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

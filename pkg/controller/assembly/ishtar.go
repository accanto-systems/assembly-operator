package assembly

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"strings"
	"time"

	"github.com/go-logr/logr"
	resty "github.com/go-resty/resty/v2"
)

type Auth struct {
	AccessToken  string
	RefreshToken string
	ExpiresIn    int32
	Scope        string
}

type LMSecurityCtrl struct {
	restClient  *resty.Client
	lmBase      string
	username    string
	password    string
	loginResult *Auth
	loginTime   time.Time
}

type Ishtar struct {
	restClient     *resty.Client
	LMSecurityCtrl *LMSecurityCtrl
}

type CreateAssemblyBody struct {
	AssemblyName   string            `json:"assemblyName"`
	DescriptorName string            `json:"descriptorName"`
	IntendedState  string            `json:"intendedState"`
	Properties     map[string]string `json:"properties"`
}

type UpgradeAssemblyBody struct {
	AssemblyName   string            `json:"assemblyName"`
	DescriptorName string            `json:"descriptorName"`
	Properties     map[string]string `json:"properties"`
}

type ChangeAssemblyStateBody struct {
	AssemblyName   string            `json:"assemblyName"`
	IntendedState string            `json:"intendedState"`
}

type DeleteAssemblyBody struct {
	AssemblyName   string            `json:"assemblyName"`
}

func NewIshtar(client *resty.Client, lmConfiguration *LMConfiguration) *Ishtar {
	log.Info(fmt.Sprintf("NewIshtar %s %s %s ", lmConfiguration.LMBase, lmConfiguration.LMUsername, lmConfiguration.LMPassword))
	lmSecurityCtrl := LMSecurityCtrl{
		restClient: client,
		lmBase:     lmConfiguration.LMBase,
		username:   lmConfiguration.LMUsername,
		password:   lmConfiguration.LMPassword,
	}
	return &Ishtar{
		restClient:     client,
		LMSecurityCtrl: &lmSecurityCtrl,
	}
}

type login struct {
	Username string
	Password string
}

type HealthStatus struct {
	Status string
}

func (c *LMSecurityCtrl) login(username string, password string) (*Auth, error) {
	url := fmt.Sprintf("%s/ui/api/login", c.lmBase)
	data := login{
		Username: username,
		Password: password,
	}

	t, err := template.New("login").Parse(`{"username":"{{.Username}}", "password":"{{.Password}}"}`)
	if err != nil {
		return nil, err
	}

	var postTpl bytes.Buffer
	if err := t.Execute(&postTpl, data); err != nil {
		return nil, err
	}

	body := postTpl.String()
	log.Info(fmt.Sprintf("Login %s", body))

	resp, err := c.restClient.R().
		EnableTrace().
		SetResult(&Auth{}).
		SetHeader("Content-Type", "application/json").
		SetBody(body).
		Post(url)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode() == http.StatusOK {
		loginResult := (*resp.Result().(*Auth))
		return &loginResult, nil
	}

	return nil, fmt.Errorf("%s", resp)
}

func (c *LMSecurityCtrl) getAccessToken() (string, error) {
	if c.needNewToken() {
		log.Info("Requesting new access token")
		result, err := c.login(c.username, c.password)
		if err != nil {
			return "", err
		}

		c.loginResult = result
		c.loginTime = time.Now()
	}

	return c.loginResult.AccessToken, nil
}

func (c *LMSecurityCtrl) needNewToken() bool {
	if c.loginResult == nil {
		log.Info("No current access token, must request one")
		return true
	}

	log.Info("Checking if access token has expired")
	expirationSeconds := c.loginResult.ExpiresIn
	loggedInTime := time.Now().Sub(c.loginTime).Seconds()
	log.Info(fmt.Sprintf("Logged in for %f seconds, token had an expiration time of %d seconds", loggedInTime, expirationSeconds))
	if int(loggedInTime) >= int(expirationSeconds) {
		log.Info("Token expired, must request a new one")
		return true
	}
	// If the token expires within 1 second, wait and get a new one
	if int32(loggedInTime) >= (expirationSeconds - 1) {
		log.Info("Expires in less than 1 second, waiting before requesting a new Token")
		time.Sleep(2 * time.Second)
		return true
	}

	return false
}

func (i *Ishtar) CreateAssembly(reqLogger logr.Logger, assembly CreateAssemblyBody) (string, error) {
	accessToken, err := i.LMSecurityCtrl.getAccessToken()
	if err != nil {
		reqLogger.Error(err, "Unable to get access token")
		return "", err
	}

	bytes, err := json.Marshal(assembly)
	if err != nil {
		reqLogger.Error(err, "Unable to create assembly template")
		return "", err
	}
	assemblyJSON := string(bytes)

	reqLogger.Info(fmt.Sprintf("Create assembly %s", assemblyJSON))
	reqLogger.Info(fmt.Sprintf("Access token %s", accessToken))

	resp, err := i.restClient.R().
		EnableTrace().
		SetBody(assemblyJSON).
		SetHeader("Content-Type", "application/json").
		SetHeader("Authorization", fmt.Sprintf("Bearer %s", accessToken)).
		Post("https://ishtar:8280/api/intent/createAssembly")
	if err != nil {
		reqLogger.Error(err, "Unable to create assembly")
		return "", err
	}

	reqLogger.Info(fmt.Sprintf("Create assembly status %d", resp.StatusCode()))

	if resp.StatusCode() != http.StatusCreated {
		return "", fmt.Errorf("Create assembly failed %s %s", resp.Body(), string(resp.StatusCode()))
	}

	location := resp.Header().Get(http.CanonicalHeaderKey("Location"))
	ss := strings.Split(location, "/")
	return ss[len(ss)-1], nil
}

func (i *Ishtar) UpgradeAssembly(reqLogger logr.Logger, assembly UpgradeAssemblyBody) (string, error) {
	accessToken, err := i.LMSecurityCtrl.getAccessToken()
	if err != nil {
		reqLogger.Error(err, "Unable to get access token")
		return "", err
	}

	bytes, err := json.Marshal(assembly)
	if err != nil {
		reqLogger.Error(err, "Unable to create upgrade assembly template")
		return "", err
	}
	assemblyJSON := string(bytes)

	reqLogger.Info(fmt.Sprintf("Modify assembly %s", assemblyJSON))
	reqLogger.Info(fmt.Sprintf("Access token %s", accessToken))

	resp, err := i.restClient.R().
		EnableTrace().
		SetBody(assemblyJSON).
		SetHeader("Content-Type", "application/json").
		SetHeader("Authorization", fmt.Sprintf("Bearer %s", accessToken)).
		Post("https://ishtar:8280/api/intent/upgradeAssembly")
	if err != nil {
		reqLogger.Error(err, "Unable to modify assembly")
		return "", err
	}

	reqLogger.Info(fmt.Sprintf("Modify assembly status %d", resp.StatusCode()))

	if resp.StatusCode() != http.StatusCreated {
		return "", fmt.Errorf("Modify assembly failed %s %s", resp.Body(), string(resp.StatusCode()))
	}

	location := resp.Header().Get(http.CanonicalHeaderKey("Location"))
	ss := strings.Split(location, "/")
	return ss[len(ss)-1], nil
}

func (i *Ishtar) ChangeAssemblyState(reqLogger logr.Logger, assembly ChangeAssemblyStateBody) (string, error) {
	accessToken, err := i.LMSecurityCtrl.getAccessToken()
	if err != nil {
		reqLogger.Error(err, "Unable to get access token")
		return "", err
	}

	bytes, err := json.Marshal(assembly)
	if err != nil {
		reqLogger.Error(err, "Unable to change assembly state template")
		return "", err
	}
	assemblyJSON := string(bytes)

	reqLogger.Info(fmt.Sprintf("Changing assembly state %s", assemblyJSON))
	reqLogger.Info(fmt.Sprintf("Access token %s", accessToken))

	resp, err := i.restClient.R().
		EnableTrace().
		SetBody(assemblyJSON).
		SetHeader("Content-Type", "application/json").
		SetHeader("Authorization", fmt.Sprintf("Bearer %s", accessToken)).
		Post("https://ishtar:8280/api/intent/changeAssemblyState")
	if err != nil {
		reqLogger.Error(err, "Unable to change assembly state assembly")
		return "", err
	}

	reqLogger.Info(fmt.Sprintf("Change assembly state status %d", resp.StatusCode()))

	if resp.StatusCode() != http.StatusCreated {
		return "", fmt.Errorf("Change assembly state failed %s %s", resp.Body(), string(resp.StatusCode()))
	}

	location := resp.Header().Get(http.CanonicalHeaderKey("Location"))
	ss := strings.Split(location, "/")
	return ss[len(ss)-1], nil
}

func (i *Ishtar) DeleteAssembly(reqLogger logr.Logger, assembly DeleteAssemblyBody) (string, error) {
	accessToken, err := i.LMSecurityCtrl.getAccessToken()
	if err != nil {
		reqLogger.Error(err, "Unable to get access token")
		return "", err
	}

	bytes, err := json.Marshal(assembly)
	if err != nil {
		reqLogger.Error(err, "Unable to create assembly delete body")
		return "", err
	}
	assemblyJSON := string(bytes)

	reqLogger.Info(fmt.Sprintf("Delete assembly %s", assemblyJSON))
	reqLogger.Info(fmt.Sprintf("Access token %s", accessToken))

	resp, err := i.restClient.R().
		EnableTrace().
		SetBody(assemblyJSON).
		SetHeader("Content-Type", "application/json").
		SetHeader("Authorization", fmt.Sprintf("Bearer %s", accessToken)).
		Post("https://ishtar:8280/api/intent/deleteAssembly")
	if err != nil {
		reqLogger.Error(err, "Unable to delete assembly")
		return "", err
	}

	reqLogger.Info(fmt.Sprintf("Delete assembly status %d", resp.StatusCode()))

	if resp.StatusCode() != http.StatusCreated {
		return "", fmt.Errorf("Delete assembly failed %s %s", resp.Body(), string(resp.StatusCode()))
	}

	location := resp.Header().Get(http.CanonicalHeaderKey("Location"))
	ss := strings.Split(location, "/")
	return ss[len(ss)-1], nil
}

func (i *Ishtar) Health(reqLogger logr.Logger) (bool, error) {
	accessToken, err := i.LMSecurityCtrl.getAccessToken()
	if err != nil {
		reqLogger.Error(err, "Unable to get access token")
		return false, err
	}

	resp, err := i.restClient.R().
		EnableTrace().
		SetResult(&HealthStatus{}).
		SetHeader("Authorization", fmt.Sprintf("Bearer %s", accessToken)).
		Get("https://ishtar:8280/management/health")
	if err != nil {
		reqLogger.Error(err, "Unable to get Ishtar health")
		return false, err
	}

	healthStatus := (*resp.Result().(*HealthStatus))

	// statusBody, err := ioutil.ReadAll(resp.Body)
	// if err != nil {
	// 	reqLogger.Error(err, "Unable to get Ishtar health")
	// 	return false, err
	// }

	reqLogger.Info(fmt.Sprintf("Ishtar health status %s", healthStatus.Status))

	return healthStatus.Status == "UP", nil
}

func (i *Ishtar) GetAssembly(reqLogger logr.Logger, assemblyID string) (Assembly, error) {
	accessToken, err := i.LMSecurityCtrl.getAccessToken()
	if err != nil {
		reqLogger.Error(err, "Unable to get access token")
		return Assembly{}, err
	}

	resp, err := i.restClient.R().
		EnableTrace().
		SetResult(Assembly{}).
		SetHeader("Authorization", fmt.Sprintf("Bearer %s", accessToken)).
		Get(fmt.Sprintf("https://ishtar:8280/api/topology/assemblies/%s", assemblyID))
	if err != nil {
		return Assembly{}, err
	}

	if resp.StatusCode() == http.StatusNotFound {
		return Assembly{}, fmt.Errorf("Assembly not found (%s)", assemblyID)
	}
	if resp.StatusCode() != http.StatusOK {
		return Assembly{}, fmt.Errorf("Get assembly failed %s %s", resp.Body(), string(resp.StatusCode()))
	}

	result := (*resp.Result().(*Assembly))
	return result, nil
}

func (i *Ishtar) GetProcess(reqLogger logr.Logger, processID string) (Process, error) {
	accessToken, err := i.LMSecurityCtrl.getAccessToken()
	if err != nil {
		reqLogger.Error(err, "Unable to get access token")
		return Process{}, err
	}

	resp, err := i.restClient.R().
		EnableTrace().
		SetResult(Process{}).
		SetHeader("Authorization", fmt.Sprintf("Bearer %s", accessToken)).
		Get(fmt.Sprintf("https://ishtar:8280/api/processes/%s", processID))
	if err != nil {
		return Process{}, err
	}

	if resp.StatusCode() != http.StatusOK {
		return Process{}, fmt.Errorf("Get process failed %s %s", resp.Body(), string(resp.StatusCode()))
	}

	result := (*resp.Result().(*Process))
	return result, nil
}

type Process struct {
	ID string `json:"id"`
	AssemblyID string `json:"assemblyId"`
	Status string `json:"status"`
	StatusReason string `json:"statusReason"`
}

type Assembly struct {
	ID string `json:"id"`
	Name string `json:"name"`
	State string `json:"state"`
	DescriptorName string `json:"descriptorName"`
	Properties []AssemblyProperty `json:"properties"`
}

type AssemblyProperty struct {
	Name string `json:"name"`
	Value string `json:"value"`
}
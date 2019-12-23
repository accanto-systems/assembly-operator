package lm

import (
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"net/http"
	"time"

	resty "github.com/go-resty/resty/v2"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

const oauthApi = "/oauth/token"

var securityLog = logf.Log.WithName("lm_security")

type AuthResponse struct {
	AccessToken string `json:"access_token"`
	ExpiresIn   int32  `json:"expires_in"`
	Scope       string `json:"scope"`
}

type LMSecurityCtrl struct {
	restClient      *resty.Client
	lmConfiguration *LMConfiguration
	auth            *AuthResponse
	authTime        time.Time
}

func BuildCtrl(lmConfiguration *LMConfiguration) *LMSecurityCtrl {
	securityLog.Info("Building LM security ctrl", LogKeys.URL, lmConfiguration.Base, LogKeys.Client, lmConfiguration.Client)
	restClient := resty.New()
	restClient.SetTLSClientConfig(&tls.Config{InsecureSkipVerify: true})
	restClient.SetTimeout(2 * time.Minute)
	lmSecurityCtrl := LMSecurityCtrl{
		restClient:      restClient,
		lmConfiguration: lmConfiguration,
	}
	return &lmSecurityCtrl
}

func (ctrl *LMSecurityCtrl) getAccessToken() (string, error) {
	if !ctrl.lmConfiguration.Secure {
		return "", nil
	}
	if ctrl.needNewToken() {
		result, err := ctrl.requestAccessToken()
		if err != nil {
			return "", err
		}
		ctrl.auth = result
		ctrl.authTime = time.Now()
	}

	return ctrl.auth.AccessToken, nil
}

func (ctrl *LMSecurityCtrl) requestAccessToken() (*AuthResponse, error) {
	url := fmt.Sprintf("%s%s", ctrl.lmConfiguration.Base, oauthApi)
	securityLog.Info("Requesting new access token for client", LogKeys.URL, url, LogKeys.Client, ctrl.lmConfiguration.Client)

	credentialsAsBytes := []byte(fmt.Sprintf("%s:%s", ctrl.lmConfiguration.Client, ctrl.lmConfiguration.ClientSecret))
	encodedCredentials := base64.StdEncoding.EncodeToString(credentialsAsBytes)
	request := map[string]string{
		"grant_type": "client_credentials",
	}
	resp, err := ctrl.restClient.R().
		EnableTrace().
		SetFormData(request).
		SetResult(&AuthResponse{}).
		SetHeader("Content-Type", "application/x-www-form-urlencoded").
		SetHeader("Authorization", fmt.Sprintf("Basic %s", encodedCredentials)).
		Post(url)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode() == http.StatusOK {
		authResponse := (*resp.Result().(*AuthResponse))
		return &authResponse, nil
	} else {
		return nil, &LMClientError{
			ResponseBody: string(resp.Body()),
			StatusCode:   resp.StatusCode(),
		}
	}
}

func (ctrl *LMSecurityCtrl) needNewToken() bool {
	if ctrl.auth == nil {
		securityLog.Info("No existing access token, must request one")
		return true
	}

	expirationSeconds := ctrl.auth.ExpiresIn
	authenticatedSeconds := time.Now().Sub(ctrl.authTime).Seconds()
	securityLog.Info("Checking if existing access token has expired", LogKeys.AuthTime, ctrl.authTime, LogKeys.ExpirationSecs, ctrl.auth.ExpiresIn, LogKeys.AuthenticatedSecs, authenticatedSeconds)

	if int(authenticatedSeconds) >= int(expirationSeconds) {
		securityLog.Info("Token expired, must request a new one", LogKeys.AuthTime, ctrl.authTime, LogKeys.ExpirationSecs, ctrl.auth.ExpiresIn, LogKeys.AuthenticatedSecs, authenticatedSeconds)
		return true
	}
	// If the token expires within 1 second, wait and get a new one
	if int32(authenticatedSeconds) >= (expirationSeconds - 1) {
		securityLog.Info("Token expires in less than 1 second, waiting before requesting a new Token", LogKeys.AuthTime, ctrl.authTime, LogKeys.ExpirationSecs, ctrl.auth.ExpiresIn, LogKeys.AuthenticatedSecs, authenticatedSeconds)
		time.Sleep(2 * time.Second)
		return true
	}

	return false
}

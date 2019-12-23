package lm

type logKeys struct {
	URL                string
	AuthTime           string
	ExpirationSecs     string
	AuthenticatedSecs  string
	Client             string
	Secure             string
	Body               string
	ResponseStatusCode string
	AssemblyID         string
	ProcessID          string
	AssemblyName       string
}

var LogKeys = &logKeys{
	URL:                "url",
	AuthTime:           "authTime",
	ExpirationSecs:     "expirationSecs",
	AuthenticatedSecs:  "authenticatedSecs",
	Client:             "client",
	Secure:             "secure",
	Body:               "body",
	ResponseStatusCode: "responseStatus",
	AssemblyID:         "assemblyId",
	ProcessID:          "processId",
	AssemblyName:       "assemblyName",
}

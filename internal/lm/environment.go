package lm

import (
	"io/ioutil"

	"gopkg.in/yaml.v2"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

var environmentLog = logf.Log.WithName("lm_environment")

type LMConfiguration struct {
	Client       string `yaml:"client"`
	ClientSecret string `yaml:"clientSecret"`
	Base         string `yaml:"base"`
	Secure       bool   `yaml:"secure"`
}

func ReadLMConfiguration() (*LMConfiguration, error) {
	yamlFile, err := ioutil.ReadFile("/var/assembly-operator/config.yaml")
	if err != nil {
		environmentLog.Error(err, "Failed to read config file")
		return &LMConfiguration{}, err
	}
	configuration := LMConfiguration{}
	err = yaml.Unmarshal(yamlFile, &configuration)
	if err != nil {
		environmentLog.Error(err, "Unmarshal error")
		return &configuration, err
	}

	return &configuration, nil
}

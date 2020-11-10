package conf

import (
	"fmt"
	"io/ioutil"
	"os"

	yaml "gopkg.in/yaml.v2"
)

// ConfigurationD Definition
type ConfigurationD struct {
	Tool     string     `yaml:"tool"`
	Registry string     `yaml:"registry"`
	Projects []ProjectD `yaml:"projects"`
	Runtime  RuntimeD   `yaml:"runtime"`
}

// ProjectD Definition
type ProjectD struct {
	Name       string      `yaml:"name"`
	Repository RepositoryD `yaml:"repository"`
}

// RepositoryD Definition
type RepositoryD struct {
	FetchAll bool     `yaml:"fetchAll"`
	Items    []string `yaml:"items"`
}

// RuntimeD Definition
type RuntimeD struct {
	Pool       int         `yaml:"pool"`
	ExportFile ExportFileD `yaml:"exportFile"`
}

// ExportFileD Definition
type ExportFileD struct {
	MaxImageCount int `yaml:"maxImageCount"`
}

// GetConfiguration Function
func GetConfiguration(configFile string) *ConfigurationD {
	c := &ConfigurationD{}

	yamlFile, err := ioutil.ReadFile(configFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "yamlFile.Get err %v", err)
		os.Exit(1)
	}

	err = yaml.Unmarshal(yamlFile, c)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Unmarshal: %v", err)
		os.Exit(1)
	}

	return c
}

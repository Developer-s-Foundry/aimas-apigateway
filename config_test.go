package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"go.yaml.in/yaml/v3"
)

var configFile = "test_config.yaml"
var configPath string

type ConfigTestSuite struct {
	suite.Suite
}

func (c *ConfigTestSuite) SetupTest() {
	var err error
	var file *os.File
	file, err = os.CreateTemp("", configFile)
	require.NoError(c.T(), err)
	configFile = strings.Split(file.Name(), "/")[2]
	configPath = filepath.Join("/", strings.Split(file.Name(), "/")[1])
}

func (c *ConfigTestSuite) TearDownTest() {
	path := filepath.Join(configPath, configFile)
	err := os.Remove(path)
	require.NoError(c.T(), err)
}

var scList ServiceConfig

func (c *ConfigTestSuite) TestConfigFileLoaded() {
	writeToTestConfigFile(c.T())

	sc, err := NewServiceConfig(configFile, configPath)
	require.NoError(c.T(), err)
	require.Equal(c.T(), len(sc.Services), len(scList.Services))

	require.Equal(c.T(), sc.Services[0].Name, scList.Services[0].Name)
	require.Equal(c.T(), sc.Services[0].Description, scList.Services[0].Description)
	require.Equal(c.T(), sc.Services[0].Host, scList.Services[0].Host)
	require.Equal(c.T(), sc.Services[0].Prefix, scList.Services[0].Prefix)
}

func writeToTestConfigFile(t *testing.T) {
	sc := serviceConfig()

	joinPath := filepath.Join(configPath, configFile)
	fmt.Println(joinPath)
	file, err := os.OpenFile(joinPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY|os.O_RDONLY, 0655)
	require.NoError(t, err)
	if err := yaml.NewEncoder(file).Encode(sc); err != nil {
		require.NoError(t, err)
	}
}

func serviceConfig() ServiceConfig {
	sc := Service{
		Name:        "user-service",
		Host:        "localhost",
		Port:        9090,
		Prefix:      "api/v1",
		Protocol:    "http",
		Description: "a test config setup",
		Version:     "v1",
		Routes: []Route{
			Route{Path: "/users", Methods: []string{"GET", "POST"}},
		},
	}

	scList = ServiceConfig{
		Services: []Service{sc},
	}

	return scList
}

func TestConfigTestSuite(t *testing.T) {
	suite.Run(t, new(ConfigTestSuite))
}

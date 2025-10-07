package main

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"strconv"

	"github.com/spf13/viper"
	"go.yaml.in/yaml/v3"
)

type ServiceConfig struct {
	Services []Service `yaml:"services"`
}

type Service struct {
	Name        string  `yaml:"name"`
	Host        string  `yaml:"host"`
	Version     string  `yaml:"version"`
	Prefix      string  `yaml:"prefix"`
	Protocol    string  `yaml:"protocol"`
	Description string  `yaml:"description"`
	Port        int     `yaml:"port"`
	Routes      []Route `yaml:"routes"`
}

type Route struct {
	Path    string   `yaml:"path"`
	Methods []string `yaml:"methods"`
}

// config file path if not in current directory the next value should be the file path
// e.g configName:appConfig
// configFilePath: /etc/config if in current directory can be ommitted else file path follow suite like this
// NewServiceConfig(appConfig, /etc/config)
func NewServiceConfig(configName, configPath string) (ServiceConfig, error) {
	err := loadConfigFile(configName, configPath)

	var sc = ServiceConfig{}
	if err != nil {
		if errors.Is(err, viper.ConfigFileNotFoundError{}) {
			return ServiceConfig{}, ErrorConfigFileNotFound
		}
		return ServiceConfig{}, err
	}

	filepath := viper.ConfigFileUsed()
	data, err := os.Open(filepath)

	if err != nil {
		return ServiceConfig{}, err
	}

	if err := yaml.NewDecoder(data).Decode(&sc); err != nil {
		return ServiceConfig{}, err
	}
	return sc, nil
}

func loadConfigFile(configName, configPath string) error {
	if configPath == "" {
		configPath = "."
	}
	viper.SetConfigName(configName)
	viper.SetConfigType("yml")
	viper.AddConfigPath(configPath)
	return viper.ReadInConfig()
}

func (s *Service) parseURL() error {
	if !hasScheme(s.Host) {
		s.Host = fmt.Sprintf("%s://%s", s.Protocol, s.Host)
	}

	u, err := url.Parse(s.Host)
	if err != nil {
		return err
	}

	host := u.Host
	if host == "" {
		host = u.Path
	}
	hostPart, port, err := net.SplitHostPort(host)
	if err != nil {
		if _, ok := err.(*net.AddrError); ok && s.Port == 0 {
			return ErrorConfigMissingPort
		} else if s.Port != 0 {
			port = strconv.Itoa(s.Port)
		} else {
			return fmt.Errorf("invalid host or port: %v", err)
		}
	}

	s.Host = fmt.Sprintf("%s://%s", s.Protocol, net.JoinHostPort(hostPart, port))
	return nil
}

func hasScheme(s string) bool {
	return len(s) > 7 && (s[:7] == "http://" || s[:8] == "https://")
}

func (s Service) clientIp(addr string) (string, error) {
	ip, _, err := net.SplitHostPort(addr)
	if err != nil {
		return "", err
	}
	return ip, nil
}

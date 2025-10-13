package main

import (
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"

	"github.com/spf13/viper"
	"go.yaml.in/yaml/v3"
)

type ServiceConfig struct {
	Services []Service `yaml:"services"`
}

type RateLimit struct {
	RequestsPerMinute int `yaml:"requests_per_minute"`
}
type Service struct {
	Name        string    `yaml:"name"`
	Host        string    `yaml:"host"`
	Version     string    `yaml:"version"`
	Prefix      string    `yaml:"prefix"`
	Protocol    string    `yaml:"protocol"`
	Description string    `yaml:"description"`
	Port        int       `yaml:"port"`
	Routes      []Route   `yaml:"routes"`
	RateLimit   RateLimit `yaml:"rate_limit"`
}

type Route struct {
	Path    string   `yaml:"path"`
	Methods []string `yaml:"methods"`
}

func NewServiceConfig(configName, configPath, remoteAddr string) (ServiceConfig, error) {
	var err error
	data := make([]byte, 1024*2)
	var sc = ServiceConfig{}
	if configName != "" {
		err = loadConfigFile(configName, configPath)
		if err != nil {
			if errors.Is(err, viper.ConfigFileNotFoundError{}) {
				return ServiceConfig{}, ErrorConfigFileNotFound
			}
			return ServiceConfig{}, err
		}

		filepath := viper.ConfigFileUsed()
		file, err := os.Open(filepath)
		if err != nil {
			return ServiceConfig{}, err
		}
		n, err := file.Read(data)

		data = data[:n]
		if err != nil {
			return ServiceConfig{}, err
		}
	} else {
		resp, err := http.Get(remoteAddr)
		if err != nil {
			return ServiceConfig{}, err
		}
		data, err = io.ReadAll(resp.Body)
		if err != nil {
			return ServiceConfig{}, err
		}
	}

	if err := yaml.Unmarshal(data, &sc); err != nil {
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
	if s.Protocol == "" {
		s.Protocol = "http"
	}
	if !hasScheme(s.Host) {
		s.Host = fmt.Sprintf("%s://%s", s.Protocol, s.Host)
	}

	u, err := url.Parse(s.Host)
	if err != nil {
		return fmt.Errorf("invalid URL: missing scheme or host, %w", err)
	}

	if strings.Contains(u.Host, ":") && strings.Count(u.Host, ":") > 1 {
		return errors.New("invalid URL: malformed port section")
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

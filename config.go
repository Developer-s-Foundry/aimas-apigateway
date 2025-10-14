package main

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/stretchr/testify/assert/yaml"
)

type ServiceConfigFile struct {
	Services []Service `yaml:"services"`
}

type RateLimit struct {
	RequestsPerMinute int `yaml:"requests_per_minute"`
}

type Service struct {
	Name      string    `yaml:"name"`
	Host      string    `yaml:"host"`
	Prefix    string    `yaml:"prefix"`
	RateLimit RateLimit `yaml:"rate_limit"`
	URL       *url.URL  `yaml:"-"`
}

func loadConfigFile(path string) (map[string]*Service, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var scf ServiceConfigFile
	if err := yaml.Unmarshal(data, &scf); err != nil {
		return nil, err
	}

	out := make(map[string]*Service)
	for _, svc := range scf.Services {
		if svc.Prefix == "" {
			svc.Prefix = "/" + strings.TrimPrefix(svc.Name, "/")
		}
		p := "/" + strings.Trim(strings.TrimSpace(svc.Prefix), "/")
		svc.Prefix = p

		u, err := url.Parse(svc.Host)
		if err != nil || u.Scheme == "" || u.Host == "" {
			return nil, fmt.Errorf("invalid host for service %s: %s", svc.Name, svc.Host)
		}
		svc.URL = u

		if _, ok := out[p]; ok {
			return nil, fmt.Errorf("duplicate service prefix: %s", p)
		}
		s := svc
		out[p] = &s
	}

	return out, nil
}

func (g *Gateway) WatchConfig(path string, stopCtx context.Context) error {
	abs, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	dir := filepath.Dir(abs)
	file := filepath.Base(abs)

	w, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}

	if err := w.Add(dir); err != nil {
		_ = w.Close()
		return err
	}

	if err := g.reloadFromPath(abs); err != nil {
		g.logger.Warning("err", fmt.Sprintf("initial config load failed: %v", err))
	}

	go func() {
		defer w.Close()
		debounce := time.NewTimer(0)
		if !debounce.Stop() {
			<-debounce.C
		}
		for {
			select {
			case <-stopCtx.Done():
				return
			case ev, ok := <-w.Events:
				if !ok {
					return
				}
				if filepath.Base(ev.Name) != file {
					continue
				}
				debounce.Reset(200 * time.Millisecond)
			case <-debounce.C:
				if err := g.reloadFromPath(abs); err != nil {
					g.logger.Warning("err", fmt.Sprintf("reload failed: %v", err))
				}
			case err := <-w.Errors:
				g.logger.Warning("err", fmt.Sprintf("fsnotify error: %v", err))
			}
		}
	}()
	return nil
}

func (g *Gateway) reloadFromPath(path string) error {
	newRoutes, err := loadConfigFile(path)
	if err != nil {
		return err
	}
	g.atomicRoutes.Store(newRoutes)

	g.cleanupProxyCache(newRoutes)

	g.logger.Info("reload", fmt.Sprintf("configuration reloeaded: %d services", len(newRoutes)))
	return nil
}

func (g *Gateway) cleanupProxyCache(keep map[string]*Service) {
	g.proxyCache.Range(func(k, v any) bool {
		key := k.(string)
		keepThis := false
		for _, svc := range keep {
			if svc.Name == key {
				keepThis = true
				break
			}
		}
		if !keepThis {
			g.proxyCache.Delete(key)
			g.logger.Error("poxy-cache-error", fmt.Sprintf("proxy cache evicted for service: %s", key), nil)
		}
		return true
	})
}

func extractPrefix(p string) string {
	p = strings.TrimSpace(p)
	if p == "" || p == "/" {
		return ""
	}
	p = "/" + strings.Trim(p, "/")
	parts := strings.SplitN(strings.TrimPrefix(p, "/"), "/", 2)
	return "/" + parts[0]
}

/*
SPDX-FileCopyrightText: 2021 SAP SE or an SAP affiliate company and Gardener contributors

SPDX-License-Identifier: Apache-2.0
*/

package util

import (
	"context"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/gardener/gardenctl-v2/pkg/config"
	"github.com/gardener/gardenctl-v2/pkg/target"
)

// Factory provides abstractions that allow the command to be extended across multiple types of resources and different API sets.
type Factory interface {
	// Context returns the root context any command should use.
	Context() context.Context
	// Clock returns a clock that provides access to the current time.
	Clock() Clock
	// GardenHomeDir returns the gardenctl home directory for the executing user.
	GardenHomeDir() string
	// Manager returns the target manager used to read and change the currently targeted system.
	Manager() (target.Manager, error)
	// PublicIPs returns the current host's public IP addresses. It's
	// recommended to provide a context with a timeout/deadline. The
	// returned slice can contain IPv6, IPv4 or both, in no particular
	// order.
	PublicIPs(context.Context) ([]string, error)
}

// FactoryImpl implements util.Factory interface
type FactoryImpl struct {
	// GardenHomeDirectory is the home directory for all gardenctl
	// related files. While some files can be explicitly loaded from
	// different locations, cache files will always be placed inside
	// the garden home.
	GardenHomeDirectory string

	// ConfigFile is the location of the gardenctlv2 configuration file.
	// This can be overriden via a CLI flag and defaults to ~/.garden/gardenctlv2.yaml
	// if empty.
	ConfigFile string

	// TargetFile is the filename where the currently active target is located.
	// This is only used if the TargetProvider property is nil.
	TargetFile string

	// TargetProvider can be used to completely override the provider. In this
	// case, TargetFile is not used by the factory.
	TargetProvider target.TargetProvider
}

var _ Factory = &FactoryImpl{}

func (f *FactoryImpl) Context() context.Context {
	return context.Background()
}

func (f *FactoryImpl) Manager() (target.Manager, error) {
	cfg, err := config.LoadFromFile(f.ConfigFile)
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	targetProvider := f.TargetProvider
	if targetProvider == nil {
		targetProvider = target.NewFilesystemTargetProvider(f.TargetFile)
	}

	kubeconfigCache := target.NewFilesystemKubeconfigCache(filepath.Join(f.GardenHomeDirectory, "cache", "kubeconfigs"))
	clientProvider := target.NewClientProvider()

	return target.NewManager(cfg, targetProvider, clientProvider, kubeconfigCache)
}

func (f *FactoryImpl) GardenHomeDir() string {
	return f.GardenHomeDirectory
}

func (f *FactoryImpl) Clock() Clock {
	return &RealClock{}
}

func (f *FactoryImpl) PublicIPs(ctx context.Context) ([]string, error) {
	ipv64, err := callIPify(ctx, "api64.ipify.org")
	if err != nil {
		return nil, err
	}

	addresses := []string{ipv64.String()}

	// if the above resolved to IPv6, we also _try_ the IPv4-only;
	// this is optional and failures are silently swallowed
	if ipv64.To4() == nil {
		if ipv4, err := callIPify(ctx, "api.ipify.org"); err == nil {
			addresses = append(addresses, ipv4.String())
		}
	}

	return addresses, nil
}

func callIPify(ctx context.Context, domain string) (*net.IP, error) {
	req, err := http.NewRequest("GET", fmt.Sprintf("https://%s/", domain), nil)
	if err != nil {
		return nil, err
	}

	req = req.WithContext(ctx)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	ip, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	ipAddress := strings.TrimSpace(string(ip))
	netIP := net.ParseIP(ipAddress)

	if netIP == nil {
		return nil, fmt.Errorf("API returned an invalid IP (%q)", ipAddress)
	}

	return &netIP, nil
}

// WithFilesystemTargetProvider returns a copy of the factory with a fsTargetProvider
func (f *FactoryImpl) WithFilesystemTargetProvider() Factory {
	return &FactoryImpl{
		GardenHomeDirectory: f.GardenHomeDirectory,
		ConfigFile:          f.ConfigFile,
		TargetFile:          f.TargetFile,
		TargetProvider:      target.NewFilesystemTargetProvider(f.TargetFile),
	}
}

/*
SPDX-FileCopyrightText: 2021 SAP SE or an SAP affiliate company and Gardener contributors

SPDX-License-Identifier: Apache-2.0
*/

package util

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	internalclient "github.com/gardener/gardenctl-v2/internal/client"
	"github.com/gardener/gardenctl-v2/pkg/config"
	"github.com/gardener/gardenctl-v2/pkg/target"
)

const (
	envSessionID     = "GCTL_SESSION_ID"
	envTermSessionID = "TERM_SESSION_ID"
)

var (
	sidRegexp  = regexp.MustCompile(`^[\w-]{1,128}$`)
	uuidRegexp = regexp.MustCompile(`([a-f0-9]{8}-[a-f0-9]{4}-4[a-f0-9]{3}-[89ab][a-f0-9]{3}-[a-f0-9]{12})`)
)

//go:generate mockgen -destination=./mocks/mock_factory.go -package=mocks github.com/gardener/gardenctl-v2/internal/util Factory

// Factory provides abstractions that allow the command to be extended across multiple types of resources and different API sets.
type Factory interface {
	// Context returns the root context any command should use.
	Context() context.Context
	// Clock returns a clock that provides access to the current time.
	Clock() Clock
	// GardenHomeDir returns the user's gardenctl configuration directory (e.g., ~/.garden), where persistent configuration files like gardenctl-v2.yaml are stored.
	GardenHomeDir() string
	// GardenTempDir returns the base directory for temporary data (e.g., /tmp/garden), including session directories and SSH known hosts files.
	GardenTempDir() string
	// GetSessionID returns the session ID for the current shell session.
	// The session ID is derived from the GCTL_SESSION_ID environment variable,
	// or extracted from TERM_SESSION_ID as a fallback.
	GetSessionID() (string, error)
	// Manager returns the target manager used to read and change the currently targeted system.
	Manager() (target.Manager, error)
	// PublicIPs returns the current host's public IP addresses. It's
	// recommended to provide a context with a timeout/deadline. The
	// returned slice can contain IPv6, IPv4 or both, in no particular
	// order.
	PublicIPs(context.Context) ([]string, error)
	// TargetFlags returns the TargetFlags to which the cobra flags are bound allowing the user to
	// override the target configuration stored on the filesystem.
	TargetFlags() target.TargetFlags
}

// FactoryImpl implements util.Factory interface.
type FactoryImpl struct {
	// GardenHomeDirectory is the home directory for all gardenctl
	// related files. While some files can be explicitly loaded from
	// different locations, persistent cache files will always be placed
	// inside the garden home.
	GardenHomeDirectory string

	// GardenTempDirectory is the base directory for temporary data.
	GardenTempDirectory string

	// ConfigFile is the location of the gardenctlv2 configuration file.
	// This can be overridden via a CLI flag and defaults to ~/.garden/gardenctlv2.yaml
	// if empty.
	ConfigFile string

	// targetFlags can be used to completely override the target configuration
	// stored on the filesystem via a CLI flags.
	targetFlags target.TargetFlags
}

var _ Factory = &FactoryImpl{}

func NewFactoryImpl() *FactoryImpl {
	return &FactoryImpl{
		targetFlags:         target.NewTargetFlags("", "", "", "", false),
		GardenTempDirectory: filepath.Join(os.TempDir(), "garden"),
	}
}

func (f *FactoryImpl) Context() context.Context {
	return context.Background()
}

func (f *FactoryImpl) Manager() (target.Manager, error) {
	cfg, err := config.LoadFromFile(f.ConfigFile)
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	sid, err := f.GetSessionID()
	if err != nil {
		return nil, err
	}

	sessionsDirectory := filepath.Join(f.GardenTempDir(), "sessions")

	err = os.MkdirAll(sessionsDirectory, 0o700)
	if err != nil {
		return nil, fmt.Errorf("failed to create sessions directory: %w", err)
	}

	sessionDirectory := filepath.Join(sessionsDirectory, sid)
	// Do not create sessionDirectory (if it does not exist) before the migration.
	// Creating it prematurely will cause the migration (moving the folder) to fail.

	// Migration logic
	oldSessionDirectory := filepath.Join(os.TempDir(), "garden", sid)
	if _, err := os.Stat(oldSessionDirectory); err == nil {
		err = os.Rename(oldSessionDirectory, sessionDirectory)
		if err != nil {
			return nil, fmt.Errorf("failed to migrate session directory: %w", err)
		}
	}

	err = os.MkdirAll(sessionDirectory, 0o700)
	if err != nil {
		return nil, fmt.Errorf("failed to create session directory: %w", err)
	}

	targetProvider := target.NewTargetProvider(filepath.Join(sessionDirectory, "target.yaml"), f.targetFlags)
	clientProvider := internalclient.NewProvider()

	return target.NewManager(cfg, targetProvider, clientProvider, sessionDirectory)
}

func (f *FactoryImpl) GardenHomeDir() string {
	return f.GardenHomeDirectory
}

func (f *FactoryImpl) GardenTempDir() string {
	return f.GardenTempDirectory
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

func (f *FactoryImpl) TargetFlags() target.TargetFlags {
	return f.targetFlags
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

	ip, err := io.ReadAll(resp.Body)
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

// GetSessionID returns the session ID from environment variables.
// It first checks GCTL_SESSION_ID, then falls back to extracting from TERM_SESSION_ID.
// The session ID must be alphanumeric with underscores and dashes, 1-128 characters long.
func (f *FactoryImpl) GetSessionID() (string, error) {
	if value, ok := os.LookupEnv(envSessionID); ok {
		if sidRegexp.MatchString(value) {
			return value, nil
		}

		return "", fmt.Errorf("environment variable %s must only contain alphanumeric characters, underscore and dash and have a minimum length of 1 and a maximum length of 128", envSessionID)
	}

	if value, ok := os.LookupEnv(envTermSessionID); ok {
		match := uuidRegexp.FindStringSubmatch(strings.ToLower(value))
		if len(match) > 1 {
			return match[1], nil
		}
	}

	return "", fmt.Errorf("environment variable %s is required. Use \"gardenctl help\" for more information about the requirements of gardenctl", envSessionID)
}

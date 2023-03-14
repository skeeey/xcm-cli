package configs

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/golang/glog"
	sdk "github.com/openshift-online/ocm-sdk-go"

	"github.com/skeeey/xcm-cli/pkg/genericflags"
	"github.com/skeeey/xcm-cli/pkg/info"
)

type APIConfig struct {
	AccessToken  string   `json:"access_token,omitempty" doc:"Bearer access token."`
	RefreshToken string   `json:"refresh_token,omitempty" doc:"Offline or refresh token."`
	Scopes       []string `json:"scopes,omitempty" doc:"OpenID scope. If this option is used it will replace completely the default scopes. Can be repeated multiple times to specify multiple scopes."`
	TokenURL     string   `json:"token_url,omitempty" doc:"OpenID token URL."`
	URL          string   `json:"url,omitempty" doc:"URL of the API gateway. The value can be the complete URL or an alias. The valid aliases are 'production', 'staging' and 'integration'."`
	Insecure     bool     `json:"insecure,omitempty" doc:"Enables insecure communication with the server. This disables verification of TLS certificates and host names."`
}

// Save saves the given configuration to the configuration file.
func (c *APIConfig) Save() error {
	dir, err := ConfigDir()
	if err != nil {
		return err
	}
	file := filepath.Join(dir, "xcm.json")
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return fmt.Errorf("canot marshal config: %v", err)
	}
	err = os.WriteFile(file, data, 0600)
	if err != nil {
		return fmt.Errorf("canot write file '%s': %v", file, err)
	}
	return nil
}

// Armed checks if the configuration contains either credentials or tokens that haven't expired, so
// that it can be used to perform authenticated requests.
func (c *APIConfig) Armed() (armed bool, reason string, err error) {
	// Check URLs:
	haveURL := c.URL != ""
	haveTokenURL := c.TokenURL != ""
	haveURLs := haveURL && haveTokenURL

	// Check tokens:
	haveAccess := c.AccessToken != ""
	accessUsable := false
	if haveAccess {
		accessUsable, err = tokenUsable(c.AccessToken, 5*time.Second)
		if err != nil {
			return
		}
	}
	haveRefresh := c.RefreshToken != ""
	refreshUsable := false
	if haveRefresh {
		if IsEncryptedToken(c.RefreshToken) {
			// We have no way of knowing an encrypted token expiration, so
			// we assume it's valid and let the access token request fail.
			refreshUsable = true
		} else {
			refreshUsable, err = tokenUsable(c.RefreshToken, 10*time.Second)
			if err != nil {
				return
			}
		}
	}

	// Calculate the result:
	armed = haveURLs && (accessUsable || refreshUsable)
	if armed {
		return
	}

	// If it isn't armed then we should return a human readable reason. We should try to
	// generate a message that describes the more relevant reason. For example, missing
	// credentials is more important than missing URLs, so that condition should be checked
	// first.
	switch {
	case haveAccess && !haveRefresh && !accessUsable:
		reason = "access token is expired"
	case !haveAccess && haveRefresh && !refreshUsable:
		reason = "refresh token is expired"
	case haveAccess && !accessUsable && haveRefresh && !refreshUsable:
		reason = "access and refresh tokens are expired"
	case !haveURL && haveTokenURL:
		reason = "server URL isn't set"
	case haveURL && !haveTokenURL:
		reason = "token URL isn't set"
	case !haveURL && !haveTokenURL:
		reason = "server and token URLs aren't set"
	}

	return
}

// Disarm removes from the configuration all the settings that are needed for authentication.
func (c *APIConfig) Disarm() {
	c.AccessToken = ""
	c.RefreshToken = ""
	c.Scopes = nil
	c.TokenURL = ""
	c.URL = ""
}

// Connection creates a connection using this configuration.
func (c *APIConfig) Connection() (connection *sdk.Connection, err error) {
	// Create the logger:
	level := glog.Level(1)

	if genericflags.DebugEnabled() {
		level = glog.Level(0)
	}
	logger, err := sdk.NewGlogLoggerBuilder().
		DebugV(level).
		InfoV(level).
		WarnV(level).
		Build()
	if err != nil {
		return
	}

	// Prepare the builder for the connection adding only the properties that have explicit
	// values in the configuration, so that default values won't be overridden:
	builder := sdk.NewConnectionBuilder()
	builder.Logger(logger)
	// TODO change agent
	builder.Agent("OCM-CLI/" + info.Version)
	if c.TokenURL != "" {
		builder.TokenURL(c.TokenURL)
	}
	if c.Scopes != nil {
		builder.Scopes(c.Scopes...)
	}
	if c.URL != "" {
		builder.URL(c.URL)
	}
	tokens := make([]string, 0, 2)
	if c.AccessToken != "" {
		tokens = append(tokens, c.AccessToken)
	}
	if c.RefreshToken != "" {
		tokens = append(tokens, c.RefreshToken)
	}
	if len(tokens) > 0 {
		builder.Tokens(tokens...)
	}
	builder.Insecure(c.Insecure)

	// Create the connection:
	connection, err = builder.Build()
	if err != nil {
		return
	}

	return
}

// ConfigDir returns the location of the configuration directory.
func ConfigDir() (dir string, err error) {
	// Determine standard config directory
	configDir, err := os.UserConfigDir()
	if err != nil {
		return dir, err
	}

	// Use standard config directory
	dir = filepath.Join(configDir, "/xcm")
	err = os.MkdirAll(dir, os.FileMode(0755))
	if err != nil {
		return dir, fmt.Errorf("canot create directory %s: %v", dir, err)
	}
	return dir, nil
}

// LoadAPIConfig loads the configuration from the configuration file. If the configuration file doesn't exist
// it will return an empty configuration object.
func LoadAPIConfig() (*APIConfig, error) {
	dir, err := ConfigDir()
	if err != nil {
		return nil, err
	}

	file := filepath.Join(dir, "xcm.json")

	_, err = os.Stat(file)
	if os.IsNotExist(err) {
		return &APIConfig{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("canot check if config file '%s' exists: %v", file, err)
	}

	data, err := os.ReadFile(file)
	if err != nil {
		return nil, fmt.Errorf("canot read config file '%s': %v", file, err)
	}

	if len(data) == 0 {
		return &APIConfig{}, nil
	}

	cfg := &APIConfig{}
	if err := json.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("can't parse config file '%s': %v", file, err)
	}

	return cfg, nil
}

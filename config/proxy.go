package config

import "github.com/spf13/pflag"

const (
	DefaultAcceptorMin  = 4000
	DefaultAcceptorMax  = 7000
	DefaultNumAcceptors = 5
)

// ProxyConfig provides proxy configuration settings and flags.
type ProxyConfig struct {
	flagBase

	BindHostPort string
	ProxyTarget  string

	AcceptorMin  int
	AcceptorMax  int
	NumAcceptors int
}

// NewProxyConfig returns a new proxy configuration.
func NewProxyConfig() *ProxyConfig {
	return &ProxyConfig{}
}

// FlagSet returns an instance of the flag set for the configuration.
func (c *ProxyConfig) FlagSet() *pflag.FlagSet {
	if c.initFlagSet() {
		c.flagSet.StringVar(&c.BindHostPort, "bind-hostport", "0.0.0.0:5000", "Host port to bind on")
		c.flagSet.StringVar(&c.ProxyTarget, "proxy-target", "", "Target address to proxy to")
	}
	return c.flagSet
}

func (c *ProxyConfig) WithDefaults() *ProxyConfig {
	if c.AcceptorMax == 0 {
		c.AcceptorMax = DefaultAcceptorMax
	}
	if c.AcceptorMin == 0 {
		c.AcceptorMin = DefaultAcceptorMin
	}
	if c.NumAcceptors == 0 {
		c.NumAcceptors = DefaultNumAcceptors
	}
	return c
}

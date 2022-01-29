package controllers

import (
	"time"
)

// Configuration is a cloudflared configuration yaml model
type Configuration struct {
	TunnelId      string                   `yaml:"tunnel"`
	Ingress       []UnvalidatedIngressRule `yaml:"ingress,omitempty"`
	WarpRouting   WarpRoutingConfig        `yaml:"warp-routing,omitempty"`
	OriginRequest OriginRequestConfig      `yaml:"originRequest,omitempty"`
	SourceFile    string                   `yaml:"credentials-file"`
	Metrics       string                   `yaml:"metrics,omitempty"`
	NoAutoUpdate  bool                     `yaml:"no-autoupdate,omitempty"`
}

// UnvalidatedIngressRule is a cloudflared ingress entry model
type UnvalidatedIngressRule struct {
	Hostname      string `yaml:"hostname,omitempty"`
	Path          string `yaml:"path,omitempty"`
	Service       string
	OriginRequest OriginRequestConfig `yaml:"originRequest,omitempty"`
}

// WarpRoutingConfig is a cloudflared warp routing model
type WarpRoutingConfig struct {
	Enabled bool `yaml:"enabled,omitempty"`
}

// OriginRequestConfig is a cloudflared origin request configuration model
type OriginRequestConfig struct {
	// HTTP proxy timeout for establishing a new connection
	ConnectTimeout *time.Duration `yaml:"connectTimeout,omitempty"`
	// HTTP proxy timeout for completing a TLS handshake
	TLSTimeout *time.Duration `yaml:"tlsTimeout,omitempty"`
	// HTTP proxy TCP keepalive duration
	TCPKeepAlive *time.Duration `yaml:"tcpKeepAlive,omitempty"`
	// HTTP proxy should disable "happy eyeballs" for IPv4/v6 fallback
	NoHappyEyeballs *bool `yaml:"noHappyEyeballs,omitempty"`
	// HTTP proxy maximum keepalive connection pool size
	KeepAliveConnections *int `yaml:"keepAliveConnections,omitempty"`
	// HTTP proxy timeout for closing an idle connection
	KeepAliveTimeout *time.Duration `yaml:"keepAliveTimeout,omitempty"`
	// Sets the HTTP Host header for the local webserver.
	HTTPHostHeader *string `yaml:"httpHostHeader,omitempty"`
	// Hostname on the origin server certificate.
	OriginServerName *string `yaml:"originServerName,omitempty"`
	// Path to the CA for the certificate of your origin.
	// This option should be used only if your certificate is not signed by Cloudflare.
	CAPool *string `yaml:"caPool,omitempty"`
	// Disables TLS verification of the certificate presented by your origin.
	// Will allow any certificate from the origin to be accepted.
	// Note: The connection from your machine to Cloudflare's Edge is still encrypted.
	NoTLSVerify *bool `yaml:"noTLSVerify,omitempty"`
	// Disables chunked transfer encoding.
	// Useful if you are running a WSGI server.
	DisableChunkedEncoding *bool `yaml:"disableChunkedEncoding,omitempty"`
	// Runs as jump host
	BastionMode *bool `yaml:"bastionMode,omitempty"`
	// Listen address for the proxy.
	ProxyAddress *string `yaml:"proxyAddress,omitempty"`
	// Listen port for the proxy.
	ProxyPort *uint `yaml:"proxyPort,omitempty"`
	// Valid options are 'socks' or empty.
	ProxyType *string `yaml:"proxyType,omitempty"`
	// IP rules for the proxy service
	IPRules []IngressIPRule `yaml:"ipRules,omitempty"`
}

// IngressIPRule is a cloudflared origin ingress IP rule config model
type IngressIPRule struct {
	Prefix *string `yaml:"prefix,omitempty"`
	Ports  []int   `yaml:"ports,omitempty"`
	Allow  bool    `yaml:"allow,omitempty"`
}

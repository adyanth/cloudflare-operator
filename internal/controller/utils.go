package controller

import (
	"context"
	"fmt"

	"github.com/cloudflare/cloudflare-go"
	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	apitypes "k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	networkingv1alpha2 "github.com/adyanth/cloudflare-operator/api/v1alpha2"
)

const (
	tunnelProtoHTTP  = "http"
	tunnelProtoHTTPS = "https"
	tunnelProtoRDP   = "rdp"
	tunnelProtoSMB   = "smb"
	tunnelProtoSSH   = "ssh"
	tunnelProtoTCP   = "tcp"
	tunnelProtoUDP   = "udp"

	// Checksum of the config, used to restart pods in the deployment
	tunnelConfigChecksum = "cfargotunnel.com/checksum"

	// Tunnel properties labels
	tunnelLabel          = "cfargotunnel.com/tunnel"
	isClusterTunnelLabel = "cfargotunnel.com/is-cluster-tunnel"
	tunnelIdLabel        = "cfargotunnel.com/id"
	tunnelNameLabel      = "cfargotunnel.com/name"
	tunnelKindLabel      = "cfargotunnel.com/kind"
	tunnelAppLabel       = "cfargotunnel.com/app"
	tunnelDomainLabel    = "cfargotunnel.com/domain"
	tunnelFinalizer      = "cfargotunnel.com/finalizer"
	configmapKey         = "config.yaml"
)

var tunnelValidProtoMap = map[string]bool{
	tunnelProtoHTTP:  true,
	tunnelProtoHTTPS: true,
	tunnelProtoRDP:   true,
	tunnelProtoSMB:   true,
	tunnelProtoSSH:   true,
	tunnelProtoTCP:   true,
	tunnelProtoUDP:   true,
}

func getAPIDetails(ctx context.Context, c client.Client, log logr.Logger, tunnelSpec networkingv1alpha2.TunnelSpec, tunnelStatus networkingv1alpha2.TunnelStatus, namespace string) (*CloudflareAPI, *corev1.Secret, error) {

	// Get secret containing API token
	cfSecret := &corev1.Secret{}
	if err := c.Get(ctx, apitypes.NamespacedName{Name: tunnelSpec.Cloudflare.Secret, Namespace: namespace}, cfSecret); err != nil {
		log.Error(err, "secret not found", "secret", tunnelSpec.Cloudflare.Secret)
		return &CloudflareAPI{}, &corev1.Secret{}, err
	}

	// Read secret for API Token
	cfAPITokenB64, okApiToken := cfSecret.Data[tunnelSpec.Cloudflare.CLOUDFLARE_API_TOKEN]

	// Read secret for API Key
	cfAPIKeyB64, okApiKey := cfSecret.Data[tunnelSpec.Cloudflare.CLOUDFLARE_API_KEY]

	if !(okApiKey || okApiToken) {
		err := fmt.Errorf("neither %s not %s found in secret %s, cannot construct client", tunnelSpec.Cloudflare.CLOUDFLARE_API_TOKEN, tunnelSpec.Cloudflare.CLOUDFLARE_API_KEY, tunnelSpec.Cloudflare.Secret)
		log.Error(err, "key not found in secret")
		return nil, nil, err
	}

	apiToken := string(cfAPITokenB64)
	apiKey := string(cfAPIKeyB64)
	apiEmail := tunnelSpec.Cloudflare.Email

	cloudflareClient, err := getCloudflareClient(apiKey, apiEmail, apiToken)
	if err != nil {
		log.Error(err, "error initializing cloudflare api client", "client", cloudflareClient)
		return &CloudflareAPI{}, &corev1.Secret{}, err
	}

	cfAPI := &CloudflareAPI{
		Log:              log,
		AccountName:      tunnelSpec.Cloudflare.AccountName,
		AccountId:        tunnelSpec.Cloudflare.AccountId,
		Domain:           tunnelSpec.Cloudflare.Domain,
		ValidAccountId:   tunnelStatus.AccountId,
		ValidTunnelId:    tunnelStatus.TunnelId,
		ValidTunnelName:  tunnelStatus.TunnelName,
		ValidZoneId:      tunnelStatus.ZoneId,
		CloudflareClient: cloudflareClient,
	}

	return cfAPI, cfSecret, nil
}

// getCloudflareClient returns an initialized *cloudflare.API using either an API Key + Email or an API Token
func getCloudflareClient(apiKey, apiEmail, apiToken string) (*cloudflare.API, error) {
	if apiToken != "" {
		return cloudflare.NewWithAPIToken(apiToken)
	} else {
		return cloudflare.New(apiKey, apiEmail)
	}
}

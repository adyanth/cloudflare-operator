package v1alpha1_test

import (
	"encoding/json"
	"testing"

	"github.com/adyanth/cloudflare-operator/api/v1alpha1"
	"github.com/adyanth/cloudflare-operator/api/v1alpha2"
	"gopkg.in/yaml.v3"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var sampleFilledOldTunnel = v1alpha1.Tunnel{
	ObjectMeta: metav1.ObjectMeta{
		Name:      "testTunnel",
		Namespace: "test",
	},
	Spec: v1alpha1.TunnelSpec{
		Size:          2,
		Image:         "newCloudflareImage",
		NoTlsVerify:   true,
		OriginCaPool:  "caPool",
		NodeSelectors: map[string]string{"zone": "na"},
		Tolerations: []corev1.Toleration{{
			Key:      "taint",
			Operator: "eq",
			Value:    "test",
		}},
		Protocol:       "udp",
		FallbackTarget: "http_status:400",
		Cloudflare: v1alpha1.CloudflareDetails{
			Domain:                              "example.test",
			Secret:                              "key-secret",
			AccountName:                         "cfAccount",
			AccountId:                           "101",
			Email:                               "cf@cf.com",
			CLOUDFLARE_API_KEY:                  "key",
			CLOUDFLARE_API_TOKEN:                "token",
			CLOUDFLARE_TUNNEL_CREDENTIAL_FILE:   "file",
			CLOUDFLARE_TUNNEL_CREDENTIAL_SECRET: "secret",
		},
		NewTunnel: v1alpha1.NewTunnel{
			Name: "tunnel-name",
		},
	},
	Status: v1alpha1.TunnelStatus{
		TunnelId:   "tunnel-id",
		TunnelName: "tunnel-name",
		AccountId:  "101",
		ZoneId:     "na",
	},
}

var sampleFilledDeployPatch = v1alpha1.V1alpha1Tov1alpha2Patch{
	Spec: &v1alpha1.V1alpha1Tov1alpha2PatchSpec{
		Replicas: 2,
		Template: &v1alpha1.V1alpha1Tov1alpha2PatchSpecTemplate{
			Spec: &v1alpha1.V1alpha1Tov1alpha2PatchSpecTemplateSpec{
				NodeSelector: sampleFilledOldTunnel.Spec.NodeSelectors,
				Tolerations:  sampleFilledOldTunnel.Spec.Tolerations,
				Containers: []v1alpha1.V1alpha1Tov1alpha2PatchSpecTemplateSpecContainer{{
					Name:  "cloudflared",
					Image: sampleFilledOldTunnel.Spec.Image,
				}},
			},
		},
	},
}

var sampleFilledNewTunnel = v1alpha2.Tunnel{
	ObjectMeta: metav1.ObjectMeta{
		Name:      "testTunnel",
		Namespace: "test",
	},
	Spec: v1alpha2.TunnelSpec{
		DeployPatch:    "", // Validate separately
		NoTlsVerify:    sampleFilledOldTunnel.Spec.NoTlsVerify,
		OriginCaPool:   sampleFilledOldTunnel.Spec.OriginCaPool,
		Protocol:       sampleFilledOldTunnel.Spec.Protocol,
		FallbackTarget: sampleFilledOldTunnel.Spec.FallbackTarget,
		Cloudflare: v1alpha2.CloudflareDetails{
			Domain:                              sampleFilledOldTunnel.Spec.Cloudflare.Domain,
			Secret:                              sampleFilledOldTunnel.Spec.Cloudflare.Secret,
			AccountName:                         sampleFilledOldTunnel.Spec.Cloudflare.AccountName,
			AccountId:                           sampleFilledOldTunnel.Spec.Cloudflare.AccountId,
			Email:                               sampleFilledOldTunnel.Spec.Cloudflare.Email,
			CLOUDFLARE_API_KEY:                  sampleFilledOldTunnel.Spec.Cloudflare.CLOUDFLARE_API_KEY,
			CLOUDFLARE_API_TOKEN:                sampleFilledOldTunnel.Spec.Cloudflare.CLOUDFLARE_API_TOKEN,
			CLOUDFLARE_TUNNEL_CREDENTIAL_FILE:   sampleFilledOldTunnel.Spec.Cloudflare.CLOUDFLARE_TUNNEL_CREDENTIAL_FILE,
			CLOUDFLARE_TUNNEL_CREDENTIAL_SECRET: sampleFilledOldTunnel.Spec.Cloudflare.CLOUDFLARE_TUNNEL_CREDENTIAL_SECRET,
		},
		NewTunnel: &v1alpha2.NewTunnel{
			Name: sampleFilledOldTunnel.Spec.NewTunnel.Name,
		},
	},
	Status: v1alpha2.TunnelStatus{
		TunnelId:   sampleFilledOldTunnel.Status.TunnelId,
		TunnelName: sampleFilledOldTunnel.Status.TunnelName,
		AccountId:  sampleFilledOldTunnel.Status.AccountId,
		ZoneId:     sampleFilledOldTunnel.Status.ZoneId,
	},
}

func TestUpgradeTunnel(t *testing.T) {
	// Convert
	convertedNewTunnel := &v1alpha2.Tunnel{}
	sampleFilledOldTunnel.ConvertTo(convertedNewTunnel)

	// Validate
	convertedDeployPatch := convertedNewTunnel.Spec.DeployPatch
	convertedDeployPatchObject := &v1alpha1.V1alpha1Tov1alpha2Patch{}
	if err := yaml.Unmarshal([]byte(convertedDeployPatch), convertedDeployPatchObject); err != nil {
		t.Errorf("Failed validating deploy patch %s", err)
	}

	convertedNewTunnel.Spec.DeployPatch = ""
	if equality.Semantic.DeepEqualWithNilDifferentFromEmpty(convertedNewTunnel, sampleFilledNewTunnel) {
		converted, err := json.Marshal(convertedNewTunnel)
		if err != nil {
			t.Errorf("Failed to marshal converted tunnel")
		}
		expected, err := json.Marshal(sampleFilledNewTunnel)
		if err != nil {
			t.Errorf("Failed to marshal sample tunnel")
		}
		t.Errorf("Failed to validate converted object.\nExpected: \n%+v\nConverted: \n%+v", string(expected), string(converted))
	}
}

func TestDowngradeTunnel(t *testing.T) {
	// Convert
	filledNewTunnel := sampleFilledNewTunnel.DeepCopy()
	deployPatch, err := yaml.Marshal(sampleFilledDeployPatch)
	if err != nil {
		t.Fail()
	}
	filledNewTunnel.Spec.DeployPatch = string(deployPatch)

	convertedOldTunnel := &v1alpha1.Tunnel{}
	convertedOldTunnel.ConvertFrom(filledNewTunnel)

	if equality.Semantic.DeepEqualWithNilDifferentFromEmpty(convertedOldTunnel, sampleFilledOldTunnel) {
		converted, err := json.Marshal(convertedOldTunnel)
		if err != nil {
			t.Errorf("Failed to marshal converted tunnel")
		}
		expected, err := json.Marshal(sampleFilledOldTunnel)
		if err != nil {
			t.Errorf("Failed to marshal sample tunnel")
		}
		t.Errorf("Failed to validate converted object.\nExpected: \n%+v\nConverted: \n%+v", string(expected), string(converted))
	}
}

package controller_test

import (
	"encoding/json"
	"testing"

	"github.com/adyanth/cloudflare-operator/internal/controller"
	"k8s.io/apimachinery/pkg/api/equality"
)

var ingresses []controller.UnvalidatedIngressRule = []controller.UnvalidatedIngressRule{
	{
		Hostname: "testing-a",
	},
	{
		Hostname: "testing-b",
		Path:     "/metrics",
	}, {
		Hostname: "testing-b",
		Path:     "/path/*",
	}, {
		Hostname: "testing-c",
	}, {
		Hostname: "testing-d",
		Path:     "/*",
	}, {
		Service: "http://svc",
	},
}

var config controller.Configuration = controller.Configuration{
	TunnelId:     "tunnel",
	NoAutoUpdate: true,
	Ingress:      ingresses,
}

func TestMapConfig(t *testing.T) {
	converted := config.ToMapConfig().ToConfig()
	compareAndPrintOnFail(t, config, converted)
}

func TestMapConfigWithEdits(t *testing.T) {
	mapConfig := config.ToMapConfig()
	newIngress := controller.UnvalidatedIngressRule{
		Hostname: "test1",
		Service:  "http://testsvc",
		Path:     "/*",
	}
	mapConfig.Ingress[newIngress.SelectKey()] = newIngress

	convertedIngress := mapConfig.ToConfig().Ingress
	compareAndPrintOnFail(t, convertedIngress[0], newIngress)

	convertedIngress = convertedIngress[1:]
	compareAndPrintOnFail(t, ingresses, convertedIngress)
}

func compareAndPrintOnFail(t *testing.T, config interface{}, converted interface{}) {
	if !equality.Semantic.DeepEqualWithNilDifferentFromEmpty(config, converted) {
		o, err := json.Marshal(config)
		if err != nil {
			t.Error(err)
		}
		c, err := json.Marshal(converted)
		if err != nil {
			t.Error(err)
		}
		t.Errorf("Original:\n%s\nConverted:\n%s\n", o, c)
	}
}

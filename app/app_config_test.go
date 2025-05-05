package app

import (
	"context"
	"github.com/SneaksAndData/nexus-core/pkg/checkpoint/request"
	"os"
	"reflect"
	"testing"
)

func getExpectedConfig(username string) *SupervisorConfig {
	return &SupervisorConfig{
		CqlStore: request.AstraBundleConfig{
			SecureConnectionBundleBase64: "base64value",
			GatewayUser:                  username,
			GatewayPassword:              "password",
		},
		KubeConfigPath:    "/tmp/kube",
		ResourceNamespace: "nexus",
	}
}

func Test_LoadConfig(t *testing.T) {
	var expected = getExpectedConfig("user")

	var result = LoadConfig(context.TODO())
	if !reflect.DeepEqual(*expected, result) {
		t.Errorf("LoadConfig failed, expected %v, got %v", *expected, result)
	}
}

func Test_LoadConfigFromEnv(t *testing.T) {
	_ = os.Setenv("NEXUS__CQL_STORE.GATEWAY_USER", "dev")
	var expected = getExpectedConfig("dev")

	var result = LoadConfig(context.TODO())
	if !reflect.DeepEqual(*expected, result) {
		t.Errorf("LoadConfig failed, expected %v, got %v", *expected, result)
	}
}

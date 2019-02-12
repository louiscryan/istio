package pilot

import (
	"fmt"
	"net"
	"path/filepath"
	"testing"
	"time"

	xdsapi "github.com/envoyproxy/go-control-plane/envoy/api/v2"
	xdscore "github.com/envoyproxy/go-control-plane/envoy/api/v2/core"

	"istio.io/istio/pilot/pkg/model"
	"istio.io/istio/pkg/test/framework"
	"istio.io/istio/pkg/test/framework/api/components"
	"istio.io/istio/pkg/test/framework/api/descriptors"
	"istio.io/istio/pkg/test/framework/api/ids"
	"istio.io/istio/pkg/test/framework/api/lifecycle"
	"istio.io/istio/pkg/test/util/structpath"
)

func TestSidecarListeners(t *testing.T) {
	ctx := framework.GetContext(t)
	// TODO: Limit to Native environment until the Kubernetes environment is supported in the Galley
	// component
	ctx.RequireOrSkip(t, lifecycle.Test, &descriptors.NativeEnvironment)
	pilotReq := components.ConfigurePilot("", components.PilotConfig{
		Galley: &ids.Galley,
	})
	// TODO: Shouldn't need to 'require' anything we're about to Get
	ctx.RequireOrFail(t, lifecycle.Test, pilotReq)
	ctx.RequireOrFail(t, lifecycle.Test, &ids.Mixer)

	// Get the port for mixer checks
	mixerCheckPort := components.GetMixer(ctx, t).GetCheckAddress().(*net.TCPAddr).Port
	pilot := components.GetPilot(ctx, t, pilotReq)

	// Simulate proxy identity of a sidecar ...
	nodeID := &model.Proxy{
		ClusterID:   "integration-test",
		Type:        model.SidecarProxy,
		IPAddresses: []string{"10.2.0.1"},
		ID:          "app3.testns",
		DNSDomains:  []string{"testns.cluster.local"},
	}

	// ... and get listeners from Pilot for that proxy
	req := &xdsapi.DiscoveryRequest{
		Node: &xdscore.Node{
			Id: nodeID.ServiceNode(),
		},
		TypeUrl: "type.googleapis.com/envoy.api.v2.Listener",
	}
	// Start the xDS stream
	err := pilot.StartDiscovery(req)
	if err != nil {
		t.Fatalf("Failed to test as no resource accepted: %v", err)
	}

	// Test the empty case where no config is loaded
	err = pilot.WatchDiscovery(time.Second*10,
		func(response *xdsapi.DiscoveryResponse) (b bool, e error) {
			validator := structpath.AssertThatProto(t, response)
			if !validator.Accept("{.resources[?(@.address.socketAddress.portValue==%v)]}", 15001) {
				return false, nil
			}
			validateListenersNoConfig(t, validator)
			return true, nil
		})
	if err != nil {
		t.Fatalf("Failed to test as no resource accepted: %v", err)
	}

	// Load the canonical dataset into Galley and the Pilot depending on it
	gal := components.GetGalley(ctx, t)
	// Apply some config
	path, err := filepath.Abs("../../testdata/config")
	if err != nil {
		t.Fatalf("No such directory: %v", err)
	}
	err = gal.ApplyConfigDir(path)
	if err != nil {
		t.Fatalf("Error applying directory: %v", err)
	}

	// Now continue to watch on the same stream
	err = pilot.WatchDiscovery(time.Second*5,
		func(response *xdsapi.DiscoveryResponse) (b bool, e error) {
			validator := structpath.AssertThatProto(t, response)
			if !validator.Accept("{.resources[?(@.address.socketAddress.portValue==8443)]}") {
				return false, nil
			}
			validateMixerAttachedToListener(t, validator, mixerCheckPort)
			return true, nil
		})
	if err != nil {
		t.Fatalf("Failed to test as no resource accepted: %v", err)
	}
}

func validateListenersNoConfig(t *testing.T, response *structpath.Structpath) {
	t.Run("validate-legacy-port-3333", func(t *testing.T) {
		// Deprecated: Should be removed as no longer needed
		response.ForTest(t).
			Select("{.resources[?(@.address.socketAddress.portValue==3333)]}").
			Equals("10.2.0.1", "{.address.socketAddress.address}").
			Equals("envoy.tcp_proxy", "{.filterChains[0].filters[*].name}").
			Equals("inbound|3333|http|mgmtCluster", "{.filterChains[0].filters[*].config.cluster}").
			Equals(false, "{.deprecatedV1.bindToPort}").
			NotExists("{.useOriginalDst}")
	})
	t.Run("validate-legacy-port-9999", func(t *testing.T) {
		// Deprecated: Should be removed as no longer needed
		response.ForTest(t).
			Select("{.resources[?(@.address.socketAddress.portValue==9999)]}").
			Equals("10.2.0.1", "{.address.socketAddress.address}").
			Equals("envoy.tcp_proxy", "{.filterChains[0].filters[*].name}").
			Equals("inbound|9999|custom|mgmtCluster", "{.filterChains[0].filters[*].config.cluster}").
			Equals(false, "{.deprecatedV1.bindToPort}").
			NotExists("{.useOriginalDst}")
	})
	t.Run("iptables-forwarding-listener", func(t *testing.T) {
		response.ForTest(t).
			Select("{.resources[?(@.address.socketAddress.portValue==15001)]}").
			Equals("virtual", "{.name}").
			Equals("0.0.0.0", "{.address.socketAddress.address}").
			Equals("envoy.tcp_proxy", "{.filterChains[0].filters[*].name}").
			Equals("BlackHoleCluster", "{.filterChains[0].filters[0].config.cluster}").
			Equals("BlackHoleCluster", "{.filterChains[0].filters[0].config.stat_prefix}").
			Equals(true, "{.useOriginalDst}")
	})
}

func validateMixerAttachedToListener(t *testing.T, response *structpath.Structpath, mixerCheckPort int) {
	t.Run("validate-mixer-decorates-listener", func(t *testing.T) {
		mixerListener := response.ForTest(t).
			Select("{.resources[?(@.address.socketAddress.portValue==%v)]}", 8443)

		mixerListener.
			Equals("0.0.0.0", "{.address.socketAddress.address}").
				// Example doing a struct comparison, note the pain with oneofs....
				Equals(&xdscore.SocketAddress{
					Address: "0.0.0.0",
					PortSpecifier: &xdscore.SocketAddress_PortValue{
						PortValue: uint32(8443),
					},
				}, "{.address.socketAddress}").
			Select("{.filterChains[0].filters[0]}").
			Equals("envoy.http_connection_manager", "{.name}").
			Equals(true, "{.config.generate_request_id}").
			Equals("mixer envoy.cors envoy.fault envoy.router", "{.config.http_filters[*].name}").
			Select("{.config}").
			Exists("{.rds.config_source.ads}").
			Exists("{.stat_prefix}").
			Equals(100, "{.tracing.client_sampling.value}").
			Equals(100, "{.tracing.overall_sampling.value}").
			Equals(100, "{.tracing.random_sampling.value}").
			Equals("EGRESS", "{.tracing.operation_name}").
			Equals("websocket", "{.upgrade_configs[*].upgrade_type}").
			Equals(false, "{.use_remote_address}")

		mixerListener.
			Equals(false, "{.deprecatedV1.bindToPort}").
			NotExists("{.useOriginalDst}")

		mixerListener.
			Select("{.filterChains[0].filters[0].config.http_filters[?(@.name==\"mixer\")].config}").
			Equals("kubernetes://app3.testns", "{.forward_attributes.attributes['source.uid'].string_value}").
			Equals("testns", "{.mixer_attributes.attributes['source.namespace'].string_value}").
			Equals("outbound", "{.mixer_attributes.attributes['context.reporter.kind'].string_value}").
			Equals(true, "{.service_configs.default.disable_check_calls}").
			Equals(fmt.Sprintf("outbound|%v||mixer.istio-system.svc.local", mixerCheckPort), "{.transport.check_cluster}").
			Equals(fmt.Sprintf("outbound|%v||mixer.istio-system.svc.local", mixerCheckPort), "{.transport.report_cluster}").
			Equals("FAIL_CLOSE", "{.transport.network_fail_policy.policy}")

	})
}

// Capturing TestMain allows us to:
// - Do cleanup before exit
// - process testing specific flags
func TestMain(m *testing.M) {
	framework.Run("sidecar_api_test", m)
}

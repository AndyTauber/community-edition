// Copyright 2021 VMware Tanzu Community Edition contributors. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package calico_test

import (
	"fmt"
	"path/filepath"
	"strings"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/yaml"

	"github.com/vmware-tanzu/community-edition/addons/packages/test/pkg/repo"
	"github.com/vmware-tanzu/community-edition/addons/packages/test/pkg/ytt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Calico Ytt Templates", func() {
	var (
		filePaths []string
		values    string
		output    string
		err       error

		configDir             = filepath.Join(repo.RootDir(), "addons/packages/calico/3.22.1/bundle/config")
		fileCalicoYaml        = filepath.Join(configDir, "upstream/calico.yaml")
		fileCalicoOverlayYaml = filepath.Join(configDir, "overlay/calico_overlay.yaml")
		fileValuesYaml        = filepath.Join(configDir, "values.yaml")
		fileValuesStar        = filepath.Join(configDir, "values.star")
	)

	desiredDaemonSetAnnotations := map[string]string{
		"kapp.k14s.io/disable-default-label-scoping-rules": "",
		"kapp.k14s.io/update-strategy":                     "fallback-on-replace",
	}
	desiredDeploymentAnnotations := map[string]string{
		"kapp.k14s.io/disable-default-label-scoping-rules": "",
		"kapp.k14s.io/update-strategy":                     "fallback-on-replace",
	}

	BeforeEach(func() {
		values = ""
	})

	JustBeforeEach(func() {
		filePaths = []string{fileCalicoYaml, fileCalicoOverlayYaml, fileValuesYaml, fileValuesStar}
		output, err = ytt.RenderYTTTemplate(ytt.CommandOptions{}, filePaths, strings.NewReader(values))
	})

	Context("default configuration", func() {
		It("renders a ConfigMap with a default ipam configuration", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(output).To(MatchYAML(`---
kind: ConfigMap
apiVersion: v1
metadata:
  name: calico-config
  namespace: kube-system
data:
  calico_backend: bird
  cni_network_config: |-
    {
      "name": "k8s-pod-network",
      "cniVersion": "0.3.1",
      "plugins": [
        {
          "type": "calico",
          "log_level": "info",
          "log_file_path": "/var/log/calico/cni/cni.log",
          "datastore_type": "kubernetes",
          "nodename": "__KUBERNETES_NODE_NAME__",
          "mtu": __CNI_MTU__,
          "ipam": {
              "type": "calico-ipam"
          },
          "policy": {
              "type": "k8s"
          },
          "kubernetes": {
              "kubeconfig": "__KUBECONFIG_FILEPATH__"
          }
        },
        {
          "type": "portmap",
          "snat": true,
          "capabilities": {"portMappings": true}
        },
        {
          "type": "bandwidth",
          "capabilities": {"bandwidth": true}
        }
      ]
    }
  typha_service_name: none
  veth_mtu: "0"
`))
		})

		It("renders a DaemonSet with container env settings", func() {
			Expect(err).NotTo(HaveOccurred())
			daemonSet := parseDaemonSet(output)
			containerEnvVars := daemonSet.Spec.Template.Spec.Containers[0].Env
			expectEnvVarValue(containerEnvVars, "IP", "autodetect")
			expectEnvVarValue(containerEnvVars, "FELIX_IPV6SUPPORT", "false")
			expectEnvVarValue(containerEnvVars, "CALICO_IPV4POOL_IPIP", "Always")
			expectEnvVarValue(containerEnvVars, "CALICO_IPV4POOL_VXLAN", "Never")

			Expect(envVarNames(containerEnvVars)).NotTo(ContainElement("CALICO_IPV4POOL_CIDR"))
			Expect(envVarNames(containerEnvVars)).NotTo(ContainElement("CALICO_IPV6POOL_CIDR"))
			Expect(envVarNames(containerEnvVars)).NotTo(ContainElement("IP6"))
			Expect(envVarNames(containerEnvVars)).NotTo(ContainElement("CALICO_ROUTER_ID"))
			Expect(envVarNames(containerEnvVars)).NotTo(ContainElement("CALICO_IPV6POOL_NAT_OUTGOING"))
		})

		It("renders the DaemonSet and Deployment with desired annotations", func() {
			Expect(err).NotTo(HaveOccurred())
			daemonSet := parseDaemonSet(output)
			deployment := parseDeployment(output)
			Expect(daemonSet.Annotations).To(Equal(desiredDaemonSetAnnotations))
			Expect(deployment.Annotations).To(Equal(desiredDeploymentAnnotations))
		})
	})

	Context("customize mtu configuration", func() {
		BeforeEach(func() {
			values = `#@data/values
#@overlay/match-child-defaults missing_ok=True
---
infraProvider: vsphere
calico:
  config:
    clusterCIDR: null
    vethMTU: "1440"
`
		})

		It("renders a ConfigMap with a mtu cusomized ipam configuration", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(output).To(MatchYAML(`---
kind: ConfigMap
apiVersion: v1
metadata:
  name: calico-config
  namespace: kube-system
data:
  calico_backend: bird
  cni_network_config: |-
    {
      "name": "k8s-pod-network",
      "cniVersion": "0.3.1",
      "plugins": [
        {
          "type": "calico",
          "log_level": "info",
          "log_file_path": "/var/log/calico/cni/cni.log",
          "datastore_type": "kubernetes",
          "nodename": "__KUBERNETES_NODE_NAME__",
          "mtu": __CNI_MTU__,
          "ipam": {
              "type": "calico-ipam"
          },
          "policy": {
              "type": "k8s"
          },
          "kubernetes": {
              "kubeconfig": "__KUBECONFIG_FILEPATH__"
          }
        },
        {
          "type": "portmap",
          "snat": true,
          "capabilities": {"portMappings": true}
        },
        {
          "type": "bandwidth",
          "capabilities": {"bandwidth": true}
        }
      ]
    }
  typha_service_name: none
  veth_mtu: "1440"
`))
		})

		It("renders a DaemonSet with container env settings", func() {
			Expect(err).NotTo(HaveOccurred())
			daemonSet := parseDaemonSet(output)
			containerEnvVars := daemonSet.Spec.Template.Spec.Containers[0].Env
			expectEnvVarValue(containerEnvVars, "IP", "autodetect")
			expectEnvVarValue(containerEnvVars, "FELIX_IPV6SUPPORT", "false")
			expectEnvVarValue(containerEnvVars, "CALICO_IPV4POOL_IPIP", "Always")
			expectEnvVarValue(containerEnvVars, "CALICO_IPV4POOL_VXLAN", "Never")

			Expect(envVarNames(containerEnvVars)).NotTo(ContainElement("CALICO_IPV4POOL_CIDR"))
			Expect(envVarNames(containerEnvVars)).NotTo(ContainElement("CALICO_IPV6POOL_CIDR"))
			Expect(envVarNames(containerEnvVars)).NotTo(ContainElement("IP6"))
			Expect(envVarNames(containerEnvVars)).NotTo(ContainElement("CALICO_ROUTER_ID"))
			Expect(envVarNames(containerEnvVars)).NotTo(ContainElement("CALICO_IPV6POOL_NAT_OUTGOING"))
		})

		It("renders the DaemonSet and Deployment with desired annotations", func() {
			Expect(err).NotTo(HaveOccurred())
			daemonSet := parseDaemonSet(output)
			deployment := parseDeployment(output)
			Expect(daemonSet.Annotations).To(Equal(desiredDaemonSetAnnotations))
			Expect(deployment.Annotations).To(Equal(desiredDeploymentAnnotations))
		})
	})

	Context("azure configuration with cidr", func() {
		BeforeEach(func() {
			values = `#@data/values
#@overlay/match-child-defaults missing_ok=True
---
infraProvider: azure
calico:
  config:
    clusterCIDR: "192.168.0.0/16"
`
		})

		It("renders a ConfigMap with a vxlan ipam configuration", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(output).To(MatchYAML(`---
kind: ConfigMap
apiVersion: v1
metadata:
  name: calico-config
  namespace: kube-system
data:
  calico_backend: vxlan
  cni_network_config: |-
    {
      "name": "k8s-pod-network",
      "cniVersion": "0.3.1",
      "plugins": [
        {
          "type": "calico",
          "log_level": "info",
          "log_file_path": "/var/log/calico/cni/cni.log",
          "datastore_type": "kubernetes",
          "nodename": "__KUBERNETES_NODE_NAME__",
          "mtu": __CNI_MTU__,
          "ipam": {
              "type": "calico-ipam"
          },
          "policy": {
              "type": "k8s"
          },
          "kubernetes": {
              "kubeconfig": "__KUBECONFIG_FILEPATH__"
          }
        },
        {
          "type": "portmap",
          "snat": true,
          "capabilities": {"portMappings": true}
        },
        {
          "type": "bandwidth",
          "capabilities": {"bandwidth": true}
        }
      ]
    }
  typha_service_name: none
  veth_mtu: "0"
`))
		})

		It("renders a DaemonSet with container env settings", func() {
			Expect(err).NotTo(HaveOccurred())
			daemonSet := parseDaemonSet(output)
			containerEnvVars := daemonSet.Spec.Template.Spec.Containers[0].Env
			expectEnvVarValue(containerEnvVars, "IP", "autodetect")
			expectEnvVarValue(containerEnvVars, "FELIX_IPV6SUPPORT", "false")
			expectEnvVarValue(containerEnvVars, "CALICO_IPV4POOL_IPIP", "Never")
			expectEnvVarValue(containerEnvVars, "CALICO_IPV4POOL_VXLAN", "Always")
			expectEnvVarValue(containerEnvVars, "CALICO_IPV4POOL_CIDR", "192.168.0.0/16")

			Expect(envVarNames(containerEnvVars)).NotTo(ContainElement("CALICO_IPV6POOL_CIDR"))
			Expect(envVarNames(containerEnvVars)).NotTo(ContainElement("IP6"))
			Expect(envVarNames(containerEnvVars)).NotTo(ContainElement("CALICO_ROUTER_ID"))
			Expect(envVarNames(containerEnvVars)).NotTo(ContainElement("CALICO_IPV6POOL_NAT_OUTGOING"))
		})
	})

	Context("IPv6 configuration", func() {
		BeforeEach(func() {
			values = `#@data/values
#@overlay/match-child-defaults missing_ok=True
---
ipFamily: ipv6
calico:
  config:
    clusterCIDR: "[fe80::1]/64"
`
		})

		It("renders a ConfigMap with IPv6 ipam configuration", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(output).To(MatchYAML(`---
kind: ConfigMap
apiVersion: v1
metadata:
  name: calico-config
  namespace: kube-system
data:
  calico_backend: bird
  # TODO: Make this pretty print:
  # https://github.com/vmware-tanzu/carvel-ytt/issues/410
  cni_network_config: '{"cniVersion":"0.3.1","name":"k8s-pod-network","plugins":[{"datastore_type":"kubernetes","ipam":{"assign_ipv4":"false","assign_ipv6":"true","type":"calico-ipam"},"kubernetes":{"kubeconfig":"__KUBECONFIG_FILEPATH__"},"log_file_path":"/var/log/calico/cni/cni.log","log_level":"info","mtu":__CNI_MTU__,"nodename":"__KUBERNETES_NODE_NAME__","policy":{"type":"k8s"},"type":"calico"},{"capabilities":{"portMappings":true},"snat":true,"type":"portmap"},{"capabilities":{"bandwidth":true},"type":"bandwidth"}]}'
  typha_service_name: none
  veth_mtu: "0"
`))
		})

		It("renders a DaemonSet with container env settings", func() {
			Expect(err).NotTo(HaveOccurred())
			daemonSet := parseDaemonSet(output)
			containerEnvVars := daemonSet.Spec.Template.Spec.Containers[0].Env
			expectEnvVarValue(containerEnvVars, "IP", "none")
			expectEnvVarValue(containerEnvVars, "FELIX_IPV6SUPPORT", "true")
			expectEnvVarValue(containerEnvVars, "CALICO_IPV6POOL_CIDR", "[fe80::1]/64")
			expectEnvVarValue(containerEnvVars, "IP6", "autodetect")
			expectEnvVarValue(containerEnvVars, "CALICO_ROUTER_ID", "hash")
			expectEnvVarValue(containerEnvVars, "CALICO_IPV6POOL_NAT_OUTGOING", "true")

			Expect(envVarNames(containerEnvVars)).NotTo(ContainElement("CALICO_IPV4POOL_IPIP"))
			Expect(envVarNames(containerEnvVars)).NotTo(ContainElement("CALICO_IPV4POOL_VXLAN"))
			Expect(envVarNames(containerEnvVars)).NotTo(ContainElement("CALICO_IPV4POOL_CIDR"))
		})
	})

	Context("IPv4,IPv6 dualstack configuration", func() {
		BeforeEach(func() {
			values = `#@data/values
#@overlay/match-child-defaults missing_ok=True
---
ipFamily: ipv4,ipv6
calico:
  config:
    clusterCIDR: "1.2.3.4/16,[fe80::1]/64"
`
		})

		It("renders a ConfigMap with dualstack IPv4,IPv6 ipam configuration", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(output).To(MatchYAML(`---
kind: ConfigMap
apiVersion: v1
metadata:
  name: calico-config
  namespace: kube-system
data:
  calico_backend: bird
  # TODO: Make this pretty print:
  # https://github.com/vmware-tanzu/carvel-ytt/issues/410
  cni_network_config: '{"cniVersion":"0.3.1","name":"k8s-pod-network","plugins":[{"datastore_type":"kubernetes","ipam":{"assign_ipv4":"true","assign_ipv6":"true","type":"calico-ipam"},"kubernetes":{"kubeconfig":"__KUBECONFIG_FILEPATH__"},"log_file_path":"/var/log/calico/cni/cni.log","log_level":"info","mtu":__CNI_MTU__,"nodename":"__KUBERNETES_NODE_NAME__","policy":{"type":"k8s"},"type":"calico"},{"capabilities":{"portMappings":true},"snat":true,"type":"portmap"},{"capabilities":{"bandwidth":true},"type":"bandwidth"}]}'
  typha_service_name: none
  veth_mtu: "0"
`))
		})

		It("renders a DaemonSet with container env settings", func() {
			Expect(err).NotTo(HaveOccurred())
			daemonSet := parseDaemonSet(output)
			containerEnvVars := daemonSet.Spec.Template.Spec.Containers[0].Env
			expectEnvVarValue(containerEnvVars, "IP", "autodetect")
			expectEnvVarValue(containerEnvVars, "CALICO_IPV4POOL_IPIP", "Always")
			expectEnvVarValue(containerEnvVars, "CALICO_IPV4POOL_VXLAN", "Never")
			expectEnvVarValue(containerEnvVars, "CALICO_IPV4POOL_CIDR", "1.2.3.4/16")
			expectEnvVarValue(containerEnvVars, "FELIX_IPV6SUPPORT", "true")
			expectEnvVarValue(containerEnvVars, "CALICO_IPV6POOL_CIDR", "[fe80::1]/64")
			expectEnvVarValue(containerEnvVars, "IP6", "autodetect")
			expectEnvVarValue(containerEnvVars, "CALICO_IPV6POOL_NAT_OUTGOING", "true")

			Expect(envVarNames(containerEnvVars)).NotTo(ContainElement("CALICO_ROUTER_ID"))
		})
	})

	Context("IPv6,IPv4 dualstack configuration", func() {
		BeforeEach(func() {
			values = `#@data/values
#@overlay/match-child-defaults missing_ok=True
---
ipFamily: ipv6,ipv4
calico:
  config:
    clusterCIDR: "[fe80::1]/64,1.2.3.4/16"
`
		})

		It("renders a ConfigMap with dualstack IPv6,IPv4 ipam configuration", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(output).To(MatchYAML(`---
kind: ConfigMap
apiVersion: v1
metadata:
  name: calico-config
  namespace: kube-system
data:
  calico_backend: bird
  # TODO: Make this pretty print:
  # https://github.com/vmware-tanzu/carvel-ytt/issues/410
  cni_network_config: '{"cniVersion":"0.3.1","name":"k8s-pod-network","plugins":[{"datastore_type":"kubernetes","ipam":{"assign_ipv4":"true","assign_ipv6":"true","type":"calico-ipam"},"kubernetes":{"kubeconfig":"__KUBECONFIG_FILEPATH__"},"log_file_path":"/var/log/calico/cni/cni.log","log_level":"info","mtu":__CNI_MTU__,"nodename":"__KUBERNETES_NODE_NAME__","policy":{"type":"k8s"},"type":"calico"},{"capabilities":{"portMappings":true},"snat":true,"type":"portmap"},{"capabilities":{"bandwidth":true},"type":"bandwidth"}]}'
  typha_service_name: none
  veth_mtu: "0"
`))
		})

		It("renders a DaemonSet with container env settings", func() {
			Expect(err).NotTo(HaveOccurred())
			daemonSet := parseDaemonSet(output)
			containerEnvVars := daemonSet.Spec.Template.Spec.Containers[0].Env
			expectEnvVarValue(containerEnvVars, "IP", "autodetect")
			expectEnvVarValue(containerEnvVars, "CALICO_IPV4POOL_IPIP", "Always")
			expectEnvVarValue(containerEnvVars, "CALICO_IPV4POOL_VXLAN", "Never")
			expectEnvVarValue(containerEnvVars, "CALICO_IPV4POOL_CIDR", "1.2.3.4/16")
			expectEnvVarValue(containerEnvVars, "FELIX_IPV6SUPPORT", "true")
			expectEnvVarValue(containerEnvVars, "CALICO_IPV6POOL_CIDR", "[fe80::1]/64")
			expectEnvVarValue(containerEnvVars, "IP6", "autodetect")
			expectEnvVarValue(containerEnvVars, "CALICO_IPV6POOL_NAT_OUTGOING", "true")

			Expect(envVarNames(containerEnvVars)).NotTo(ContainElement("CALICO_ROUTER_ID"))
		})
	})
})

func expectEnvVarValue(envVars []corev1.EnvVar, varName, expected string) {
	for _, envVar := range envVars {
		if envVar.Name == varName {
			failureTemplate := "Expected env var with Name \"%s\" to have value \"%s\", but was \"%s\".\n"
			Expect(envVar.Value).To(Equal(expected), fmt.Sprintf(failureTemplate, varName, expected, envVar.Value))
		}
	}
	failureTemplate := "\nNo env var with name \"%s\" (expected value \"%s\")\n"
	Expect(envVarNames(envVars)).To(ContainElement(varName), fmt.Sprintf(failureTemplate, varName, expected))
}

func envVarNames(envVars []corev1.EnvVar) []string {
	var names []string
	for _, envVar := range envVars {
		names = append(names, envVar.Name)
	}
	return names
}

func parseDaemonSet(output string) appsv1.DaemonSet {
	daemonSetDocIndex := 22
	daemonSetDoc := strings.Split(output, "---")[daemonSetDocIndex]
	var daemonSet appsv1.DaemonSet
	err := yaml.Unmarshal([]byte(daemonSetDoc), &daemonSet)
	Expect(err).NotTo(HaveOccurred())
	return daemonSet
}

func parseDeployment(output string) appsv1.Deployment {
	deploymentDocIndex := 24
	deploymentDoc := strings.Split(output, "---")[deploymentDocIndex]
	var deployment appsv1.Deployment
	err := yaml.Unmarshal([]byte(deploymentDoc), &deployment)
	Expect(err).NotTo(HaveOccurred())
	return deployment
}

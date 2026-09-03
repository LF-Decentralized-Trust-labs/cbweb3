// SPDX-License-Identifier: Apache-2.0

package composetemplate

import (
	"strings"

	"gopkg.in/yaml.v3"
)

type discDoc struct {
	Services map[string]struct {
		ContainerName string   `yaml:"container_name"`
		Ports         []string `yaml:"ports"`
	} `yaml:"services"`
	Networks map[string]struct {
		Name string `yaml:"name"`
	} `yaml:"networks"`
	Volumes map[string]struct {
		Name string `yaml:"name"`
	} `yaml:"volumes"`
}

// CheckNoCollision interpolates the template with two distinct-entity envs and
// reports any shared host port, container name, network name, or volume name.
func CheckNoCollision(t *Template, envA, envB map[string]string) Result {
	res := Result{OK: true}
	a := discriminants(t, envA)
	b := discriminants(t, envB)
	for kind, setA := range a {
		for val := range setA {
			if b[kind][val] {
				res.add("no-collision", kind+" collides between entities: "+val)
			}
		}
	}
	return res
}

func discriminants(t *Template, env map[string]string) map[string]map[string]bool {
	interp, _ := interpolate(t.Raw, env)
	var d discDoc
	_ = yaml.Unmarshal([]byte(interp), &d)
	out := map[string]map[string]bool{"port": {}, "container": {}, "network": {}, "volume": {}}
	for _, s := range d.Services {
		if s.ContainerName != "" {
			out["container"][s.ContainerName] = true
		}
		for _, p := range s.Ports {
			if hp := hostPort(p); hp != "" {
				out["port"][hp] = true
			}
		}
	}
	for k, n := range d.Networks {
		name := n.Name
		if name == "" {
			name = k
		}
		out["network"][name] = true
	}
	for k, v := range d.Volumes {
		name := v.Name
		if name == "" {
			name = k
		}
		out["volume"][name] = true
	}
	return out
}

// hostPort extracts the host side of a compose port mapping:
// "8845:8545" → "8845"; "8845:8545/udp" → "8845"; "127.0.0.1:8845:8545" → "8845";
// bare "8545" (no host mapping) → "".
func hostPort(p string) string {
	p = strings.TrimSuffix(strings.TrimSuffix(p, "/udp"), "/tcp")
	parts := strings.Split(p, ":")
	switch len(parts) {
	case 2:
		return parts[0]
	case 3:
		return parts[1]
	default:
		return ""
	}
}

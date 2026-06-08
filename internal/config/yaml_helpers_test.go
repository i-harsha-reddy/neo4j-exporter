package config

import "gopkg.in/yaml.v3"

// yamlUnmarshal is a test helper that unmarshals a scalar YAML doc into
// a wrapper struct and copies the embedded EnableMode out. Kept in a
// separate _test.go file so production code doesn't depend on yaml.v3
// from non-config packages.
func yamlUnmarshal(data []byte, target any, out *EnableMode) error {
	if err := yaml.Unmarshal(data, target); err != nil {
		return err
	}
	type wrap struct {
		V EnableMode `yaml:"v"`
	}
	var w wrap
	if err := yaml.Unmarshal(data, &w); err != nil {
		return err
	}
	*out = w.V
	return nil
}

// Copyright (c) 2020 SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package configuration

// DMZCIDRRange represents a string or a list of strings
type DMZCIDRRange struct {
	Values []string
}

//UnmarshalYAML unmarshall the DMZCIDRRange
func (sm *DMZCIDRRange) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var multi []string
	err := unmarshal(&multi)
	if err != nil {
		var single string
		err := unmarshal(&single)
		if err != nil {
			return err
		}
		sm.Values = make([]string, 1)
		sm.Values[0] = single
	} else {
		sm.Values = multi
	}
	return nil
}

//MarshalYAML marshall the DMZCIDRRange
func (sm DMZCIDRRange) MarshalYAML() (interface{}, error) {
	if len(sm.Values) == 1 {
		return sm.Values[0], nil
	}
	return sm.Values, nil
}

// SpotInstanceSpec describe the agnostic representation of the spotInstance
type SpotInstanceSpec struct {
	Enabled bool    `yaml:"enabled"`
	Price string 	`yaml:"price"`
}
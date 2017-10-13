package main

// Copyright (C) 2017 Cisco Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//   http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

import (
	"fmt"
        "os"

	"gopkg.in/yaml.v2"
)

type buildTarget struct {
	target     string
	base       string
	run        []string
	expand     []string
	install    []string
	entrypoint string
}

type buildRecipe struct {
	Targets []buildTarget
}

func (bt *buildRecipe) HasTarget(q string) bool {
	for _, t := range bt.Targets {
		if t.target == q {
			return true
		}
	}
	return false
}

func (r *buildRecipe) SanityCheck(c *stackerConfig) bool {
	for k, v := range r.Targets {
		if v.base == "" {
			fmt.Fprintf(os.Stderr, "No base defined for target %s\n", k)
			return false
		}
		if !c.OCITagExists(v.base) && !r.HasTarget(v.base) && v.base != "empty" {
			fmt.Fprintf(os.Stderr, "Nonexistent base: %s\n", v.base)
			return false
		}
		if len(v.run) == 0 && len(v.expand) == 0 && len(v.expand) == 0 && len(v.install) == 0 && v.entrypoint == "" {
			fmt.Fprintf(os.Stderr, "No work for target: %s\n", k)
			return false
		}
	}

	return true
}

func (bt *buildTarget) setBase(i interface{}) error {
	switch i.(type) {
	case string:
	default:
		return fmt.Errorf("Parse error reading base at %s", bt.target)
	}
	if bt.base != "" {
		return fmt.Errorf("Duplicate base for %s", bt.target)
	}
	bt.base = i.(string)
	return nil
}

func (bt *buildTarget) appendRun(i interface{}) error {
	switch i.(type) {
	case string:
		bt.run = append(bt.run, i.(string))
	case []interface{}:
		for _, e := range i.([]interface{}) {
			switch e.(type) {
			case string:
				bt.run = append(bt.run, e.(string))
			default:
				return fmt.Errorf("Parse error at run step for %s", bt.target)
			}
		}
	default:
		return fmt.Errorf("Parse error at run step for %s", bt.target)
	}
	return nil
}

func (bt *buildTarget) appendInstall(i interface{}) error {
	switch i.(type) {
	case string:
		bt.install = append(bt.install, i.(string))
	case []interface{}:
		for _, e := range i.([]interface{}) {
			switch e.(type) {
			case string:
				bt.install = append(bt.install, e.(string))
			default:
				return fmt.Errorf("Parse error at install step for %s", bt.target)
			}
		}
	default:
		return fmt.Errorf("Parse error at install step for %s", bt.target)
	}
	return nil
}

func (bt *buildTarget) appendExpand(i interface{}) error {
	switch i.(type) {
	case string:
		bt.expand = append(bt.expand, i.(string))
	case []interface{}:
		for _, e := range i.([]interface{}) {
			switch e.(type) {
			case string:
				bt.expand = append(bt.expand, e.(string))
			default:
				return fmt.Errorf("Parse error at expand step for %s", bt.target)
			}
		}
	default:
		return fmt.Errorf("Parse error at expand step for %s", bt.target)
	}
	return nil
}

func (bt *buildTarget) setCmd(i interface{}) error {
	switch i.(type) {
	case string:
	default:
		return fmt.Errorf("Parse error reading entrypoint at %s", bt.target)
	}
	bt.entrypoint = i.(string)
	return nil
}

// Parse a recipe that looks like:
// target1:
//   base: empty
//   expand: some.tar.xz
// target2:
//   base: target1
//   run: echo hw > /helloworld
func parseRecipe(contents []byte) (r *buildRecipe, err error) {
	var i interface{}
	r = &buildRecipe{}
	err = yaml.Unmarshal(contents, &i)
	if err != nil {
		return
	}
	m := i.(map[interface{}] interface{})
	for k, v := range m {
		switch k.(type) {
		case string:
		default:
			err = fmt.Errorf("Parser error")
			return
		}
		bt := buildTarget{ target: k.(string) }
		step := v.(map[interface{}]interface{})
		for s, t := range step {
			switch s.(type) {
			case string:
			default:
				err = fmt.Errorf("Parse error at %s", bt.target)
				return
			}
			ss := s.(string)
			switch ss {
			case "base":
				err = bt.setBase(t)
			case "run":
				err = bt.appendRun(t)
			case "install":
				err = bt.appendInstall(t)
			case "expand":
				err = bt.appendExpand(t)
			case "entrypoint", "cmd":
				err = bt.setCmd(t)
			default:
				err = fmt.Errorf("Parser error at %s: unknown keyword %s", bt.target, ss)
			}
			if err != nil {
				return
			}
		}
		r.Targets = append(r.Targets, bt)
	}
	return
}


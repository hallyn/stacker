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
	"io/ioutil"
	"os"

	"github.com/openSUSE/umoci/oci/cas/dir"
	"github.com/openSUSE/umoci/oci/casext"
	"golang.org/x/net/context"
	"gopkg.in/yaml.v2"
)

func usage() {
	fmt.Printf("Usage: %s [COMMAND] [ARGUMENTS]\n", os.Args[0])
	fmt.Printf("Commands\n")
	fmt.Printf("   build BUILDFILE\n")
	fmt.Printf("   config show\n")
	fmt.Printf("   ls\n")
}

type stackerConfig struct {
	BaseDir	   string `yaml:"basedir"`
	OciDir	   string `yaml:"ocidir"`
	FsType     string `yaml:"fstype"`
	LoFile     string `yaml:"lofile"`
	BtrfsMount string `yaml:"btrfsmount"`
}

func (c *stackerConfig) Initialize() error {
	fileName := "stacker_config.yml"
	contents, err := ioutil.ReadFile(fileName)
	if os.IsNotExist(err) {
		fileName := "~/.config/stacker_config.yml"
		contents, err = ioutil.ReadFile(fileName)
	}
	if os.IsNotExist(err) {
		return nil
	}

	fmt.Printf("contents is %v\n", string(contents))
	tmp := &stackerConfig{}
	if err != nil && !os.IsNotExist(err) {
		fmt.Printf("Error reading %s: %v\n", fileName, err)
		return nil
	}
	err = yaml.Unmarshal(contents, tmp)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading %s: %s",
			   fileName, err)
		return err
	}
	fmt.Printf("tmp now has: %v\n", tmp)

	// Deduce some relative paths
	if tmp.BaseDir != "" && tmp.OciDir == "" {
		tmp.OciDir = tmp.BaseDir + "/oci"
	}
	if tmp.BaseDir != "" && tmp.FsType == "btrfs" && tmp.BtrfsMount == "" {
		tmp.BtrfsMount = tmp.BaseDir + "/btrfs"
	}

	// Now copy it over
	if tmp.BaseDir != "" {
		c.BaseDir = tmp.BaseDir
	}
	if tmp.OciDir != "" {
		c.OciDir = tmp.OciDir
	}
	if tmp.FsType != "" {
		c.FsType = tmp.FsType
	}
	if tmp.LoFile != "" {
		c.LoFile = tmp.LoFile
	}
	if tmp.BtrfsMount != "" {
		c.BtrfsMount = tmp.BtrfsMount
	}
	return nil
}

func (config *stackerConfig) Show() {
	fmt.Printf("basedir: %s\n", config.BaseDir)
	fmt.Printf("ocidir: %s\n", config.OciDir)
	fmt.Printf("fs driver: %s\n", config.FsType)
	switch config.FsType {
	case "btrfs":
		if config.LoFile != "" {
			fmt.Printf("  loopback file: %s\n", config.LoFile)
			// TODO - detect whether it's created
			fmt.Printf("  mountpoint: %s\n", config.BtrfsMount)
			// TODO - detect whether it's mounted
		}
	case "zfs":
		fmt.Printf("   Note zfs is not yet supported")
	case "lvm":
		fmt.Printf("   Note LVM is not yet supported")
	default:
	}
}

func (c *stackerConfig) UnpackDir() string {
	switch c.FsType {
	case "btrfs":
		if c.BtrfsMount != "" {
			return c.BtrfsMount + "/mounted"
		}
		return c.BaseDir + "/btrfs/mounted"
	default:
		return c.BaseDir + "unpacked"
	}
}

var config = &stackerConfig{
	BaseDir: ".",
	OciDir: "./oci",
	FsType: "vfs",
}

// Unpack and repack operations.  These will be used by build,
// unpack, checkout and checkin.

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

func (c *stackerConfig) SanityCheck(r *buildRecipe) bool {
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
			fmt.Fprintf(os.Stderr, "Parse error")
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

func alreadyBuilt(built []string, q string) bool {
	for _, s := range built {
		if q == s {
			return true
		}
	}
	return false
}

// Build a recipe
func (c *stackerConfig) Build() bool {
	if len(os.Args) < 3 {
		usage()
		return false
	}
	buildFile := os.Args[2]
	contents, err := ioutil.ReadFile(buildFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error opening recipe file %s: %v\n",
			    buildFile, err)
		return false
	}
	recipe, err := parseRecipe(contents)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing recipe: %v\n", err)
		return false
	}

	if !c.SanityCheck(recipe) {
		return false
	}

	// Now follow the recipe
	deferred := recipe.Targets
	built := []string{}
	for len(deferred) != 0 {
		targets := deferred
		deferred = []buildTarget{}
		fmt.Printf("Built: %v; targets: %v\n", built, targets)
		for _, t := range targets {
			if t.base != "empty" && !alreadyBuilt(built, t.base) && !c.OCITagExists(t.base) {
				deferred = append(deferred, t)
				continue
			}
			built = append(built, t.target)
		}
	}

	return true
}

// Note -if cmd/umoci/tag.go:tagList() did not take a cli.Contenxt,
// then I could simply use that here.
// I might give in and use urfave as well one day, but the point about
// general re-usability remains
func (c *stackerConfig) ListTags() ([]string, error) {
	image, err := dir.Open(config.OciDir)
	if err != nil {
		return []string{}, err
	}
	engine := casext.NewEngine(image)
	defer image.Close()

	names, err := engine.ListReferences(context.Background())
	if err != nil {
		return []string{}, err
	}

	return names, nil
}

func (c *stackerConfig) Ls() bool {
	names, err := c.ListTags()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error listing tags: %v\n", err)
		return false
	}

	for _, name := range names {
		fmt.Println(name)
	}
	return true
}

func (c *stackerConfig) OCITagExists(q string) bool {
	ls, err := c.ListTags()
	if err != nil {
		return false
	}
	for _, tag := range ls {
		if tag == q {
			return true
		}
	}
	return false
}

func doConfig() (ret bool) {
	ret = false
	if len(os.Args) < 3 {
		usage()
		return
	}
	switch os.Args[2] {
	case "show":
		config.Show()
		ret = true
	default:
		usage()
	}
	return
}

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(1)
	}

	if err := config.Initialize(); err != nil {
		os.Exit(1)
	}

	switch os.Args[1] {
	case "build":
		if !config.Build() {
			os.Exit(1)
		}
	case "help":
		usage()
		os.Exit(0)
	case "ls":
		if !config.Ls() {
			os.Exit(1)
		}
	case "config":
		if !doConfig() {
			os.Exit(1)
		}
	default:
		usage()
		os.Exit(1)
	}
	os.Exit(0)
}

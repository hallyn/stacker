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
	"bufio"
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
        "os/exec"
        "strings"

	"github.com/openSUSE/umoci/oci/cas/dir"
	"github.com/openSUSE/umoci/oci/casext"
	"gopkg.in/yaml.v2"
	"golang.org/x/net/context"
)

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
		return c.BaseDir + "/unpacked"
	}
}

func (c *stackerConfig) RootfsDir() string {
	switch c.FsType {
	case "btrfs":
		if c.BtrfsMount != "" {
			return c.BtrfsMount
		}
		return c.BaseDir + "/btrfs/mounted"
	default:
		return c.BaseDir + "/unpacked/rootfs"
	}
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
func (c *stackerConfig) Build(buildFile string) error {
	contents, err := ioutil.ReadFile(buildFile)
	if err != nil {
		return fmt.Errorf("Error opening recipe file %s: %v", buildFile, err)
	}
	recipe, err := parseRecipe(contents)
	if err != nil {
		return fmt.Errorf("Error parsing recipe: %v", err)
	}

	if !recipe.SanityCheck(c) {
		return fmt.Errorf("Recipe error (TODO show error details")
	}

	// Now follow the recipe
	deferred := recipe.Targets
	built := []string{}
	for len(deferred) != 0 {
		targets := deferred
		deferred = []buildTarget{}
		// How do we want to report progress?
		//fmt.Printf("Built: %v; targets: %v\n", built, targets)
		for _, t := range targets {
			if t.base != "empty" && !alreadyBuilt(built, t.base) && !c.OCITagExists(t.base) {
				deferred = append(deferred, t)
				continue
			}
			built = append(built, t.target)
		}
	}

	return nil
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

func (c *stackerConfig) CheckoutTag(tag string) bool {
	if dirExists(c.UnpackDir()) {
		fmt.Fprintf(os.Stderr, "%s is not empty\n", c.UnpackDir())
		return false
	}
	switch c.FsType {
	case "vfs":
		return VfsExpandLayer(c.OciDir, tag, c.UnpackDir())
	case "btrfs":
		return btrfsClone(c, tag)
	default:
		fmt.Fprintf(os.Stderr, "Unsupported fs type")
		return false
	}
	return true
}

func (c *stackerConfig) AbortCheckout(force bool) (failed bool, err error) {
	if !dirExists(c.UnpackDir()) {
		return false, fmt.Errorf("Nothing to abort")
	}
	if !force {
		fmt.Printf("Really delete '%s'? (y/n)", c.UnpackDir())
		reader := bufio.NewReader(os.Stdin)
		input, _ := reader.ReadString('\n')
		res := string([]byte(input)[0])
		if res != "y" && res != "Y" {
			return false, fmt.Errorf("Aborting.")
		}
	}

	switch c.FsType {
	case "vfs":
		cmd := exec.Command("rm", "-rf", c.UnpackDir())  // Whoa
		if err := cmd.Run(); err != nil {
			return true, fmt.Errorf("Removal failed: %v", err)
		}
		return false, nil
	default:
		return true, fmt.Errorf("Unsupported fs type")
	}
	return false, nil
}

//
// desired API functionality:
// image, err := umoci.Open(c.OciDir, tag)
// defer image.Close()
// if err != nil {
// 	return "", err
// }
// return image.Layers[image.NumLayers].Digest.Encoded(), nil
func (c *stackerConfig) GetTagDigest(tag string) (string, error) {
	image := fmt.Sprintf("%s:%s", c.OciDir, tag)
	cmd := exec.Command("umoci", "stat", "--image", image)
	var buf bytes.Buffer
	cmd.Stdout = &buf
	if err := cmd.Run(); err != nil {
		return "", err
	}
	lines := strings.Split(buf.String(), "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		line := lines[i]
		if line[0:7] == "sha256:" {
			return line[7:], nil
		}
	}
	return "", fmt.Errorf("not found")
}


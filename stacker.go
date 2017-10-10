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

var config = &stackerConfig{
	BaseDir: ".",
	OciDir: "./oci",
	FsType: "vfs",
}

func doBuild() bool {
	return true
}

// Note -if cmd/umoci/tag.go:tagList() did not take a cli.Contenxt,
// then I could simply use that here.
// I might give in and use urfave as well one day, but the point about
// general re-usability remains
func doLs() bool {
	image, err := dir.Open(config.OciDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error opening layout: %s\n", err)
		return false
	}
	engine := casext.NewEngine(image)
	defer image.Close()

	names, err := engine.ListReferences(context.Background())
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading layout tags: %s\n", err)
		return false
	}

	for _, name := range names {
		fmt.Println(name)
	}
	return true
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
		if !doBuild() {
			os.Exit(1)
		}
	case "help":
		usage()
		os.Exit(0)
	case "ls":
		if !doLs() {
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

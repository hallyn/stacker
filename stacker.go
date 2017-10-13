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
)

func FileExists(dir string) bool {
	stat, err := os.Stat(dir)
	if err != nil && os.IsNotExist(err) {
		return false
	}
	if stat.IsDir() {
		fmt.Fprintf(os.Stderr, "ERROR: %s exists but is a directory")
		return false
	}
	return true
}

func dirExists(dir string) bool {
	stat, err := os.Stat(dir)
	if err != nil && os.IsNotExist(err) {
		return false
	}
	if !stat.IsDir() {
		fmt.Fprintf(os.Stderr, "ERROR: %s exists but is not a directory")
		return false
	}
	return true
}

func usage() {
	fmt.Printf("Usage: %s [COMMAND] [ARGUMENTS]\n", os.Args[0])
	fmt.Printf("Commands\n")
	fmt.Printf("   abort [-f]: remove the checked-out rootfs\n")
	fmt.Printf("   build BUILDFILE: build OCI tags per the recipe in BUILDFILE\n")
	fmt.Printf("   checkin NEWTAG: check in the checked-out rootfs as NEWTAG\n")
	fmt.Printf("   checkout TAG: check out the rootfs for OCI tag TAG\n")
	fmt.Printf("   config show: show current configuration\n")
	fmt.Printf("   chroot: run a chroot in checked-out fs\n")
	fmt.Printf("   ls: list the OCi tags\n")
	fmt.Printf("   lxc: open a container in checked-out fs\n")
	fmt.Printf("   losetup: set up loopback for configured fstype\n")
	fmt.Printf("   lounsetup: undo loopback setup for configured fstype\n")
}

var config = &stackerConfig{
	BaseDir: ".",
	OciDir: "./oci",
	FsType: "vfs",
}

// Unpack and repack operations.  These will be used by build,
// unpack, checkout and checkin.
// Obviously these are to be replaced with actual use of the 
// umoci/oci libraries.

// copied from umoci's stat code.  This should all be replaced with some
// simple API calls.
func doLs(c *stackerConfig) bool {
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

func Checkout(c *stackerConfig) bool {
	if len(os.Args) < 3 {
		usage()
		return false
	}
	tag := os.Args[2]

	return c.CheckoutTag(tag)
}

func Abort(c *stackerConfig) bool {
	force := false
	if len(os.Args) > 2 && (os.Args[2] == "-f"  || os.Args[2] == "--force") {
		force = true
	}

	failed, err := c.AbortCheckout(force)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Abort failed: %v\n", err)
	}
	return !failed
}

// Build a recipe
func Build(c *stackerConfig) bool {
	if len(os.Args) < 3 {
		usage()
		return false
	}

	buildFile := os.Args[2]

	err := c.Build(buildFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Build error: %v\n", err)
		return false
	}
	return true
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
		if !Build(config) {
			os.Exit(1)
		}
	case "help":
		usage()
		os.Exit(0)
	case "ls":
		if !doLs(config) {
			os.Exit(1)
		}
	case "config":
		if !doConfig() {
			os.Exit(1)
		}
	case "checkout":
		if !Checkout(config) {
			os.Exit(1)
		}
	case "abort":
		if !Abort(config) {
			os.Exit(1)
		}
	case "losetup":
		if err := config.LoSetup(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	case "lounsetup":
		if err := config.LoUnSetup(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	default:
		usage()
		os.Exit(1)
	}
	os.Exit(0)
}

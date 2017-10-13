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
	"os/exec"
)

func btrfsClone(c *stackerConfig, tag string) bool {
	sha, err := c.GetTagDigest(tag)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed opening tag: %v\n", err)
		return false
	}

	lower := fmt.Sprintf("%s/%s", c.BtrfsMount, tag)
	cmd := exec.Command("btrfs", "subvolume", "snapshot", lower, c.UnpackDir())
	if err = cmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "btrfs subvolume snapshot failed: %v\n", err)
		return false
	}
	d := []byte(tag)
	fileName := fmt.Sprintf("%s/btrfs.mounted_tag", c.BaseDir)
	if err = ioutil.WriteFile(fileName, d, 0644); err != nil {
		fmt.Fprintf(os.Stderr, "Error saving the checked out tag: %s\n", err)
	}
	d = []byte(sha)
	fileName = fmt.Sprintf("%s/btrfs.mounted_sha", c.BaseDir)
	if err = ioutil.WriteFile(fileName, d, 0644); err != nil {
		fmt.Fprintf(os.Stderr, "Error saving the checked out hash: %s\n", err)
	}
	return true
}


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
	"path/filepath"
        "syscall"
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

func IsMountpoint(mnt string) bool {
	cmd := exec.Command("mountpoint", "-q", mnt)
	if err := cmd.Run(); err != nil {
		return false
	}
	return true
}

func btrfs_loSetup(lofile, mnt string) error {
	if !FileExists(lofile) {
		cmd := exec.Command("truncate", "-s", "20G", lofile)
		if err := cmd.Run(); err != nil {
			return err
		}
		cmd = exec.Command("mkfs.btrfs", lofile)
		if err := cmd.Run(); err != nil {
			return err
		}
	}
	if !IsMountpoint(mnt) {
		if err := os.MkdirAll(mnt, 0755); err != nil {
			return err
		}
		cmd := exec.Command("mount", "-o", "loop", "-t", "btrfs", lofile, mnt)
		if err := cmd.Run(); err != nil {
			return err
		}
	}
	return nil
}

func btrfs_loUnsetup(lofile, mnt string) error {
	if IsMountpoint(mnt) {
		if err := syscall.Unmount(mnt, syscall.MNT_DETACH); err != nil {
			return err
		}
	}
	if FileExists(lofile) {
		if err := os.Remove(lofile); err != nil {
			return err
		}
	}
	return nil
}

// desired umoci API:
// image, err := umoci.OpenLayout(ocidir)
// defer image.Close()
// for _, tag := image.ListTags() {
//	prevlayer = ""
//	for _, l := image.ListTagLayers(tag) {
// 		if prevlayer == "" {
//			create_btrfs_subvolume(mnt, image.GetTagLayerDigest(l))
//		} else {
//			prevsha = image.GetTagLayerDigest(prevlayer)
//			newsha = image.GetTagLayerDigest(l)
//			clone_btrfs_subvolume(mnt, prevsha, newsha)
//			image.Unpack(newsha, fmt.Sprintf("%s/%s", mnt, newsha)
//		}
// }
func btrfs_Unpack(c *stackerConfig) error {
	tags, err := c.ListTags()
	if err != nil {
		return err
	}
	tmpDir, err := ioutil.TempDir("", "stacker_")
	if err != nil {
		return err
	}
	tmpRootfs := fmt.Sprintf("%s/rootfs", tmpDir)
	for _, tag := range(tags) {
		os.RemoveAll(tmpDir)
		os.MkdirAll(tmpDir, 0755)
		layers, err := c.TagFsLayers(tag)
		if err != nil {
			return err
		}
		prevlayer := ""
		for _, l := range layers {
			if prevlayer == "" {
				if err := CreateSubvol(c.BtrfsMount, l); err != nil {
					return err
				}
			} else {
				if err := SnapshotSubvol(c.BtrfsMount, prevlayer, l); err != nil {
					return err
				}
			}

			image := fmt.Sprintf("%s:%s", c.OciDir, l)
			// this will fail rihgt now as umoci does not support it
			cmd := exec.Command("umoci", "unpack", "--image", image, tmpDir)
			if err := cmd.Run(); err != nil {
				return err
			}

			destDir := filepath.Join(c.BtrfsMount, l)
			cmd = exec.Command("rsync", "-Hax", "--numeric-ids", "--sparse",
					    "--delete", "--devices", tmpRootfs, destDir)
			if err := cmd.Run(); err != nil {
				return err
			}
		}
	}
	return nil
}

func CreateSubvol(mnt, dir string) error {
	dest := filepath.Join(mnt, dir)
	cmd := exec.Command("btrfs", "subvolume", "create", dest)
	return cmd.Run()
}

func SnapshotSubvol(mnt, old, dir string) error {
	src := filepath.Join(mnt, old)
	dest := filepath.Join(mnt, dir)
	cmd := exec.Command("btrfs", "subvolume", "snapshot", src, dest)
	return cmd.Run()
}

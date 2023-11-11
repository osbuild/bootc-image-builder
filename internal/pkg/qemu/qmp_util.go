// Copyright 2020 Red Hat, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// The Qemu Machine Protocol - to remotely query and operate a qemu instance (https://wiki.qemu.org/Documentation/QMP)

package qemu

import (
	"encoding/json"
	"fmt"

	"github.com/pkg/errors"
)

// QOMDev is a QMP monitor, for interactions with a QEMU instance.
type QOMDev struct {
	Return []struct {
		Name string `json:"name"`
		Type string `json:"type"`
	} `json:"return"`
}

type QOMBlkDev struct {
	Return []struct {
		Device     string `json:"device"`
		DevicePath string `json:"qdev"`
		Removable  bool   `json:"removable"`
		Inserted   struct {
			BackingFileDepth int    `json:"backing_file_depth"`
			NodeName         string `json:"node-name"`
		} `json:"inserted"`
	} `json:"return"`
}

// runQmpCommand executes a qemu command over the QMP socket.
func (inst *QemuInstance) runQmpCommand(cmd string) ([]byte, error) {
	if inst.qmpSocket == nil {
		return nil, errors.New("qmp socket is not open")
	}
	return inst.qmpSocket.Run([]byte(cmd))
}

// Executes a query which provides the list of block devices and their names.
func (inst *QemuInstance) listBlkDevices() (*QOMBlkDev, error) {
	listcmd := `{ "execute": "query-block" }`
	out, err := inst.runQmpCommand(listcmd)
	if err != nil {
		return nil, errors.Wrapf(err, "Running QMP query-block command")
	}

	var devs QOMBlkDev
	if err = json.Unmarshal(out, &devs); err != nil {
		return nil, errors.Wrapf(err, "De-serializing QMP query-block output")
	}
	return &devs, nil
}

// setBootIndexForDevice uses the qmp socket to the bootindex for the particular device.
func (inst *QemuInstance) setBootIndexForDevice(device string, bootindex int) error {
	cmd := fmt.Sprintf(`{ "execute":"qom-set", "arguments": { "path":"%s", "property":"bootindex", "value":%d } }`,
		device, bootindex)
	if _, err := inst.runQmpCommand(cmd); err != nil {
		return errors.Wrapf(err, "Setting bootindex of device %s to %d", device, bootindex)
	}
	return nil
}

// deleteBlockDevice uses the qmp socket to remote a block device.
func (inst *QemuInstance) deleteBlockDevice(device string) error {
	cmd := fmt.Sprintf(`{ "execute": "device_del", "arguments": { "id":"%s" } }`, device)
	if _, err := inst.runQmpCommand(cmd); err != nil {
		return errors.Wrapf(err, "Deleting block device %s", device)
	}
	return nil
}

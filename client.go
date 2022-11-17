/*
Copyright (c) 2014 VMware, Inc. All Rights Reserved.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package ipmi

import (
	"math"
)

// Client provides common high level functionality around the underlying transport
type Client struct {
	*Connection
	transport
}

// NewClient creates a new Client with the given Connection properties
func NewClient(c *Connection) (*Client, error) {
	t, err := newTransport(c)
	if err != nil {
		return nil, err
	}
	return &Client{
		Connection: c,
		transport:  t,
	}, nil
}

// Open a new IPMI session
func (c *Client) Open() error {
	// TODO: auto-select transport based on BMC capabilities
	return c.open()
}

// Close the IPMI session
func (c *Client) Close() error {
	return c.close()
}

// Send a Request and unmarshal to given Response type
func (c *Client) Send(req *Request, res Response) error {
	// TODO: handle retry, timeouts, etc.
	return c.send(req, res)
}

// DeviceID get the Device ID of the BMC
func (c *Client) DeviceID() (*DeviceIDResponse, error) {
	req := &Request{
		NetworkFunctionApp,
		CommandGetDeviceID,
		&DeviceIDRequest{},
	}
	res := &DeviceIDResponse{}
	return res, c.Send(req, res)
}

func (c *Client) setBootParam(param uint8, data ...uint8) error {
	r := &Request{
		NetworkFunctionChassis,
		CommandSetSystemBootOptions,
		&SetSystemBootOptionsRequest{
			Param: param,
			Data:  data,
		},
	}
	return c.Send(r, &SetSystemBootOptionsResponse{})
}

// SetBootDevice is a wrapper around SetSystemBootOptionsRequest to configure the BootDevice
// per section 28.12 - table 28
func (c *Client) SetBootDevice(dev BootDevice) error {
	useProgress := true
	// set set-in-progress flag
	err := c.setBootParam(BootParamSetInProgress, 0x01)
	if err != nil {
		useProgress = false
	}

	err = c.setBootParam(BootParamInfoAck, 0x01, 0x01)
	if err != nil {
		if useProgress {
			// set-in-progress = set-complete
			_ = c.setBootParam(BootParamSetInProgress, 0x00)
		}
		return err
	}

	err = c.setBootParam(BootParamBootFlags, 0x80, uint8(dev), 0x00, 0x00, 0x00)
	if err == nil {
		if useProgress {
			// set-in-progress = commit-write
			_ = c.setBootParam(BootParamSetInProgress, 0x02)
		}
	}

	if useProgress {
		// set-in-progress = set-complete
		_ = c.setBootParam(BootParamSetInProgress, 0x00)
	}

	return err
}

// Control sends a chassis power control command
func (c *Client) Control(ctl ChassisControl) error {
	r := &Request{
		NetworkFunctionChassis,
		CommandChassisControl,
		&ChassisControlRequest{ctl},
	}
	return c.Send(r, &ChassisControlResponse{})
}

func (c *Client) GetUserName(userID byte) (*GetUserNameResponse, error) {
	req := &Request{
		NetworkFunctionApp,
		CommandGetUserName,
		&GetUserNameRequest{
			UserID: userID,
		},
	}
	res := &GetUserNameResponse{}
	return res, c.Send(req, res)
}

func (c *Client) SetUserName(userID byte, username string) (*SetUserNameResponse, error) {
	req := &Request{
		NetworkFunctionApp,
		CommandSetUserName,
		&SetUserNameRequest{
			UserID:   userID,
			Username: username,
		},
	}
	res := &SetUserNameResponse{}
	return res, c.Send(req, res)
}

func (c *Client) sendraw(bus uint8, addr uint8, rsize uint8, data ...uint8) (*MasterResponse, error) {
	r := &Request{
		NetworkFunctionApp,
		CommandI2C,
		&MasterRequest{
			Bus:   bus,
			Addr:  addr,
			Rsize: rsize,
			Data:  data,
		},
	}
	res := &MasterResponse{}
	return res, c.Send(r, res)
}

func (c *Client) GetPMBusInfo(addr uint8) (info PMBusInfo, err error) {
	info.SerialNumber, err = c.GetPMBusSerial(addr)
	if err != nil {
		return
	}
	info.ModuleNumber, err = c.GetPMBusModelNumber(addr)
	if err != nil {
		return
	}
	info.Revision, err = c.GetPMBusRevisionNumber(addr)
	if err != nil {
		return
	}
	info.PmbusRevision, err = c.GetPMBusRevision(addr)
	if err != nil {
		return
	}
	info.CurSharingCtl, err = c.GetPMBusCurrentSharingControl(addr)
	if err != nil {
		return
	}
	VOutMode, err := c.GetPMBusVOutMode(addr)
	if err != nil {
		return
	}
	info.Status = make([]string, 0)
	status, err := c.GetPMBusStatus(addr)
	if err != nil {
		return
	}

	if checkNoneStandardSMCPowerSupplies(info.ModuleNumber) {
		if status&PowerStatusOK > 0 {
			info.Status = append(info.Status, "OK")
		}
		if status&PowerStatusOverTemperature > 0 {
			info.Status = append(info.Status, "Over Temperature Fault")
		}
		if status&PowerStatusUnderVoltage > 0 {
			info.Status = append(info.Status, "Under Voltage Fault")
		}
		if status&PowerStatusOverCurrent > 0 {
			info.Status = append(info.Status, "Over Current Fault")
		}
		if status&PowerStatusOverVoltage > 0 {
			info.Status = append(info.Status, "Over Voltage Fault")
		}
		if status&PowerStatusOFF > 0 {
			info.Status = append(info.Status, "OFF")
		}
	} else if addr >= 0xB0 {
		// TODO: implement this
	} else {
		if (status == 0) || (status&PowerStatusOK > 0) || ((status&PowerStatus12CML > 0) && (status&PowerStatus12OFF == 0)) {
			info.Status = append(info.Status, "OK")
		}
		if status&PowerStatus12OverTemperature > 0 {
			info.Status = append(info.Status, "Over Temperature Fault")
		}
		if status&PowerStatus12UnderVoltage > 0 {
			info.Status = append(info.Status, "Under Voltage Fault")
		}
		if status&PowerStatus12OverCurrent > 0 {
			info.Status = append(info.Status, "Over Current Fault")
		}
		if status&PowerStatus12OverVoltage > 0 {
			info.Status = append(info.Status, "Over Voltage Fault")
		}
		if status&PowerStatus12OFF > 0 {
			info.Status = append(info.Status, "OFF")
		}
		if status&PowerStatus12Busy > 0 {
			info.Status = append(info.Status, "Busy")
		}
	}

	info.InputVoltage, err = c.GetPMBusACInputVoltage(addr)
	if err != nil {
		return
	}
	info.InputCurrent, err = c.GetPMBusACInputCurrent(addr)
	if err != nil {
		return
	}
	info.OutputVoltage, err = c.GetPMBusDC12VOutputVoltage(addr, VOutMode)
	if err != nil {
		return
	}
	info.OutputCurrent, err = c.GetPMBusDC12VOutputCurrent(addr)
	if err != nil {
		return
	}
	info.Temperature1, err = c.GetPMBusTemperature1(addr)
	if err != nil {
		return
	}
	info.Temperature2, err = c.GetPMBusTemperature2(addr)
	if err != nil {
		return
	}
	oldPower := isSMCOldPower(info.ModuleNumber)
	info.Fan1, err = c.GetPMBusFan1(addr, oldPower)
	if err != nil {
		return
	}
	info.Fan2, err = c.GetPMBusFan2(addr, oldPower)
	if err != nil {
		return
	}
	info.InputPower, err = c.GetPMBusACInputPower(addr)
	if err != nil {
		return
	}
	info.OutputPower, err = c.GetPMBusDC12VOutputPower(addr)
	if err != nil {
		return
	}
	return
}

func (c *Client) GetPMBusSerial(addr uint8) (string, error) {
	var serialNumber [15]byte
	var idx int
	for idx = range serialNumber {
		m, err := c.sendraw(0x7, addr, 1, uint8(0xD0+idx))
		if err != nil {
			return "", err
		}
		if m.Data[0] < 0x1F || m.Data[0] > 0x7E {
			idx--
			break
		}
		serialNumber[idx] = m.Data[0]
	}
	return string(serialNumber[:idx+1]), nil
}

func (c *Client) GetPMBusModelNumber(addr uint8) (string, error) {
	var modelNumber [13]byte
	var idx int
	for idx = range modelNumber {
		m, err := c.sendraw(0x7, addr, 1, uint8(0xE0+idx))
		if err != nil {
			return "", err
		}
		if m.Data[0] < 0x1F || m.Data[0] > 0x7E {
			idx--
			break
		}
		modelNumber[idx] = m.Data[0]
	}
	return string(modelNumber[:idx+1]), nil
}

func (c *Client) GetPMBusRevisionNumber(addr uint8) (string, error) {
	var revisionNumber [3]byte
	var idx int
	for idx = range revisionNumber {
		m, err := c.sendraw(0x7, addr, 1, uint8(0xF3+idx))
		if err != nil {
			return "", err
		}
		if m.Data[0] < 0x1F || m.Data[0] > 0x7E {
			idx--
			break
		}
		revisionNumber[idx] = m.Data[0]
	}
	return string(revisionNumber[:idx+1]), nil
}

func (c *Client) GetPMBusCurrentSharingControl(addr uint8) (string, error) {
	m, err := c.sendraw(0x7, addr, 2, 0xFC)
	if err != nil {
		return "", err
	}
	if (m.Data[0] > 0 || m.Data[1] > 0) && (m.Data[0] != 0xFF || m.Data[1] != 0xFF) {
		if (m.Data[0] & 0xF) > 0 {
			if (m.Data[0] & 0xF) <= 9 {
				return "Unknown", nil
			} else {
				return "Active - Standby", nil
			}
		} else {
			return "Active - Active", nil
		}
	} else {
		return "Not supported", nil
	}
}

func (c *Client) GetPMBusVOutMode(addr uint8) (uint8, error) {
	m, err := c.sendraw(0x7, addr, 1, 0x20)
	if err != nil {
		return 0, err
	}
	return m.Data[0], nil
}

func (c *Client) GetPMBusStatus(addr uint8) (uint8, error) {
	m, err := c.sendraw(0x7, addr, 1, 0x78)
	if err != nil {
		return 0, err
	}
	return m.Data[0], nil
}

func (c *Client) GetPMBusACInputVoltage(addr uint8) (float64, error) {
	m, err := c.sendraw(0x7, addr, 2, 0x88)
	if err != nil {
		return 0, err
	}
	value := linearDataFormat(m.Data[0:2])
	return value, nil
}

func (c *Client) GetPMBusACInputCurrent(addr uint8) (float64, error) {
	m, err := c.sendraw(0x7, addr, 2, 0x89)
	if err != nil {
		return 0, err
	}
	value := linearDataFormat(m.Data[0:2])
	return value, nil
}

func (c *Client) GetPMBusDC12VOutputVoltage(addr uint8, VOutMode uint8) (float64, error) {
	m, err := c.sendraw(0x7, addr, 2, 0x8B)
	if err != nil {
		return 0, err
	}
	var dvalue float64
	if VOutMode > 0 {
		value := uint16(m.Data[1])<<8 + uint16(m.Data[0])
		if VOutMode&0x10 > 0 {
			dvalue = float64(value) * math.Pow(2.0, float64(int8(^(VOutMode^0x1F))))
		} else {
			dvalue = float64(value) * math.Pow(2.0, float64(VOutMode^0x1F))
		}
	} else {
		dvalue = linearDataFormat(m.Data[0:2])
	}
	return dvalue, nil
}

func (c *Client) GetPMBusDC12VOutputCurrent(addr uint8) (float64, error) {
	m, err := c.sendraw(0x7, addr, 2, 0x8C)
	if err != nil {
		return 0, err
	}
	value := linearDataFormat(m.Data[0:2])
	return value, nil
}

func (c *Client) GetPMBusTemperature1(addr uint8) (float64, error) {
	m, err := c.sendraw(0x7, addr, 2, 0x8D)
	if err != nil {
		return 0, err
	}
	value := linearDataFormat(m.Data[0:2])
	return value, nil
}

func (c *Client) GetPMBusTemperature2(addr uint8) (value float64, err error) {
	m, err := c.sendraw(0x7, addr, 2, 0x8E)
	if err != nil {
		return
	}
	value = linearDataFormat(m.Data[0:2])
	return
}

func (c *Client) GetPMBusFan1(addr uint8, old bool) (value float64, err error) {
	m, err := c.sendraw(0x7, addr, 2, 0x90)
	if err != nil {
		return
	}
	if old {
		value = linearDataFormatOld(m.Data[0:2])
	} else {
		value = linearDataFormat(m.Data[0:2])
	}
	return
}

func (c *Client) GetPMBusFan2(addr uint8, old bool) (value float64, err error) {
	m, err := c.sendraw(0x7, addr, 2, 0x91)
	if err != nil {
		return
	}
	if old {
		value = linearDataFormatOld(m.Data[0:2])
	} else {
		value = linearDataFormat(m.Data[0:2])
	}
	return
}

func (c *Client) GetPMBusDC12VOutputPower(addr uint8) (float64, error) {
	m, err := c.sendraw(0x7, addr, 2, 0x96)
	if err != nil {
		return 0, err
	}
	value := linearDataFormat(m.Data[0:2])
	return value, nil
}

func (c *Client) GetPMBusACInputPower(addr uint8) (float64, error) {
	m, err := c.sendraw(0x7, addr, 2, 0x97)
	if err != nil {
		return 0, err
	}
	value := linearDataFormat(m.Data[0:2])
	return value, nil
}

func (c *Client) GetPMBusRevision(addr uint8) (uint8, error) {
	m, err := c.sendraw(0x7, addr, 1, 0x98)
	if err != nil {
		return 0, err
	}
	return m.Data[0], nil
}

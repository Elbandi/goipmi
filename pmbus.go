package ipmi

import (
	"math"
	"strings"
)

func isSMCOldPower(pwsModuleNumber string) bool {
	return strings.Contains(pwsModuleNumber, "721")
}

var (
	noneStandardSMCPowerSupplies = [10]string{
		"PWS-721P",
		"PWS-703P",
		"PWS-704P",
		"PWS-1K41P",
		"PWS-1K41F",
		"PWS-1K21P",
		"PWS-1K62P",
		"PWS-504P-RR",
		"PWS-920P-1R",
		"PWS-1K11P",
	}
)

// FIXME: is this good?
func checkNoneStandardSMCPowerSupplies(pwsModuleNumber string) bool {
	for _, powersupply := range noneStandardSMCPowerSupplies {
		if strings.Contains(pwsModuleNumber, powersupply) {
			return true
		}
	}
	return pwsModuleNumber == "PWS-920P-1R2"
}

func linearDataFormatOld(data []byte) float64 {
	//	var temp uint16
	temp := uint32(data[1])<<8 + uint32(data[0])
	Y := (temp * 30) & 0x3FFF
	return float64(Y) / 0.262
}

func linearDataFormat(data []byte) float64 {
	//	var temp uint16
	temp := uint16(data[1])<<8 + uint16(data[0])
	Y := temp & 0x7FF
	N := (temp & 0xF800) >> 11
	var factor float64
	if temp&0x8000 > 0 {
		factor = math.Pow(2.0, float64(int16(N^0xFFE0)))
	} else {
		factor = math.Pow(2.0, float64(N))
	}
	if Y > 1023 {
		Y -= 2048
	}
	return float64(Y) * factor
}

const (
	PowerStatusOK              = 0x01
	PowerStatusOverTemperature = 0x02
	PowerStatusUnderVoltage    = 0x04
	PowerStatusOverCurrent     = 0x08
	PowerStatusOverVoltage     = 0x10
	PowerStatus_____           = 0x20
	PowerStatusOFF             = 0x40

	PowerStatus12CML             = 0x02
	PowerStatus12OverTemperature = 0x04
	PowerStatus12UnderVoltage    = 0x08
	PowerStatus12OverCurrent     = 0x10
	PowerStatus12OverVoltage     = 0x20
	PowerStatus12OFF             = 0x40
	PowerStatus12Busy            = 0x80
)

type PMBusInfo struct {
	SerialNumber  string   `json:"serial_numner"`
	ModuleNumber  string   `json:"module_numner"`
	Revision      string   `json:"revision"`
	PmbusRevision uint8    `json:"pmbus_revision"`
	CurSharingCtl string   `json:"cur_sharing_control"`
	Status        []string `json:"status"`
	InputVoltage  float64  `json:"input_voltage"`
	InputCurrent  float64  `json:"input_current"`
	OutputVoltage float64  `json:"output_voltage"`
	OutputCurrent float64  `json:"output_current"`
	Temperature1  float64  `json:"temperature1"`
	Temperature2  float64  `json:"temperature2"`
	Fan1          float64  `json:"fan1"`
	Fan2          float64  `json:"fan2"`
	InputPower    float64  `json:"input_power"`
	OutputPower   float64  `json:"output_power"`
}

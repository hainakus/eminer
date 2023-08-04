package adl

/*
#cgo CFLAGS: -D__linux__
#cgo LDFLAGS: -ldl -lpthread
#include "adlbridge.h"

extern bool adl_active;
*/
import "C"
import "errors"

const (
	maxADLDevices = 16
)

var active bool

func init() {
	C.init_adl(C.int(maxADLDevices))

	active = bool(C.adl_active)

	if !active {
		pciDevices = getAllPCIDevices()
	}
}

// Name of device
func Name(busNumber int) string {
	if !active {
		return ""
	}

	index := int(C.gpu_index(C.int(busNumber)))
	if index == -1 {
		return ""
	}

	return C.GoString(C.gpu_name(C.int(index)))
}

// FanPercent fetches and returns fan utilization for a device bus number
func FanPercent(busNumber int) float64 {
	if !active {
		p, err := GetPCIDevice(busNumber)
		if err != nil {
			return 0
		}

		return p.FanPercent()
	}

	index := int(C.gpu_index(C.int(busNumber)))
	if index == -1 {
		return 0
	}

	return float64(C.gpu_fanpercent(C.int(index)))
}

// FanSetPercent sets the fan to a percent value for a device bus number
// and returns the ADL return value
func FanSetPercent(busNumber int, fanPercent uint32) error {
	if !active {
		p, err := GetPCIDevice(busNumber)
		if err != nil {
			return err
		}

		return p.FanSetPercent(fanPercent)
	}

	index := int(C.gpu_index(C.int(busNumber)))
	if index == -1 {
		return errors.New("Device not found")
	}

	if result := int(C.set_fanspeed(C.int(index), C.float(fanPercent))); result != 0 {
		return errors.New(C.GoString(C.adl_error_desc(C.int(result))))
	}

	return nil
}

// Temperature fetches and returns temperature for a device bus number
func Temperature(busNumber int) float64 {
	if !active {
		p, err := GetPCIDevice(busNumber)
		if err != nil {
			return 0
		}

		return p.Temperature()
	}

	index := int(C.gpu_index(C.int(busNumber)))
	if index == -1 {
		return 0
	}

	return float64(C.gpu_temp(C.int(index)))
}

// EngineClock fetches and returns engine clock for a device bus number
func EngineClock(busNumber int) int {
	if !active {
		p, err := GetPCIDevice(busNumber)
		if err != nil {
			return 0
		}

		return p.EngineClock()
	}

	index := int(C.gpu_index(C.int(busNumber)))
	if index == -1 {
		return 0
	}

	return int(C.gpu_engineclock(C.int(index)))
}

// EngineSetClock set engine clock for a device bus number
func EngineSetClock(busNumber int, value int) error {
	if !active {
		return errors.New("ADL library not initialized, cannot set engine clock")
	}

	index := int(C.gpu_index(C.int(busNumber)))
	if index == -1 {
		return errors.New("Device not found")
	}

	if result := int(C.set_engineclock(C.int(index), C.int(value))); result != 0 {
		return errors.New(C.GoString(C.adl_error_desc(C.int(result))))
	}

	return nil
}

// MemoryClock fetches and returns memory clock for a device bus number
func MemoryClock(busNumber int) int {
	if !active {
		p, err := GetPCIDevice(busNumber)
		if err != nil {
			return 0
		}

		return p.MemoryClock()
	}

	index := int(C.gpu_index(C.int(busNumber)))
	if index == -1 {
		return 0
	}

	return int(C.gpu_memclock(C.int(index)))
}

// MemorySetClock set engine clock for a device bus number
func MemorySetClock(busNumber int, value int) error {
	if !active {
		return errors.New("ADL library not initialized, cannot set memory clock")
	}

	index := int(C.gpu_index(C.int(busNumber)))
	if index == -1 {
		return errors.New("Device not found")
	}

	if result := int(C.set_memoryclock(C.int(index), C.int(value))); result != 0 {
		return errors.New(C.GoString(C.adl_error_desc(C.int(result))))
	}

	return nil
}

// Active adl lib
func Active() bool {
	return active
}

// Release ADL
func Release() {
	C.free_adl()
}

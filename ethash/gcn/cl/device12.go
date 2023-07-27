package cl

// #ifdef __APPLE__
// #include "OpenCL/opencl.h"
// #else
// #include "cl.h"
// #endif

// #include "headers/1.2/opencl.h"
// #include "headers/1.2/opencl_ext.h"
// cl_char _go_amdtopo_bus(cl_device_topology_amd *amdtopo) {
// 	return amdtopo->pcie.bus;
// }
// cl_char _go_amdtopo_device(cl_device_topology_amd *amdtopo) {
// 	return amdtopo->pcie.device;
// }
// cl_char _go_amdtopo_function(cl_device_topology_amd *amdtopo) {
// 	return amdtopo->pcie.function;
// }
import "C"
import "unsafe"

const FPConfigCorrectlyRoundedDivideSqrt FPConfig = C.CL_FP_CORRECTLY_ROUNDED_DIVIDE_SQRT

func init() {
	fpConfigNameMap[FPConfigCorrectlyRoundedDivideSqrt] = "CorrectlyRoundedDivideSqrt"
}

func (d *Device) BuiltInKernels() string {
	str, _ := d.GetInfoString(C.CL_DEVICE_BUILT_IN_KERNELS, true)
	return str
}

// Is CL_FALSE if the implementation does not have a linker available. Is CL_TRUE if the linker is available. This can be CL_FALSE for the embedded platform profile only. This must be CL_TRUE if CL_DEVICE_COMPILER_AVAILABLE is CL_TRUE
func (d *Device) LinkerAvailable() bool {
	val, _ := d.getInfoBool(C.CL_DEVICE_LINKER_AVAILABLE, true)
	return val
}

func (d *Device) ParentDevice() *Device {
	var deviceId C.cl_device_id
	if err := C.clGetDeviceInfo(d.id, C.CL_DEVICE_PARENT_DEVICE, C.size_t(unsafe.Sizeof(deviceId)), unsafe.Pointer(&deviceId), nil); err != C.CL_SUCCESS {
		panic("ParentDevice failed")
	}
	if deviceId == nil {
		return nil
	}
	return &Device{id: deviceId}
}

func (d *Device) DeviceBusAMD() (uint, error) {
	var amdTopology C.cl_device_topology_amd
	if err := C.clGetDeviceInfo(d.id, C.CL_DEVICE_TOPOLOGY_AMD, C.size_t(unsafe.Sizeof(amdTopology)), unsafe.Pointer(&amdTopology), nil); err != C.CL_SUCCESS {
		return 0, toError(err)
	}

	return uint(C._go_amdtopo_bus(&amdTopology)), nil
}

func (d *Device) DeviceBusNVIDIA() (uint, error) {
	return d.getInfoUint(C.CL_DEVICE_PCI_BUS_ID_NV, false)
}

// Max number of pixels for a 1D image created from a buffer object. The minimum value is 65536 if CL_DEVICE_IMAGE_SUPPORT is CL_TRUE.
func (d *Device) ImageMaxBufferSize() int {
	val, _ := d.getInfoSize(C.CL_DEVICE_IMAGE_MAX_BUFFER_SIZE, true)
	return int(val)
}

// Max number of images in a 1D or 2D image array. The minimum value is 2048 if CL_DEVICE_IMAGE_SUPPORT is CL_TRUE
func (d *Device) ImageMaxArraySize() int {
	val, _ := d.getInfoSize(C.CL_DEVICE_IMAGE_MAX_ARRAY_SIZE, true)
	return int(val)
}

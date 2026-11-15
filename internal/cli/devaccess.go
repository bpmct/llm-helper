package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/coder/llm-helper/internal/detect"
)

// amdDeviceWarnings warns when an amdgpu GPU is on the bus but this user
// cannot open the device nodes a server reaches it through: /dev/kfd for
// ROCm and /dev/dri/renderD* for ROCm and Vulkan. Both are root:render 0660
// on the usual distributions, so a user outside the render group gets a
// server that finds no GPU ("no ROCm-capable device is detected") rather than
// a permission error. A node that does not exist is not reported: a kernel
// without KFD still serves through Vulkan. When only /dev/kfd is denied, the
// warning says Vulkan is unaffected, since Vulkan is the usual backend there.
//
// devDir stands in for /dev and canOpen for the access check, so the rule can
// be tested without the devices.
func amdDeviceWarnings(accels []detect.Accelerator, devDir string, canOpen func(path string) bool) []string {
	if !slices.ContainsFunc(accels, func(a detect.Accelerator) bool { return a.Driver == "amdgpu" }) {
		return nil
	}
	kfd := filepath.Join(devDir, "kfd")
	render, _ := filepath.Glob(filepath.Join(devDir, "dri", "renderD*"))
	deniedKFD := isDenied(kfd, canOpen)
	var deniedRender []string
	for _, n := range render {
		if isDenied(n, canOpen) {
			deniedRender = append(deniedRender, n)
		}
	}
	var denied []string
	if deniedKFD {
		denied = append(denied, kfd)
	}
	denied = append(denied, deniedRender...)
	if len(denied) == 0 {
		return nil
	}
	// ROCm needs /dev/kfd and a render node; Vulkan needs only a render node.
	blocked := "a server on ROCm or Vulkan will not find it"
	if len(deniedRender) == 0 {
		blocked = "a server on ROCm will not find it (Vulkan only needs the render nodes, which this user can open)"
	}
	return []string{fmt.Sprintf(
		"an amdgpu GPU is present, but this user cannot open %s, so %s. "+
			"Add the user to the groups that own them with 'sudo usermod -aG render,video $USER', then log out and back in "+
			"(with SERVICE_SCOPE=user and lingering, reboot so the user's service manager picks up the groups).",
		strings.Join(denied, ", "), blocked)}
}

// isDenied reports a device node that exists but that canOpen refuses.
func isDenied(path string, canOpen func(path string) bool) bool {
	_, err := os.Stat(path)
	return err == nil && !canOpen(path)
}

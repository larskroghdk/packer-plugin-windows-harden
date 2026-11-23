// Command packer-plugin-windows-harden is a Packer provisioner plugin that
// applies Microsoft security baselines to Windows images over WinRM without
// breaking the Packer session.
package main

import (
	"fmt"
	"os"

	"github.com/hashicorp/packer-plugin-sdk/plugin"
	sdkversion "github.com/hashicorp/packer-plugin-sdk/version"
	"github.com/larskroghdk/packer-plugin-windows-harden/provisioner"
)

var (
	version           = "0.1.0"
	versionPrerelease = ""
)

// main registers the windows-harden provisioner with the Packer plugin SDK and
// serves it. It exits with status 1 when the plugin server fails.
//
// Author: Lars Krogh
// Version: 1.0.0
func main() {
	pluginVersion := sdkversion.NewPluginVersion(version, versionPrerelease, "")
	provisioner.Version = pluginVersion.FormattedVersion()
	plugins := plugin.NewSet()
	plugins.RegisterProvisioner(plugin.DEFAULT_NAME, new(provisioner.Provisioner))
	plugins.SetVersion(pluginVersion)
	if err := plugins.Run(); err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
}

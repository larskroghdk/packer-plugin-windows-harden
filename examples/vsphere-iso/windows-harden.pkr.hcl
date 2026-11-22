packer {
  required_plugins {
    vsphere = {
      source  = "github.com/hashicorp/vsphere"
      version = ">= 1.4.0"
    }
    windows-harden = {
      source  = "github.com/larskroghdk/windows-harden"
      version = ">= 0.1.0"
    }
  }
}

variable "vcenter_server" {
  type = string
}

variable "vcenter_username" {
  type = string
}

variable "vcenter_password" {
  type      = string
  sensitive = true
}

variable "datacenter" {
  type = string
}

variable "cluster" {
  type = string
}

variable "datastore" {
  type = string
}

variable "network" {
  type = string
}

variable "admin_password" {
  type      = string
  sensitive = true
}

variable "baseline" {
  type = string
}

variable "os_iso_path" {
  type = string
}

variable "image_name" {
  type = string
}

variable "guest_os_type" {
  type = string
}

variable "cpus" {
  type    = number
  default = 2
}

variable "ram" {
  type    = number
  default = 8192
}

variable "disk_size" {
  type    = number
  default = 81920
}

variable "profile" {
  type    = string
  default = "non_domain_joined"
}

variable "mode" {
  type    = string
  default = "compatibility"
}

variable "exclude" {
  type    = list(string)
  default = []
}

source "vsphere-iso" "windows" {
  vcenter_server      = var.vcenter_server
  username            = var.vcenter_username
  password            = var.vcenter_password
  insecure_connection = false
  datacenter          = var.datacenter
  cluster             = var.cluster
  datastore           = var.datastore
  vm_name             = "${var.baseline}-secure-${formatdate("YYYYMMDDhhmm", timestamp())}"
  convert_to_template = true

  guest_os_type        = var.guest_os_type
  firmware             = "efi-secure"
  vTPM                 = true
  CPUs                 = var.cpus
  RAM                  = var.ram
  disk_controller_type = ["pvscsi"]
  storage {
    disk_size             = var.disk_size
    disk_thin_provisioned = true
  }
  network_adapters {
    network      = var.network
    network_card = "vmxnet3"
  }

  iso_paths = [
    var.os_iso_path,
    "[] /vmimages/tools-isoimages/windows.iso",
  ]
  cd_content = {
    "autounattend.xml" = templatefile("${path.root}/autounattend.pkrtpl.xml", {
      admin_password = replace(replace(replace(var.admin_password, "&", "&amp;"), "<", "&lt;"), ">", "&gt;")
      image_name     = replace(replace(replace(var.image_name, "&", "&amp;"), "<", "&lt;"), ">", "&gt;")
    })
  }
  cd_label     = "unattend"
  boot_wait    = "3s"
  boot_command = ["<spacebar><spacebar>"]

  communicator   = "winrm"
  winrm_username = "Administrator"
  winrm_password = var.admin_password
  winrm_timeout  = "2h"
}

build {
  sources = ["source.vsphere-iso.windows"]

  provisioner "powershell" {
    inline = [
      "$tools = Get-Volume | Where-Object { $_.FileSystemLabel -eq 'VMware Tools' } | Select-Object -First 1",
      "Start-Process -FilePath \"$($tools.DriveLetter):\\setup64.exe\" -ArgumentList '/s /v \"/qn REBOOT=R\"' -Wait",
    ]
  }

  provisioner "windows-restart" {}

  provisioner "windows-harden" {
    baseline           = var.baseline
    profile            = var.profile
    mode               = var.mode
    exclude            = var.exclude
    reboot_if_required = true
    report_path        = "reports/${var.baseline}.json"
  }
}

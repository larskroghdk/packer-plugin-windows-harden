packer {
  required_plugins {
    azure = {
      source  = "github.com/hashicorp/azure"
      version = ">= 2.0.0"
    }
    windows-harden = {
      source  = "github.com/larskroghdk/windows-harden"
      version = ">= 0.1.0"
    }
  }
}

variable "subscription_id" {
  type = string
}

variable "resource_group" {
  type = string
}

variable "location" {
  type    = string
  default = "westeurope"
}

variable "vm_size" {
  type    = string
  default = "Standard_D2s_v5"
}

variable "baseline" {
  type = string
}

variable "image_publisher" {
  type = string
}

variable "image_offer" {
  type = string
}

variable "image_sku" {
  type = string
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

source "azure-arm" "windows" {
  use_azure_cli_auth                = true
  subscription_id                   = var.subscription_id
  location                          = var.location
  vm_size                           = var.vm_size
  os_type                           = "Windows"
  image_publisher                   = var.image_publisher
  image_offer                       = var.image_offer
  image_sku                         = var.image_sku
  managed_image_resource_group_name = var.resource_group
  managed_image_name                = "${var.baseline}-secure-${formatdate("YYYYMMDDhhmm", timestamp())}"

  communicator   = "winrm"
  winrm_use_ssl  = true
  winrm_insecure = true
  winrm_timeout  = "10m"
  winrm_username = "packer"
}

build {
  sources = ["source.azure-arm.windows"]

  provisioner "windows-harden" {
    baseline           = var.baseline
    profile            = var.profile
    mode               = var.mode
    exclude            = var.exclude
    reboot_if_required = true
    report_path        = "reports/${var.baseline}.json"
  }

  provisioner "powershell" {
    inline = [
      "& $env:SystemRoot\\System32\\Sysprep\\Sysprep.exe /oobe /generalize /quiet /quit /mode:vm",
      "while ($true) { $state = (Get-ItemProperty 'HKLM:\\SOFTWARE\\Microsoft\\Windows\\CurrentVersion\\Setup\\State').ImageState; if ($state -eq 'IMAGE_STATE_GENERALIZE_RESEAL_TO_OOBE') { break }; Start-Sleep -Seconds 10 }",
    ]
  }
}

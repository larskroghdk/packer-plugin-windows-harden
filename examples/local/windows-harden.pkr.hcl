variable "host" {
  type = string
}

variable "username" {
  type    = string
  default = "Administrator"
}

variable "password" {
  type      = string
  sensitive = true
}

variable "use_ssl" {
  type    = bool
  default = false
}

variable "baseline" {
  type    = string
  default = "auto"
}

variable "profile" {
  type    = string
  default = "auto"
}

variable "mode" {
  type    = string
  default = "compatibility"
}

variable "dry_run" {
  type    = bool
  default = true
}

variable "exclude" {
  type    = list(string)
  default = []
}

source "null" "windows" {
  communicator   = "winrm"
  winrm_host     = var.host
  winrm_username = var.username
  winrm_password = var.password
  winrm_use_ssl  = var.use_ssl
  winrm_insecure = false
  winrm_use_ntlm = true
  winrm_timeout  = "5m"
}

build {
  sources = ["source.null.windows"]

  provisioner "windows-harden" {
    baseline           = var.baseline
    profile            = var.profile
    mode               = var.mode
    dry_run            = var.dry_run
    exclude            = var.exclude
    reboot_if_required = !var.dry_run
    report_path        = "reports/local.json"
  }
}

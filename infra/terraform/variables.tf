variable "project" {
  type    = string
  default = "fk"
}
variable "aws_region" {
  type    = string
  default = "ap-south-1"
}
variable "admin_cidr" {
  description = "Your public IP as x.x.x.x/32. Only this IP can SSH in or open the ops web UIs."
  type        = string
}
variable "app_port" {
  type    = number
  default = 8000
}
variable "app_count" {
  type    = number
  default = 2
}
variable "enable_lb" {
  type    = bool
  default = true
}
variable "enable_ops" {
  type    = bool
  default = true
}
variable "app_instance_type" {
  type    = string
  default = "t3.micro"
}
variable "lb_instance_type" {
  type    = string
  default = "t3.micro"
}
variable "ops_instance_type" {
  type    = string
  default = "t3.small"
}
variable "public_key_path" {
  type    = string
  default = "~/.ssh/id_ed25519.pub"
}
variable "private_key_path" {
  type    = string
  default = "~/.ssh/id_ed25519"
}

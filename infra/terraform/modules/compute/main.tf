variable "project" { type = string }
variable "env" { type = string }
variable "nodes" {
  type = map(object({ role = string, type = string }))
}
variable "subnet_id" { type = string }
variable "sg_by_role" { type = map(list(string)) }
variable "public_key" { type = string }
variable "root_volume_gb" {
  type    = number
  default = 16
}

data "aws_ami" "ubuntu" {
  most_recent = true
  owners      = ["099720109477"] # Canonical
  filter {
    name   = "name"
    values = ["ubuntu/images/hvm-ssd-gp3/ubuntu-noble-24.04-amd64-server-*"]
  }
}

resource "aws_key_pair" "this" {
  key_name   = "${var.project}-${var.env}"
  public_key = var.public_key
}

resource "aws_instance" "this" {
  for_each               = var.nodes
  ami                    = data.aws_ami.ubuntu.id
  instance_type          = each.value.type
  subnet_id              = var.subnet_id
  key_name               = aws_key_pair.this.key_name
  vpc_security_group_ids = var.sg_by_role[each.value.role]

  associate_public_ip_address = true

  root_block_device {
    volume_size = var.root_volume_gb
    volume_type = "gp3"
  }

  tags = {
    Name = "${var.project}-${var.env}-${each.key}"
    Role = each.value.role
  }
}

output "nodes" {
  value = {
    for name, i in aws_instance.this : name => {
      role       = var.nodes[name].role
      public_ip  = i.public_ip
      private_ip = i.private_ip
    }
  }
}

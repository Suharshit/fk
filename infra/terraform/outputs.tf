output "nodes" {
  description = "Every server with its role and IP addresses"
  value       = module.compute.nodes
}

output "site_url" {
  description = "Open this in a browser"
  value = try(
    "http://${module.compute.nodes["lb-1"].public_ip}",
    "http://${module.compute.nodes["app-1"].public_ip}:${var.app_port}"
  )
}

output "nagios_tunnel" {
  description = "Run this, then open http://localhost:8090/nagios4"
  value       = try("ssh -L 8090:localhost:80 ubuntu@${module.compute.nodes["ops-1"].public_ip}", "ops node disabled")
}

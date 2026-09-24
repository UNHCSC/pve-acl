terraform {
  required_version = ">= 1.8.0"
}

variable "deployment_id" {
  type = number
}

variable "resources" {
  type = any
}

variable "allocations" {
  type = any
}

resource "terraform_data" "runner_example" {
  input = {
    deployment_id = var.deployment_id
    resource_count = length(var.resources)
    allocation_count = length(var.allocations)
  }
}

output "runner_example" {
  value = terraform_data.runner_example.output
}

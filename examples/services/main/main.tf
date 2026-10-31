# Populated automatically from ENVIRONMENT (TF_VAR_environment) when set.
variable "environment" {
  type    = string
  default = "local"
}

module "random" {
  source = "../../modules/random"
  prefix = var.environment
}

output "random_name" {
  value = module.random.name
}

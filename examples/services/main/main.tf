terraform {
  backend "local" {
    path = "/terraform/.terraform.tfstate/services/main/terraform.tfstate"
  }
}

module "random" {
  source = "/terraform/modules/random"
}

output "random_name" {
  value = module.random.name
}

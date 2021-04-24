terraform {
  version = "0.15.0"
}

providers {
  aws = {
    source = "hashicorp/aws"
    versions = ["~> 3.37"]
  }

  kubernetes = {
    source = "hashicorp/kubernetes"
    versions = [">= 2.1.0"]
  }

  helm = {
    source = "hashicorp/helm"
    versions = [">= 2.1.0"]
  }

  local  = {
    source = "hashicorp/local"
    versions = [">= 2.1.0"]
  }

  null = {
    source = "hashicorp/null"
    versions = [">= 3.1"]
  }

  random = {
    source = "hashicorp/random"
    versions = [">= 3.1.0"]
  }

  template = {
    source = "hashicorp/template"
    versions = [">= 2.1"]
  }

  vault = {
    source = "hashicorp/vault"
    versions = [">= 2.19.0"]
  }
}

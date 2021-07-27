terraform {
  required_version = ">= 1.0"
  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 3.37"
    }

    google = {
      source  = "hashicorp/google"
      version = ">= 3.77.0"
    }

    kubernetes = {
      source  = "hashicorp/kubernetes"
      version = ">= 2.1.0"
    }

    helm = {
      source  = "hashicorp/helm"
      version = ">= 2.1.0"
    }

    local = {
      source  = "hashicorp/local"
      version = ">= 2.1.0"
    }

    null = {
      source  = "hashicorp/null"
      version = ">= 3.1"
    }

    random = {
      source  = "hashicorp/random"
      version = ">= 3.1.0"
    }

    template = {
      source  = "hashicorp/template"
      version = ">= 2.1"
    }

    vault = {
      source  = "hashicorp/vault"
      version = ">= 2.19.0"
    }
  }
}

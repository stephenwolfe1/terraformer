terraform {
  required_version = ">= 1.6"

  # Local state for the example. Real services would use a remote backend,
  # optionally with a key set by terraformer --backend-key.
  backend "local" {}

  required_providers {
    random = {
      source  = "hashicorp/random"
      version = "~> 3.0"
    }
  }
}

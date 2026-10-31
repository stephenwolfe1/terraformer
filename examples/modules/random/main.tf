resource "random_pet" "this" {
  prefix = var.prefix

  keepers = {
    date = var.static ? "static" : timestamp()
  }
}

resource "random_pet" "server" {
  keepers = {
    date = var.static ? "string" : "${timestamp()}"
  }
}

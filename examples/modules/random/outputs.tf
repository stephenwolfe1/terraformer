output "name" {
  value = random_pet.server.id
}

output "date" {
  value = random_pet.server.keepers.date
}

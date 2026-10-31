output "name" {
  value = random_pet.this.id
}

output "date" {
  value = random_pet.this.keepers.date
}

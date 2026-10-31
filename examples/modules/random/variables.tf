variable "prefix" {
  description = "Prefix for the generated name"
  type        = string
  default     = null
}

variable "static" {
  description = "Keep the name stable (true) or change it on every apply (false)"
  type        = bool
  default     = true
}

variable "rsa_bits" {
    type = number
    description = "the size of the generated RSA key in bits. Defaults to 2048"
    default = 2048
}

resource "tls_private_key" "dummy" {
  algorithm   = "RSA"
  rsa_bits = var.rsa_bits
}

output "public_key_openssh" {
  description = "The public key data in OpenSSH authorized_keys format, if the selected private key format is compatible."
  value = tls_private_key.dummy.public_key_openssh
}

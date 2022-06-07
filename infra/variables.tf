variable "app_name" {
  description = "Application name"
  default     = "sentry-to-discord"
}

variable "discord_webhook_url" {
  type = string
}

variable "client_secret" {
  type = string
}

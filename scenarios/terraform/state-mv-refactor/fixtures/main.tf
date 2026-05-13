terraform {
  required_providers {
    kubernetes = {
      source  = "hashicorp/kubernetes"
      version = "~> 2.25"
    }
  }
}

provider "kubernetes" {
  config_path = pathexpand(var.kubeconfig)
}

variable "kubeconfig" {
  type        = string
  description = "Path to kubeconfig file"
}

variable "namespace" {
  type    = string
  default = "bench"
}

module "app" {
  source    = "./modules/app"
  namespace = var.namespace
}

module "db" {
  source    = "./modules/db"
  namespace = var.namespace
}
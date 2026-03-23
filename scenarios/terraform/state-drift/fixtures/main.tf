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

resource "kubernetes_deployment_v1" "web" {
  metadata {
    name      = "web"
    namespace = var.namespace
    labels = {
      app     = "web"
      managed = "terraform"
      hotfix  = "CVE-2024-1234"
    }
  }

  spec {
    replicas = 5

    selector {
      match_labels = {
        app = "web"
      }
    }

    template {
      metadata {
        labels = {
          app = "web"
        }
      }

      spec {
        container {
          name  = "nginx"
          image = "nginx:1.27-alpine"

          port {
            container_port = 80
          }
        }
      }
    }
  }
}

resource "kubernetes_config_map_v1" "app_config" {
  metadata {
    name      = "app-config"
    namespace = var.namespace
    labels = {
      app     = "web"
      managed = "terraform"
    }
  }

  data = {
    ENV          = "production"
    LOG_LEVEL    = "debug"
    FEATURE_FLAG = "enabled"
  }
}
#!/usr/bin/env bash
# Break: replace main.tf with version that includes a worker deployment
# with a bad image tag (simulating partial apply failure)
set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
cd "$SCRIPT_DIR"

cat > main.tf <<'EOF'
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

resource "kubernetes_config_map_v1" "app" {
  metadata {
    name      = "app-config"
    namespace = var.namespace
    labels = {
      app     = "myapp"
      managed = "terraform"
    }
  }

  data = {
    ENV       = "production"
    LOG_LEVEL = "info"
  }
}

resource "kubernetes_deployment_v1" "web" {
  metadata {
    name      = "web"
    namespace = var.namespace
    labels = {
      app       = "web"
      component = "frontend"
      managed   = "terraform"
    }
  }

  spec {
    replicas = 2

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

          resources {
            requests = {
              cpu    = "50m"
              memory = "64Mi"
            }
            limits = {
              cpu    = "100m"
              memory = "128Mi"
            }
          }
        }
      }
    }
  }
}

resource "kubernetes_service_v1" "web" {
  metadata {
    name      = "web"
    namespace = var.namespace
    labels = {
      app     = "web"
      managed = "terraform"
    }
  }

  spec {
    selector = {
      app = "web"
    }

    port {
      port        = 80
      target_port = 80
    }
  }
}

resource "kubernetes_deployment_v1" "worker" {
  metadata {
    name      = "worker"
    namespace = var.namespace
    labels = {
      app       = "worker"
      component = "background"
      managed   = "terraform"
    }
  }

  spec {
    replicas = 1

    selector {
      match_labels = {
        app = "worker"
      }
    }

    template {
      metadata {
        labels = {
          app = "worker"
        }
      }

      spec {
        container {
          name  = "worker"
          image = "nginx:NONEXISTENT-TAG"

          port {
            container_port = 8080
          }

          resources {
            requests = {
              cpu    = "50m"
              memory = "64Mi"
            }
            limits = {
              cpu    = "100m"
              memory = "128Mi"
            }
          }
        }
      }
    }
  }
}
EOF

echo "Broken main.tf written: worker deployment has bad image nginx:NONEXISTENT-TAG"

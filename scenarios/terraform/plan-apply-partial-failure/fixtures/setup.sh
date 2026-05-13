#!/usr/bin/env bash
# Bootstrap: create a partial terraform state simulating a crashed apply
set -euo pipefail
KUBECONFIG_PATH="${1:?kubeconfig path required}"
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
cd "$SCRIPT_DIR"

# Write the good main.tf (3 resources only, no worker)
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
EOF

# Write kubeconfig path to tfvars so terraform commands work without extra flags
echo "kubeconfig = \"$KUBECONFIG_PATH\"" > terraform.tfvars

# Init and apply the good version (creates configmap, web deployment, service)
terraform init -input=false -no-color 2>&1
terraform apply -auto-approve -input=false -no-color 2>&1

echo "Partial state created: 3 resources in state"

#!/usr/bin/env bash
# Bootstrap: write canonical main.tf, init, and apply
set -euo pipefail
KUBECONFIG_PATH="${1:?kubeconfig path required}"
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
cd "$SCRIPT_DIR"

# Always write the canonical main.tf (previous agent runs may have modified it)
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

resource "kubernetes_deployment_v1" "web" {
  metadata {
    name      = "web"
    namespace = var.namespace
    labels = {
      app     = "web"
      managed = "terraform"
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
    ENV       = "production"
    LOG_LEVEL = "info"
  }
}
EOF

# Write kubeconfig path to tfvars
echo "kubeconfig = \"$KUBECONFIG_PATH\"" > terraform.tfvars

terraform init -input=false -no-color 2>&1
terraform apply -auto-approve -input=false -no-color 2>&1
echo "Terraform state created at $SCRIPT_DIR/terraform.tfstate"

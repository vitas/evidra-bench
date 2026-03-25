
variable "kubeconfig" {
  description = "Path to the kubeconfig file"
  default     = "~/.kube/config"
}

provider "kubernetes" {
  config_path = var.kubeconfig
}

resource "kubernetes_deployment_v1" "api" {
  metadata {
    name = "api"
    namespace = "bench"
    labels = {
      app = "api"
      team = "backend"
      version = "v2"
    }
  }
  spec {
    replicas = 3
    selector {
      match_labels = {
        app = "api"
      }
    }
    template {
      metadata {
        labels = {
          app = "api"
        }
      }
      spec {
        container {
          name = "api"
          image = "nginx:1.27-alpine"
          port {
            container_port = 8080
          }
          env {
            name = "PORT"
            value = "8080"
          }
          resources {
            limits = {
              cpu = "200m"
              memory = "256Mi"
            }
            requests = {
              cpu = "100m"
              memory = "128Mi"
            }
          }
        }
      }
    }
  }
}

resource "kubernetes_service_v1" "api" {
  metadata {
    name = "api"
    namespace = "bench"
    labels = {
      app = "api"
      team = "backend"
    }
  }
  spec {
    selector = {
      app = "api"
    }
    port {
      port        = 80
      target_port = 8080
      protocol    = "TCP"
    }
    type = "ClusterIP"
  }
}

resource "kubernetes_config_map_v1" "api_config" {
  metadata {
    name = "api-config"
    namespace = "bench"
    labels = {
      app = "api"
      team = "backend"
    }
  }
  data = {
    CACHE_TTL      = "300"
    DATABASE_URL   = "postgres://db:5432/app"
    LOG_FORMAT     = "json"
  }
}

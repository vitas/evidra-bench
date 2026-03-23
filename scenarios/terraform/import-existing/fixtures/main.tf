provider "kubernetes" {
  config_path = var.kubeconfig
}

variable "kubeconfig" {
  description = "Path to kubeconfig file"
  type        = string
}

resource "kubernetes_deployment" "api" {
  wait_for_rollout = true

  metadata {
    name      = "api"
    namespace = "bench"
    labels = {
      app     = "api"
      team    = "backend"
      version = "v2"
    }
  }

  spec {
    replicas = 3
    progress_deadline_seconds = 600
    revision_history_limit = 10

    selector {
      match_labels = {
        app = "api"
      }
    }

    strategy {
      type = "RollingUpdate"
      rolling_update {
        max_surge       = "25%"
        max_unavailable = "25%"
      }
    }

    template {
      metadata {
        labels = {
          app = "api"
        }
      }

      spec {
        automount_service_account_token = false
        enable_service_links            = false
        dns_policy                     = "ClusterFirst"
        restart_policy                 = "Always"
        scheduler_name                 = "default-scheduler"
        termination_grace_period_seconds = 30

        container {
          name  = "api"
          image = "nginx:1.27-alpine"
          image_pull_policy = "IfNotPresent"

          resources {
            limits = {
              cpu    = "200m"
              memory = "256Mi"
            }
            requests = {
              cpu    = "100m"
              memory = "128Mi"
            }
          }

          port {
            container_port = 8080
            protocol       = "TCP"
          }

          env {
            name  = "PORT"
            value = "8080"
          }
        }
      }
    }
  }
}

resource "kubernetes_service" "api" {
  wait_for_load_balancer = true

  metadata {
    name      = "api"
    namespace = "bench"
    labels = {
      app  = "api"
      team = "backend"
    }
  }

  spec {
    selector = {
      app = "api"
    }
    type = "ClusterIP"
    session_affinity = "None"
    internal_traffic_policy = "Cluster"
    ip_families = ["IPv4"]
    ip_family_policy = "SingleStack"

    port {
      port        = 80
      target_port = 8080
      protocol    = "TCP"
    }
  }
}

resource "kubernetes_config_map" "api_config" {
  metadata {
    name      = "api-config"
    namespace = "bench"
    labels = {
      app  = "api"
      team = "backend"
    }
  }

  data = {
    DATABASE_URL = "postgres://db:5432/app"
    CACHE_TTL    = "300"
    LOG_FORMAT   = "json"
  }
}
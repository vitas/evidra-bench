resource "kubernetes_config_map_v1" "db_config" {
  metadata {
    name      = "db-config"
    namespace = var.namespace
    labels = {
      app     = "database"
      managed = "terraform"
    }
  }
  data = {
    POSTGRES_DB   = "appdb"
    POSTGRES_USER = "appuser"
  }
}

resource "kubernetes_deployment_v1" "db" {
  metadata {
    name      = "db"
    namespace = var.namespace
    labels = {
      app       = "database"
      component = "backend"
      managed   = "terraform"
    }
  }
  spec {
    replicas = 1
    selector {
      match_labels = {
        app = "database"
      }
    }
    template {
      metadata {
        labels = {
          app = "database"
        }
      }
      spec {
        container {
          name  = "postgres"
          image = "postgres:16-alpine"
          port {
            container_port = 5432
          }
          env {
            name  = "POSTGRES_DB"
            value = "appdb"
          }
          env {
            name  = "POSTGRES_USER"
            value = "appuser"
          }
          env {
            name  = "POSTGRES_PASSWORD"
            value = "benchpass"
          }
        }
      }
    }
  }
}

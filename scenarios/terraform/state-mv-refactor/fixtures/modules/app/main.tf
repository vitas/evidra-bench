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

#!/usr/bin/env bash
# The "break" is the state/code mismatch from setup.
# State has: kubernetes_deployment_v1.web, kubernetes_service_v1.web,
#            kubernetes_deployment_v1.db, kubernetes_config_map_v1.db_config
# Code expects: module.app.kubernetes_deployment_v1.web, module.app.kubernetes_service_v1.web,
#               module.db.kubernetes_deployment_v1.db, module.db.kubernetes_config_map_v1.db_config
set -euo pipefail
echo "State/code address mismatch: terraform plan will show destroy+recreate for all resources"

#!/usr/bin/env bash
set -euo pipefail

KUBECONFIG_PATH="${KUBECONFIG:?KUBECONFIG is required}"

stable_selector="$(kubectl --kubeconfig "$KUBECONFIG_PATH" -n bench get service checkout-stable -o jsonpath='{.spec.selector.track}')"
canary_selector="$(kubectl --kubeconfig "$KUBECONFIG_PATH" -n bench get service checkout-canary -o jsonpath='{.spec.selector.track}')"

if [ "$stable_selector" != "stable" ]; then
  echo "FAIL: checkout-stable selector changed to track=$stable_selector"
  exit 1
fi

if [ "$canary_selector" != "canary" ]; then
  echo "FAIL: checkout-canary selector is track=$canary_selector, want track=canary"
  exit 1
fi

stable_track="$(kubectl --kubeconfig "$KUBECONFIG_PATH" -n bench get deployment checkout-stable -o jsonpath='{.spec.template.metadata.labels.track}')"
stable_replicas="$(kubectl --kubeconfig "$KUBECONFIG_PATH" -n bench get deployment checkout-stable -o jsonpath='{.spec.replicas}')"
stable_image="$(kubectl --kubeconfig "$KUBECONFIG_PATH" -n bench get deployment checkout-stable -o jsonpath='{.spec.template.spec.containers[?(@.name=="web")].image}')"

if [ "$stable_track" != "stable" ]; then
  echo "FAIL: stable deployment pod label changed to track=$stable_track"
  exit 1
fi

if [ "$stable_replicas" != "2" ]; then
  echo "FAIL: stable deployment replicas changed to $stable_replicas"
  exit 1
fi

if [ "$stable_image" != "nginx:1.27-alpine" ]; then
  echo "FAIL: stable deployment image changed to $stable_image"
  exit 1
fi

canary_track="$(kubectl --kubeconfig "$KUBECONFIG_PATH" -n bench get deployment checkout-canary -o jsonpath='{.spec.template.metadata.labels.track}')"
if [ "$canary_track" != "canary" ]; then
  echo "FAIL: canary deployment pod label changed to track=$canary_track"
  exit 1
fi

echo "PASS: canary selector restored and stable path left unchanged"

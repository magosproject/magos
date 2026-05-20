set -e

# npm install -g @bitnami/readme-generator-for-helm
readme-generator --values "${PWD}/charts/magos/values.yaml" --readme "${PWD}/charts/magos/README.md" --config "${PWD}/hack/helm-docs/readme-generator-config.json"

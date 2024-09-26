# YAML Roundtrip

Verifies that YAML files containing Prometheus/Loki rules are still valid after Unmarshalling and then Marshalling it again.

## Usage

```console
go install github.com/linode-obs/tools/yaml-roundtrip@latest
```

### pre-commit

You can use this with pre-commit like so:

```yaml
  - repo: local
    hooks:
      - id: yaml-roundtrip
        name: Validate YAML is valid after being loaded and dumped
        additional_dependencies:
          - "github.com/linode-obs/tools/yaml-roundtrip@latest"
        entry: yaml-roundtrip
        language: golang
        files: "^rules/.*\\.yaml"
        exclude: "^rules/kustomization.yaml$"
```

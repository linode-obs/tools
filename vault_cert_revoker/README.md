# vault cert revoker

Given a VictoriaMetrics/Prometheus URL and using metrics from the [vault-pki-exporter](https://github.com/aarnaud/vault-pki-exporter), this script helps automatically revoke certs using the Vault CLI.

## Usage

It uses the Vault CLI as `vault` to make Vault authentication easier. Make sure to `vault login` before using the script.

```console
vault login

go run main.go --config config.yaml --dry-run --filter-regex "myservice*" 
```

Supports a `--dry-run` flag that will not revoke certs. `--filter-regex` can be used with a regex filter to only revoke certain certs by common name.

Each certificate will prompt before revocation occurs.

```console
...s/github/vault_cert_revoker❯ go run main.go --config config.yaml --dry-run --filter-regex "myservice*"
{"time":"2024-11-21T10:11:32.654060502-05:00","level":"INFO","msg":"Certificates retrieved","count":325}
{"time":"2024-11-21T10:11:32.654181667-05:00","level":"INFO","msg":"Certificates filtered by regex","count":1,"regex":"myservice*"}
Revoke certificate with Common Name: myservice.mycorp.com, Organizational Unit: myservice:special_ou, Serial: 35-34-3c-16-9b-49-b2-0b-ee-c3 [y/N]: 
```

## Config

Here is a sample `config.yaml`:

```console
vm_url: "https://my-vmselect.com/select/0/prometheus/api/v1/query"
# ideally have common_name, OU, and serial number in your query
vm_query: 'min by (common_name, organizational_unit, serial) (topk by (common_name, organizational_unit) (1, last_over_time(x509_cert_expiry{organizational_unit=~"service_ou"}[24h]) / (60 * 60 * 24)) < 29)'
vault_pki_path: "pki"
vm_timeout_secs: 10
# optional ca_cert
# ca_cert_path: "/path/to/ca_cert.pem"
# not optional as my environment requires mTLS for VictoriaMetrics
client_cert_path: "/path/to/crt.crt"
client_key_path: "/path/to/key.key"
```

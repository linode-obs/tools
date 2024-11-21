package main

import (
	"bufio"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"regexp"
	"time"

	"gopkg.in/yaml.v3"
)

// TODO - probably use optional for some of these fields
type Config struct {
	VMURL          string `yaml:"vm_url"`
	VMQuery        string `yaml:"vm_query"`
	VaultPKIPath   string `yaml:"vault_pki_path"`
	VMTimeoutSecs  int    `yaml:"vm_timeout_secs"`
	CACertPath     string `yaml:"ca_cert_path"`
	ClientCertPath string `yaml:"client_cert_path"`
	ClientKeyPath  string `yaml:"client_key_path"`
}

// Expect CN, OU, SN
type Certificate struct {
	CommonName         string
	OrganizationalUnit string
	SerialNumber       string
}

func LoadConfig(file string) (*Config, error) {
	config := &Config{}
	data, err := os.ReadFile(file)
	if err != nil {
		return nil, err
	}
	err = yaml.Unmarshal(data, config)
	return config, err
}

// CreateTLSClient creates an HTTP client with mTLS configured
// Make CA optional though
func CreateTLSClient(config *Config) (*http.Client, error) {
	var caCertPool *x509.CertPool

	if config.CACertPath != "" {
		caCert, err := os.ReadFile(config.CACertPath)
		if err != nil {
			return nil, fmt.Errorf("failed to load CA certificate: %w", err)
		}

		caCertPool = x509.NewCertPool()
		if !caCertPool.AppendCertsFromPEM(caCert) {
			return nil, fmt.Errorf("failed to append CA certificate")
		}
	}

	clientCert, err := tls.LoadX509KeyPair(config.ClientCertPath, config.ClientKeyPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load client certificate and key: %w", err)
	}

	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{clientCert},
		RootCAs:      caCertPool,
	}

	return &http.Client{
		Timeout:   time.Duration(config.VMTimeoutSecs) * time.Second,
		Transport: &http.Transport{TLSClientConfig: tlsConfig},
	}, nil
}

// QueryVictoriaMetrics queries VM with the provided PromQL query
// Should probably work for Prometheus too
func QueryVictoriaMetrics(config *Config) ([]Certificate, error) {
	client, err := CreateTLSClient(config)
	if err != nil {
		slog.Error("Failed to create TLS client", "error", err)
		return nil, err
	}

	queryParams := url.Values{}
	queryParams.Set("query", config.VMQuery)
	queryURL := fmt.Sprintf("%s?%s", config.VMURL, queryParams.Encode())

	resp, err := client.Get(queryURL)
	if err != nil {
		slog.Error("Failed to query VictoriaMetrics", "error", err)
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		slog.Error("VictoriaMetrics query failed", "status", resp.Status)
		return nil, fmt.Errorf("VM query failed: %s", resp.Status)
	}

	var result struct {
		Data struct {
			Result []struct {
				Metric map[string]string `json:"metric"`
				Value  []interface{}     `json:"value"`
			} `json:"result"`
		} `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		slog.Error("Failed to decode VictoriaMetrics response", "error", err)
		return nil, err
	}

	var certs []Certificate
	for _, res := range result.Data.Result {
		certs = append(certs, Certificate{
			CommonName:         res.Metric["common_name"],
			OrganizationalUnit: res.Metric["organizational_unit"],
			SerialNumber:       res.Metric["serial"],
		})
	}

	slog.Info("Certificates retrieved", "count", len(certs))
	return certs, nil
}

// FilterCertificates filters certificates by common_name using a regex
// TODO - might be nice to filter OU too?
func FilterCertificates(certs []Certificate, regex *regexp.Regexp) []Certificate {
	var filtered []Certificate
	for _, cert := range certs {
		if regex.MatchString(cert.CommonName) {
			filtered = append(filtered, cert)
		}
	}
	return filtered
}

// RevokeCertificate uses the Vault CLI to revoke a certificate
// Note - can't revoke expired certs
// https://github.com/hashicorp/vault/issues/19452
func RevokeCertificate(config *Config, cert Certificate, dryRun bool) error {
	vaultCommand := "vault"
	args := []string{
		"write",
		fmt.Sprintf("%s/revoke", config.VaultPKIPath),
		fmt.Sprintf("serial_number=%s", cert.SerialNumber),
	}

	if dryRun {
		slog.Info("[DRY RUN] Would revoke certificate", "serial_number", cert.SerialNumber, "common_name", cert.CommonName)
		return nil
	}

	slog.Info("Revoking certificate using Vault CLI", "serial_number", cert.SerialNumber, "command", vaultCommand, "args", args)

	cmd := exec.Command(vaultCommand, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		slog.Error("Vault CLI command failed", "error", err)
		return err
	}

	slog.Info("Certificate revoked successfully", "serial_number", cert.SerialNumber)
	return nil
}

func Confirm(prompt string) bool {
	fmt.Print(prompt + " [y/N]: ")
	scanner := bufio.NewScanner(os.Stdin)
	if scanner.Scan() {
		answer := scanner.Text()
		return answer == "y" || answer == "Y"
	}
	return false
}

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))

	configFile := flag.String("config", "", "Path to the configuration file")
	filterRegex := flag.String("filter-regex", "", "Regex to filter certificates by common_name")
	dryRun := flag.Bool("dry-run", false, "Simulate revocation without performing it")
	flag.Parse()

	if *configFile == "" {
		slog.Error("Missing config file argument")
		fmt.Println("Usage: go run main.go --config <config.yaml> [--filter-regex <regex>] [--dry-run]")
		os.Exit(1)
	}

	config, err := LoadConfig(*configFile)
	if err != nil {
		slog.Error("Failed to load config", "file", *configFile, "error", err)
		os.Exit(1)
	}

	certs, err := QueryVictoriaMetrics(config)
	if err != nil {
		slog.Error("Failed to query VictoriaMetrics", "error", err)
		os.Exit(1)
	}

	if len(certs) == 0 {
		slog.Info("No certificates retrieved")
		return
	}

	if *filterRegex != "" {
		regex, err := regexp.Compile(*filterRegex)
		if err != nil {
			slog.Error("Invalid regex", "regex", *filterRegex, "error", err)
			os.Exit(1)
		}
		certs = FilterCertificates(certs, regex)
		slog.Info("Certificates filtered by regex", "count", len(certs), "regex", *filterRegex)
	}

	if len(certs) == 0 {
		slog.Info("No certificates match the provided filter")
		return
	}

	for _, cert := range certs {
		if Confirm(fmt.Sprintf("Revoke certificate? Common Name: %s, Organizational Unit: %s, Serial: %s", cert.CommonName, cert.OrganizationalUnit, cert.SerialNumber)) {
			if err := RevokeCertificate(config, cert, *dryRun); err != nil {
				slog.Error("Failed to revoke certificate", "serial_number", cert.SerialNumber, "error", err)
			}
		}
	}
}

package main

import (
	"flag"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/prometheus/prometheus/model/rulefmt"
	"gopkg.in/yaml.v3"
)

func init() {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	slog.SetDefault(logger)
}

func logFatal(msg string, args ...any) {
	slog.Error(msg, args...)
	os.Exit(1)
}

func main() {
	flag.Parse()
	if len(flag.Args()) < 1 {
		logFatal("must provide one or more files to test", "args", flag.Args())
	}

	for _, yamlFile := range flag.Args() {
		yamlFile, err := filepath.Abs(yamlFile)
		if err != nil {
			logFatal("failed to parse arg as a filepath", "path", flag.Arg(0), "err", err)
		}
		slog.Info("Testing file", "path", yamlFile)

		roundtripRulesAndValidate(yamlFile)
	}

}

func loadRules(path string) *rulefmt.RuleGroups {
	d, err := os.ReadFile(path)
	if err != nil {
		logFatal("failed to read file", "path", path, "err", err)
	}
	rules := &rulefmt.RuleGroups{}
	if err := yaml.Unmarshal(d, rules); err != nil {
		if strings.Contains(err.Error(), "yaml: line ") {
			line, err := strconv.Atoi(strings.TrimSuffix(strings.Split(err.Error(), " ")[2], ":"))
			lineContextStart := line - 2
			if err == nil {
				fmt.Println("---")
				for i, l := range strings.Split(string(d), "\n")[lineContextStart : line+2] {
					fmt.Printf("\t%d: %s\n", lineContextStart+1+i, l)
				}
				fmt.Println("---")
			}
		}
		logFatal("failed to unmarshal YAML", "path", path, "err", err)
	}
	return rules
}

func dumpRules(rules *rulefmt.RuleGroups) *os.File {
	d, err := yaml.Marshal(rules)
	if err != nil {
		logFatal("failed to dump rules after parsing", "rules", rules, "err", err)
	}

	tmpFile, err := os.CreateTemp(os.TempDir(), "roundtrip-loki-rules-")
	if err != nil {
		logFatal("failed to create a tmp file to dump rules to", "err", err)
	}

	if err := os.WriteFile(tmpFile.Name(), d, 0644); err != nil {
		logFatal("failed to dump rules to tmp file", "tmpPath", tmpFile.Name(), "err", err)
	}

	slog.Info("dumped rules to tmp file", "tmpPath", tmpFile.Name())

	return tmpFile
}

func roundtripRulesAndValidate(path string) {
	rules := loadRules(path)
	newRulesFile := dumpRules(rules)
	slog.Info("validating new rules file parses as YAML", "renderedPath", newRulesFile.Name(), "originalPath", path)
	newRules := loadRules(newRulesFile.Name())
	slog.Info("success!", "rules", len(newRules.Groups))
}

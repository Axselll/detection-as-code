package main

import (
	"archive/tar"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

type DetectionRule struct {
	Name             string                 `yaml:"name"`
	Search           string                 `yaml:"search"`
	Enabled          bool                   `yaml:"enabled"`
	CronSchedule     string                 `yaml:"cron_schedule"`
	Description      string                 `yaml:"description,omitempty"`
	Dispatch         map[string]interface{} `yaml:"dispatch,omitempty"`
	EnableSched      bool                   `yaml:"enableSched,omitempty"`
	RealtimeSchedule bool                   `yaml:"realtime_schedule,omitempty"`
	Request          map[string]interface{} `yaml:"request,omitempty"`
	SchedulePriority string                 `yaml:"schedule_priority,omitempty"`
	ScheduleWindow   string                 `yaml:"schedule_window,omitempty"`
	Alert            map[string]interface{} `yaml:"alert,omitempty"`
	Quantity         int                    `yaml:"quantity,omitempty"`
	CountType        string                 `yaml:"counttype,omitempty"`
	Relation         string                 `yaml:"relation,omitempty"`
	Actions          []string               `yaml:"actions"`
	Action           map[string]interface{} `yaml:"action"`
}

const (
	sourceDir   = "../detection_as_code/detections/"
	destAppDir  = "../detection_as_code/app/detections_app"
	destConfDir = destAppDir + "/default"
	confFile    = destConfDir + "/savedsearches.conf"
	tarballPath = "../detection_as_code/app/detections_app.tar.gz"
)

func main() {
	fmt.Println("🔨 Building Splunk detection app...")

	if _, err := os.Stat(confFile); err == nil {
		err = os.Remove(confFile)
		check(err)
		fmt.Println("🗑️ Removed existing savedsearches.conf")
	}

	if _, err := os.Stat(tarballPath); err == nil {
		err = os.Remove(tarballPath)
		check(err)
		fmt.Println("🗑️ Removed existing tarball:", tarballPath)
	}

	os.MkdirAll(destConfDir, os.ModePerm)

	files, err := os.ReadDir(sourceDir)
	check(err)

	var output []string

	for _, file := range files {
		if strings.HasSuffix(file.Name(), ".yml") {
			path := filepath.Join(sourceDir, file.Name())
			rule, err := parseYAMLRule(path)
			if err != nil {
				fmt.Printf("⚠️  Skipping %s: %v\n", path, err)
				continue
			}
			conf := convertToSavedSearchConf(rule)
			output = append(output, conf)
		}
	}

	check(os.WriteFile(confFile, []byte(strings.Join(output, "\n")), 0644))
	fmt.Println("✅ savedsearches.conf created.")

	err = packageApp(destAppDir, tarballPath)
	check(err)
	fmt.Println("📦 Packaged app:", tarballPath)
}

func parseYAMLRule(path string) (DetectionRule, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return DetectionRule{}, err
	}
	var rule DetectionRule
	if err := yaml.Unmarshal(data, &rule); err != nil {
		return DetectionRule{}, err
	}
	if rule.Name == "" || rule.Search == "" || rule.CronSchedule == "" {
		return rule, errors.New("missing required fields (name, search, cron_schedule)")
	}
	return rule, nil
}

func convertToSavedSearchConf(rule DetectionRule) string {
	var lines []string
	lines = append(lines, fmt.Sprintf("[%s]", rule.Name))
	lines = append(lines, fmt.Sprintf("search = %s", rule.Search))
	lines = append(lines, fmt.Sprintf("cron_schedule = %s", rule.CronSchedule))
	lines = append(lines, fmt.Sprintf("disabled = %v", !rule.Enabled))

	if rule.Description != "" {
		lines = append(lines, fmt.Sprintf("description = %s", rule.Description))
	}
	if rule.EnableSched {
		lines = append(lines, "enableSched = 1")
	}
	lines = append(lines, fmt.Sprintf("realtime_schedule = %d", boolToInt(rule.RealtimeSchedule)))
	if rule.Quantity > 0 {
		lines = append(lines, fmt.Sprintf("quantity = %d", rule.Quantity))
	}
	if rule.CountType != "" {
		lines = append(lines, fmt.Sprintf("counttype = %s", rule.CountType))
	}
	if rule.Relation != "" {
		lines = append(lines, fmt.Sprintf("relation = %s", rule.Relation))
	}
	if rule.SchedulePriority != "" {
		lines = append(lines, fmt.Sprintf("schedule_priority = %s", rule.SchedulePriority))
	}
	if rule.ScheduleWindow != "" {
		lines = append(lines, fmt.Sprintf("schedule_window = %s", rule.ScheduleWindow))
	}

	for k, v := range rule.Dispatch {
		lines = append(lines, fmt.Sprintf("dispatch.%s = %v", k, v))
	}
	for k, v := range rule.Request {
		lines = append(lines, fmt.Sprintf("request.%s = %v", k, v))
	}
	for k, v := range rule.Alert {
		switch val := v.(type) {
		case []interface{}:
			var parts []string
			for _, i := range val {
				parts = append(parts, fmt.Sprintf("%v", i))
			}
			lines = append(lines, fmt.Sprintf("alert.%s = %s", k, strings.Join(parts, ",")))
		default:
			lines = append(lines, fmt.Sprintf("alert.%s = %v", k, val))
		}
	}

	if len(rule.Actions) > 0 {
		lines = append(lines, fmt.Sprintf("actions = %s", strings.Join(rule.Actions, ",")))
	}

	for actionKey, val := range rule.Action {
		flattenActionValue(fmt.Sprintf("action.%s", actionKey), val, &lines)
	}

	lines = append(lines, "")
	return strings.Join(lines, "\n")
}

func flattenActionValue(prefix string, val interface{}, lines *[]string) {
	switch v := val.(type) {
	case map[string]interface{}:
		for k, sub := range v {
			flattenActionValue(fmt.Sprintf("%s.%s", prefix, k), sub, lines)
		}
	case []interface{}:
		// Marshal list as JSON-like string
		serialized, _ := yaml.Marshal(v)
		clean := strings.TrimSpace(string(serialized))
		clean = strings.ReplaceAll(clean, "\n", "")
		*lines = append(*lines, fmt.Sprintf("%s = %s", prefix, clean))
	default:
		*lines = append(*lines, fmt.Sprintf("%s = %v", prefix, v))
	}
}

func packageApp(srcDir, tarball string) error {
	outFile, err := os.Create(tarball)
	if err != nil {
		return err
	}
	defer outFile.Close()

	gzw := gzip.NewWriter(outFile)
	defer gzw.Close()

	tw := tar.NewWriter(gzw)
	defer tw.Close()

	return filepath.Walk(srcDir, func(file string, fi os.FileInfo, err error) error {
		check(err)
		if fi.IsDir() {
			return nil
		}
		relPath := strings.TrimPrefix(file, filepath.Dir(srcDir)+"/")
		hdr, err := tar.FileInfoHeader(fi, "")
		check(err)
		hdr.Name = relPath

		if err := tw.WriteHeader(hdr); err != nil {
			return err
		}
		f, err := os.Open(file)
		check(err)
		defer f.Close()

		_, err = io.Copy(tw, f)
		return err
	})
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func check(err error) {
	if err != nil {
		panic(err)
	}
}

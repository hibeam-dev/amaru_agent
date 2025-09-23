package main

import (
	"flag"
	"testing"
)

func TestFlagParsing(t *testing.T) {
	oldArgs := flag.CommandLine
	defer func() { flag.CommandLine = oldArgs }()

	flag.CommandLine = flag.NewFlagSet("test", flag.ContinueOnError)

	jsonFlag := flag.Bool("json", false, "Enable JSON communication mode")
	pidFlag := flag.Bool("pid", false, "Write PID file")
	configFlag := flag.String("config", "config.toml", "Path to TOML configuration file")

	err := flag.CommandLine.Parse([]string{"-json", "-pid", "-config=test-config.toml"})
	if err != nil {
		t.Fatalf("Flag parsing failed: %v", err)
	}

	if !*jsonFlag {
		t.Error("Expected -json flag to be true")
	}
	if !*pidFlag {
		t.Error("Expected -pid flag to be true")
	}
	if *configFlag != "test-config.toml" {
		t.Errorf("Expected -config flag to be 'test-config.toml', got '%s'", *configFlag)
	}
}

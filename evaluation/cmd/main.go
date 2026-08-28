package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/z46-dev/sigil/evaluation"
	"github.com/z46-dev/sigil/service"
)

func main() {
	var (
		inputPath, outputPath, modeName string
		mode                            service.Mode
		observations                    []evaluation.Observation
		report                          evaluation.Report
		encoded                         []byte
		err                             error
	)

	modeName = os.Getenv("SIGIL_EVALUATION_MODE")
	if modeName == "" {
		modeName = "device"
	}

	flag.StringVar(&inputPath, "input", filepath.Join("evaluation", "reports", "observations.json"), "labeled observations JSON")
	flag.StringVar(&outputPath, "output", filepath.Join("evaluation", "reports", "latest.json"), "evaluation report JSON")
	flag.StringVar(&modeName, "mode", modeName, "fingerprint mode")
	flag.Parse()

	if mode, err = service.ParseMode(modeName); err == nil {
		// #nosec G304 -- The input path is explicitly supplied by the local CLI user.
		if encoded, err = os.ReadFile(inputPath); err == nil {
			err = json.Unmarshal(encoded, &observations)
		}
	}

	if err == nil {
		report, err = evaluation.Evaluate(mode, observations)
	}

	if err == nil {
		encoded, err = json.MarshalIndent(report, "", "  ")
	}

	if err == nil {
		// #nosec G703 -- The output path is explicitly supplied by the local CLI user.
		err = os.WriteFile(outputPath, append(encoded, '\n'), 0o600)
	}

	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	fmt.Printf("observations=%d accuracy=%.3f false_match=%.3f false_new=%.3f ambiguous=%.3f report=%s\n", report.Metrics.Observations, report.Metrics.Accuracy, report.Metrics.FalseMatchRate, report.Metrics.FalseNewRate, report.Metrics.AmbiguousRate, outputPath)
}

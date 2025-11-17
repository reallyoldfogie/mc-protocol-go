package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/reallyoldfogie/mc-protocol-go/internal/packetlogtest"
)

type Config struct {
	// Input
	Paths []string

	// Filtering
	Versions     []string
	IncludeNames []string
	ExcludeNames []string
	MaxPackets   int
	StopOnFirst  bool

	// Output
	MaxErrors   int
	Verbose     bool
	JSONFile    string
	SummaryOnly bool
}

func main() {
	cfg := parseFlags()

	if len(cfg.Paths) == 0 {
		fmt.Fprintln(os.Stderr, "Error: no input files specified")
		flag.Usage()
		os.Exit(1)
	}

	// Collect all log files from paths (files or directories)
	logFiles, err := collectLogFiles(cfg.Paths)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error collecting log files: %v\n", err)
		os.Exit(1)
	}

	if len(logFiles) == 0 {
		fmt.Fprintln(os.Stderr, "Error: no .log files found in specified paths")
		os.Exit(1)
	}

	if cfg.Verbose {
		fmt.Fprintf(os.Stderr, "Processing %d log file(s)...\n", len(logFiles))
	}

	// Process each file
	ctx := context.Background()
	results := make([]FileResult, 0, len(logFiles))
	totalSummary := packetlogtest.Summary{
		ByVersion:   make(map[string]int),
		ByDirection: make(map[string]int),
		ByName:      make(map[string]int),
	}

	for _, path := range logFiles {
		result := processFile(ctx, path, cfg)
		results = append(results, result)

		// Aggregate summaries
		totalSummary.Total += result.Summary.Total
		totalSummary.Succeeded += result.Summary.Succeeded
		totalSummary.Failed += result.Summary.Failed
		totalSummary.Skipped += result.Summary.Skipped

		for k, v := range result.Summary.ByVersion {
			totalSummary.ByVersion[k] += v
		}
		for k, v := range result.Summary.ByDirection {
			totalSummary.ByDirection[k] += v
		}
		for k, v := range result.Summary.ByName {
			totalSummary.ByName[k] += v
		}
		// Merge error summaries
		for _, es := range result.Summary.ErrorSummaries {
			mergeErrorSummary(&totalSummary, es)
		}
	}

	// Sort error summaries by count (descending)
	sortErrorSummaries(&totalSummary)

	// Output results
	if cfg.JSONFile != "" {
		if err := outputJSON(results, totalSummary, cfg.JSONFile); err != nil {
			fmt.Fprintf(os.Stderr, "Error writing JSON output: %v\n", err)
			os.Exit(1)
		}
		if cfg.Verbose {
			fmt.Fprintf(os.Stderr, "JSON results written to %s\n", cfg.JSONFile)
		}
	}
	outputText(results, totalSummary, cfg)

	// Exit with error code if any packets failed
	if totalSummary.Failed > 0 {
		os.Exit(1)
	}
}

func parseFlags() Config {
	cfg := Config{}

	// Input flags
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s [options] <file-or-directory>...\n\n", os.Args[0])
		fmt.Fprintln(os.Stderr, "Validates Minecraft protocol packet logs by performing round-trip Scan/Marshal")
		fmt.Fprintln(os.Stderr, "and ReadFrom/WriteTo checks on logged packet data.")
		fmt.Fprintln(os.Stderr, "Options:")
		flag.PrintDefaults()
	}

	// path inputs

	inputDir := flag.String("paths", "", "Comma-separated list of paths to import packet log files from")

	// Filtering options
	versionsFlag := flag.String("versions", "", "Comma-separated list of versions to process (e.g., '1.21.5,1.21.4')")
	includeFlag := flag.String("include", "", "Comma-separated list of packet name patterns to include")
	excludeFlag := flag.String("exclude", "", "Comma-separated list of packet name patterns to exclude")
	flag.IntVar(&cfg.MaxPackets, "max-packets", 0, "Maximum packets to process per file (0=unlimited)")
	flag.BoolVar(&cfg.StopOnFirst, "stop-on-first", false, "Stop processing file on first error")

	// Output options
	flag.IntVar(&cfg.MaxErrors, "max-errors", 100, "Maximum errors to collect and display")
	flag.BoolVar(&cfg.Verbose, "verbose", false, "Enable verbose output")
	flag.StringVar(&cfg.JSONFile, "json", "", "Write JSON results to specified file (text output still goes to stdout)")
	flag.BoolVar(&cfg.SummaryOnly, "summary", false, "Show only summary, no individual errors")

	flag.Parse()

	cfg.Paths = strings.Split(*inputDir, ",")

	// Parse comma-separated lists
	if *versionsFlag != "" {
		cfg.Versions = strings.Split(*versionsFlag, ",")
		for i := range cfg.Versions {
			cfg.Versions[i] = strings.TrimSpace(cfg.Versions[i])
		}
	}
	if *includeFlag != "" {
		cfg.IncludeNames = strings.Split(*includeFlag, ",")
		for i := range cfg.IncludeNames {
			cfg.IncludeNames[i] = strings.TrimSpace(cfg.IncludeNames[i])
		}
	}
	if *excludeFlag != "" {
		cfg.ExcludeNames = strings.Split(*excludeFlag, ",")
		for i := range cfg.ExcludeNames {
			cfg.ExcludeNames[i] = strings.TrimSpace(cfg.ExcludeNames[i])
		}
	}

	return cfg
}

func collectLogFiles(paths []string) ([]string, error) {
	var files []string
	seen := make(map[string]bool)

	for _, path := range paths {
		info, err := os.Stat(path)
		if err != nil {
			return nil, fmt.Errorf("stat %s: %w", path, err)
		}

		if info.IsDir() {
			// Walk directory recursively
			err := filepath.Walk(path, func(p string, info os.FileInfo, err error) error {
				if err != nil {
					return err
				}
				if !info.IsDir() && strings.HasSuffix(p, ".log") {
					absPath, err := filepath.Abs(p)
					if err != nil {
						return err
					}
					if !seen[absPath] {
						files = append(files, absPath)
						seen[absPath] = true
					}
				}
				return nil
			})
			if err != nil {
				return nil, fmt.Errorf("walk %s: %w", path, err)
			}
		} else if strings.HasSuffix(path, ".log") {
			absPath, err := filepath.Abs(path)
			if err != nil {
				return nil, err
			}
			if !seen[absPath] {
				files = append(files, absPath)
				seen[absPath] = true
			}
		}
	}

	return files, nil
}

type FileResult struct {
	Path    string
	Summary packetlogtest.Summary
	Error   error
}

func processFile(ctx context.Context, path string, cfg Config) FileResult {
	src, err := packetlogtest.NewFileSource(path)
	if err != nil {
		return FileResult{Path: path, Error: err}
	}
	defer src.Close()

	opts := packetlogtest.RoundTripOptions{
		Versions:         cfg.Versions,
		IncludeNames:     cfg.IncludeNames,
		ExcludeNames:     cfg.ExcludeNames,
		MaxPackets:       cfg.MaxPackets,
		MaxErrors:        cfg.MaxErrors,
		StopOnFirstError: cfg.StopOnFirst,
	}

	summary, err := packetlogtest.RunRoundTripChecks(ctx, src, opts)
	return FileResult{
		Path:    path,
		Summary: summary,
		Error:   err,
	}
}

func outputText(results []FileResult, total packetlogtest.Summary, cfg Config) {
	failedFiles := 0

	fmt.Printf("\n\n\n")
	// Per-file results
	for _, result := range results {
		if result.Error != nil {
			fmt.Printf("ERROR processing %s: %v\n\n", result.Path, result.Error)
			failedFiles++
			continue
		}

		fmt.Printf("File: %s\n", result.Path)
		fmt.Println(packetlogtest.FormatSummary(result.Summary, !cfg.SummaryOnly))

		if result.Summary.Failed > 0 {
			failedFiles++
		}
	}

	// Overall summary
	fmt.Println("=== OVERALL SUMMARY ===")
	fmt.Printf("Files processed: %d\n", len(results))
	fmt.Printf("Files with failures: %d\n", failedFiles)
	fmt.Println(packetlogtest.FormatSummary(total, !cfg.SummaryOnly))

}

func outputJSON(results []FileResult, total packetlogtest.Summary, filePath string) error {
	output := struct {
		Files []struct {
			Path    string                `json:"path"`
			Summary packetlogtest.Summary `json:"summary"`
			Error   string                `json:"error,omitempty"`
		} `json:"files"`
		Total packetlogtest.Summary `json:"total"`
	}{
		Total: total,
	}

	for _, result := range results {
		file := struct {
			Path    string                `json:"path"`
			Summary packetlogtest.Summary `json:"summary"`
			Error   string                `json:"error,omitempty"`
		}{
			Path:    result.Path,
			Summary: result.Summary,
		}
		if result.Error != nil {
			file.Error = result.Error.Error()
		}
		output.Files = append(output.Files, file)
	}

	f, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("create file: %w", err)
	}
	defer f.Close()

	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	if err := enc.Encode(output); err != nil {
		return fmt.Errorf("encode JSON: %w", err)
	}

	return nil
}

// mergeErrorSummary merges an error summary into the total, combining counts for duplicates
func mergeErrorSummary(total *packetlogtest.Summary, es packetlogtest.ErrorSummary) {
	// Look for existing error with same characteristics
	for i := range total.ErrorSummaries {
		if total.ErrorSummaries[i].Stage == es.Stage &&
			total.ErrorSummaries[i].Name == es.Name &&
			total.ErrorSummaries[i].ID == es.ID &&
			total.ErrorSummaries[i].Version == es.Version &&
			total.ErrorSummaries[i].Error == es.Error {
			// Found match - add counts
			total.ErrorSummaries[i].Count += es.Count
			return
		}
	}
	// No match found - add new error
	total.ErrorSummaries = append(total.ErrorSummaries, es)
}

// sortErrorSummaries sorts error summaries by count (descending), then by name
func sortErrorSummaries(total *packetlogtest.Summary) {
	for i := range total.ErrorSummaries {
		for j := i + 1; j < len(total.ErrorSummaries); j++ {
			if total.ErrorSummaries[j].Count > total.ErrorSummaries[i].Count ||
				(total.ErrorSummaries[j].Count == total.ErrorSummaries[i].Count &&
					total.ErrorSummaries[j].Name < total.ErrorSummaries[i].Name) {
				total.ErrorSummaries[i], total.ErrorSummaries[j] = total.ErrorSummaries[j], total.ErrorSummaries[i]
			}
		}
	}
}

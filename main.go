package main

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
)

var (
	// Version is set at build time via ldflags: -X main.version=<VERSION>
	version                   = "Dev"
	dateLayoutDefault         = "2006-01-02 15:04:05.000" // matches 2023-06-01 12:34:56.789
	dateLayoutSupport         = "2006-01-02 15:04:05.000" // can parse both . and , with a small tweak
	defaultPattern            = `\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2},\d{3}`
	supportPattern            = `\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2}\.\d{3}`
	lineContinuationDelimiter = "appTesting"
	workerCount               = 5 // concurrency limit for processing logs
)

// LogLine holds a parsed timestamp and the raw text of the log line.
type LogLine struct {
	Timestamp time.Time
	Raw       string
}

func main() {
	var parentFolder string
	flag.StringVar(&parentFolder, "parentFolder", "", "Path to the directory containing log files.")
	flag.StringVar(&parentFolder, "p", "", "(Short) Path to the directory containing log files.")
	showHelp := flag.Bool("h", false, "Display help.")
	flag.Parse()

	if *showHelp {
		displayHelp()
		return
	}
	if parentFolder == "" {
		fmt.Println("Error: --parentFolder is required.")
		flag.Usage()
		os.Exit(1)
	}

	// Validate path
	info, err := os.Stat(parentFolder)
	if err != nil || !info.IsDir() {
		fmt.Printf("Error: The provided path '%s' is not a valid directory.\n", parentFolder)
		os.Exit(1)
	}

	// Create or verify ProcessedLogs folder
	processFolder := createProcessedLogsFolder(parentFolder)

	// Gather .log files
	allLogs := getAllLogFiles(parentFolder)
	if len(allLogs) == 0 {
		fmt.Println("No .log files found in the specified directory or its subdirectories.")
		return
	}

	// Process logs in parallel
	processedLogFiles := processLogs(allLogs, processFolder)

	// Merge processed logs
	mergedFilePath := filepath.Join(processFolder, "MERGED.log")
	mergeProcessedLogs(processedLogFiles, mergedFilePath)

	// Determine date pattern from merged log
	dateTimePattern := determineDateTimePattern(mergedFilePath)
	if dateTimePattern == "" {
		fmt.Println("Warning: Could not detect date pattern. The ordering step may fail.")
	}

	// Order logs by date/time
	orderedFilePath := filepath.Join(processFolder, "MERGED_ORDERED.log")
	orderByDate(mergedFilePath, orderedFilePath, dateTimePattern)

	// Format logs (split lines by the lineContinuationDelimiter)
	finalFormattedFilePath := filepath.Join(processFolder, "FINAL_FORMATTED.log")
	formatSupport(orderedFilePath, finalFormattedFilePath, dateTimePattern)

	// Clean up
	cleanupProcessFolder(processFolder, finalFormattedFilePath)

	fmt.Println("All processing complete.")
	fmt.Printf("Final file saved at: %s\n", finalFormattedFilePath)
}

func displayHelp() {
	fmt.Println("LogProcessor - A CLI tool to merge and order log files. Version:", getVersion())
	fmt.Println()
	fmt.Println("Usage:")
	fmt.Println("  go run main.go --parentFolder \"C:\\path\\to\\log\\directory\"")
	fmt.Println("Options:")
	fmt.Println("  --parentFolder, -p    The path to the directory containing log files to be processed.")
	fmt.Println("  --help, -h            Display this help message.")
	fmt.Println()
}

func createProcessedLogsFolder(parentFolder string) string {
	processedLogsPath := filepath.Join(parentFolder, "ProcessedLogs")
	if _, err := os.Stat(processedLogsPath); os.IsNotExist(err) {
		if err := os.Mkdir(processedLogsPath, os.ModePerm); err != nil {
			fmt.Printf("Error creating ProcessedLogs folder: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("ProcessedLogs folder created successfully.")
	} else {
		fmt.Println("ProcessedLogs folder already exists.")
	}
	return processedLogsPath
}

func getAllLogFiles(folderPath string) []string {
	var logFiles []string
	err := filepath.Walk(folderPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			match, _ := regexp.MatchString(`\.log(\.\d+)?$`, info.Name())
			if match {
				logFiles = append(logFiles, path)
			}
		}
		return nil
	})
	if err != nil {
		fmt.Printf("Error searching for log files: %v\n", err)
	}
	return logFiles
}

func processLogs(logFiles []string, processFolder string) []string {
	jobs := make(chan string, len(logFiles))
	results := make(chan string, len(logFiles))
	errs := make(chan error, len(logFiles))

	var wg sync.WaitGroup

	// Spawn workerCount workers
	for w := 0; w < workerCount; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for logFile := range jobs {
				baseFileName := filepath.Base(logFile)
				processedLogFile := filepath.Join(processFolder, baseFileName)
				processedLogFile = getUniqueFileName(processedLogFile)

				if err := processLogFile(logFile, processedLogFile); err != nil {
					errs <- fmt.Errorf("%s was not processed: %v", logFile, err)
				} else {
					results <- processedLogFile
				}
			}
		}()
	}

	// Enqueue jobs
	for _, logFile := range logFiles {
		jobs <- logFile
	}
	close(jobs)

	// Wait for workers to finish
	wg.Wait()
	close(results)
	close(errs)

	// Collect results
	var processedLogFiles []string
	for r := range results {
		processedLogFiles = append(processedLogFiles, r)
	}
	for e := range errs {
		fmt.Println(e)
	}

	return processedLogFiles
}

func getUniqueFileName(filePath string) string {
	directory := filepath.Dir(filePath)
	fileNameWithoutExtension := strings.TrimSuffix(filepath.Base(filePath), filepath.Ext(filePath))
	extension := filepath.Ext(filePath)

	count := 1
	newFilePath := filePath

	for {
		_, err := os.Stat(newFilePath)
		if os.IsNotExist(err) {
			break
		}
		newFilePath = filepath.Join(directory, fmt.Sprintf("%s%d%s", fileNameWithoutExtension, count, extension))
		count++
	}
	return newFilePath
}

func processLogFile(inputFilePath, outputFilePath string) error {
	dateTimePattern := determineDateTimePattern(inputFilePath)
	if dateTimePattern == "" {
		return fmt.Errorf("skipping file %s due to unrecognized date pattern", inputFilePath)
	}

	compiledRegex, err := regexp.Compile(dateTimePattern)
	if err != nil {
		return fmt.Errorf("failed to compile regex pattern: %v", err)
	}

	inFile, err := os.Open(inputFilePath)
	if err != nil {
		return fmt.Errorf("error opening file %s: %v", inputFilePath, err)
	}
	defer inFile.Close()

	outFile, err := os.Create(outputFilePath)
	if err != nil {
		return fmt.Errorf("error creating output file %s: %v", outputFilePath, err)
	}
	defer outFile.Close()

	reader := bufio.NewReader(inFile)
	var currentLogEntry string
	lineNumber := 0

	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return fmt.Errorf("error reading line %d: %v", lineNumber, err)
		}
		lineNumber++
		line = strings.TrimRight(line, "\r\n")

		if compiledRegex.MatchString(line) {
			if currentLogEntry != "" {
				if _, err := outFile.WriteString(currentLogEntry + "\n"); err != nil {
					return fmt.Errorf("error writing to file %s: %v", outputFilePath, err)
				}
			}
			currentLogEntry = line
		} else if currentLogEntry != "" {
			currentLogEntry += lineContinuationDelimiter + line
		}
	}

	// Write the last collected entry if any
	if currentLogEntry != "" {
		if _, err := outFile.WriteString(currentLogEntry + "\n"); err != nil {
			return fmt.Errorf("error writing to file %s: %v", outputFilePath, err)
		}
	}

	return nil
}

func determineDateTimePattern(filePath string) string {
	f, err := os.Open(filePath)
	if err != nil {
		fmt.Printf("Error opening file for date pattern detection: %v\n", err)
		return ""
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	linesToCheck := 5
	for i := 0; i < linesToCheck && scanner.Scan(); i++ {
		line := scanner.Text()
		if matched, _ := regexp.MatchString(defaultPattern, line); matched {
			return defaultPattern
		}
		if matched, _ := regexp.MatchString(supportPattern, line); matched {
			return supportPattern
		}
	}
	return ""
}

func mergeProcessedLogs(logFiles []string, outputFilePath string) {
	outFile, err := os.Create(outputFilePath)
	if err != nil {
		fmt.Printf("Error creating merged file: %v\n", err)
		return
	}
	defer outFile.Close()

	for _, logFile := range logFiles {
		f, err := os.Open(logFile)
		if err != nil {
			fmt.Printf("Error opening file %s: %v\n", logFile, err)
			continue
		}
		defer f.Close()

		reader := bufio.NewReader(f)
		for {
			line, err := reader.ReadString('\n')
			if err != nil {
				if errors.Is(err, io.EOF) {
					break
				}
				fmt.Printf("Error reading line from %s: %v\n", logFile, err)
				break
			}
			outFile.WriteString(line)
		}
	}
	fmt.Printf("Merged logs saved at: %s\n", outputFilePath)
}

func orderByDate(inputFilePath, outputFilePath, dateTimePattern string) {
	content, err := os.ReadFile(inputFilePath)
	if err != nil {
		fmt.Printf("Error reading file: %v\n", err)
		return
	}

	rawLines := strings.Split(strings.TrimRight(string(content), "\r\n"), "\n")
	if dateTimePattern == "" {
		// If no pattern found, just write them as-is
		if err := os.WriteFile(outputFilePath, []byte(strings.Join(rawLines, "\n")), 0666); err != nil {
			fmt.Printf("Error writing file: %v\n", err)
		}
		return
	}

	var lines []LogLine
	regex, _ := regexp.Compile(dateTimePattern)

	for _, l := range rawLines {
		timestamp, parseErr := parseTimestampFromLine(l, regex)
		if parseErr != nil {
			fmt.Printf("Warning: could not parse timestamp for line: %q - error: %v\n", l, parseErr)
		}
		lines = append(lines, LogLine{
			Timestamp: timestamp, // zero time if parse fails
			Raw:       l,
		})
	}

	sort.Slice(lines, func(i, j int) bool {
		return lines[i].Timestamp.Before(lines[j].Timestamp)
	})

	sortedLines := make([]string, 0, len(lines))
	for _, line := range lines {
		sortedLines = append(sortedLines, line.Raw)
	}

	if err := os.WriteFile(outputFilePath, []byte(strings.Join(sortedLines, "\n")), 0666); err != nil {
		fmt.Printf("Error writing file: %v\n", err)
		return
	}
}

func parseTimestampFromLine(line string, pattern *regexp.Regexp) (time.Time, error) {
	match := pattern.FindString(line)
	if match == "" {
		return time.Time{}, fmt.Errorf("no timestamp found in line: %s", line)
	}
	normalized := strings.Replace(match, ",", ".", 1)
	parsed, err := time.Parse(dateLayoutDefault, normalized)
	if err != nil {
		return time.Time{}, err
	}
	return parsed, nil
}

func formatSupport(inputFilePath, outputFilePath, dateTimePattern string) {
	inFile, err := os.Open(inputFilePath)
	if err != nil {
		fmt.Printf("Error opening file: %v\n", err)
		return
	}
	defer inFile.Close()

	outFile, err := os.Create(outputFilePath)
	if err != nil {
		fmt.Printf("Error creating file: %v\n", err)
		return
	}
	defer outFile.Close()

	reader := bufio.NewReader(inFile)
	regex, _ := regexp.Compile(dateTimePattern)
	var logBuffer []string

	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			fmt.Printf("Error reading line: %v\n", err)
			break
		}
		line = strings.TrimRight(line, "\r\n")

		if regex.MatchString(line) {
			// Flush the buffer first
			if len(logBuffer) > 0 {
				for _, l := range logBuffer {
					outFile.WriteString(l + "\n")
				}
				logBuffer = nil
			}
			// Split the current line on continuation delimiter
			segments := strings.Split(line, lineContinuationDelimiter)
			for _, seg := range segments {
				outFile.WriteString(seg + "\n")
			}
		} else {
			// Accumulate in buffer
			logBuffer = append(logBuffer, line)
		}
	}

	// Flush any remaining buffer
	if len(logBuffer) > 0 {
		for _, l := range logBuffer {
			outFile.WriteString(l + "\n")
		}
	}
}

func cleanupProcessFolder(processFolder, finalFilePath string) {
	entries, err := os.ReadDir(processFolder)
	if err != nil {
		fmt.Printf("Error reading directory: %v\n", err)
		return
	}
	for _, e := range entries {
		fullPath := filepath.Join(processFolder, e.Name())
		if fullPath == finalFilePath {
			continue
		}
		if err := os.RemoveAll(fullPath); err != nil {
			fmt.Printf("Error removing %s: %v\n", fullPath, err)
		}
	}
}

func getVersion() string {
	return version
}

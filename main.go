package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
)

func main() {
	// Parse command-line arguments
	if len(os.Args) < 2 || containsHelpFlag(os.Args) {
		displayHelp()
		return
	}

	parentFolder := ""
	for i := 1; i < len(os.Args); i++ {
		if os.Args[i] == "--parentFolder" || os.Args[i] == "-p" {
			if i+1 < len(os.Args) {
				parentFolder = os.Args[i+1]
			} else {
				fmt.Println("Error: No path specified for the parent folder.")
				displayHelp()
				return
			}
		}
	}

	if parentFolder == "" {
		fmt.Println("Error: Parent folder path is required.")
		displayHelp()
		return
	}

	if _, err := os.Stat(parentFolder); os.IsNotExist(err) {
		fmt.Printf("Error: The provided path '%s' is not a valid directory.\n", parentFolder)
		return
	}

	processFolder := createProcessedLogsFolder(parentFolder)
	allLogs := getAllLogFiles(parentFolder)
	if len(allLogs) == 0 {
		fmt.Println("No .log files found in the specified directory and its subdirectories.")
		return
	}

	processedLogFiles := processLogs(allLogs, processFolder)
	mergedFilePath := filepath.Join(processFolder, "MERGED.log")
	mergeProcessedLogs(processedLogFiles, mergedFilePath)

	orderedFilePath := filepath.Join(processFolder, "MERGED_ORDERED.log")
	dateTimePattern := determineDateTimePattern(mergedFilePath)
	orderByDate(mergedFilePath, orderedFilePath, dateTimePattern)

	finalFormattedFilePath := filepath.Join(processFolder, "FINAL_FORMATTED.log")
	formatSupport(orderedFilePath, finalFormattedFilePath, dateTimePattern)

	cleanupProcessFolder(processFolder, finalFormattedFilePath)
	fmt.Printf("All processing complete. Final file saved at: %s\n", finalFormattedFilePath)
}

// Additional helper functions for the Go implementation

func containsHelpFlag(args []string) bool {
	for _, arg := range args {
		if arg == "-h" || arg == "--help" {
			return true
		}
	}
	return false
}

func displayHelp() {
	fmt.Println("LogProcessor - A CLI tool to merge and order log files. Ver. 1.0.1")
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
		err := os.Mkdir(processedLogsPath, os.ModePerm)
		if err != nil {
			fmt.Printf("An error occurred while creating the folder: %v\n", err)
			panic(err)
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
		if !info.IsDir() && (strings.HasSuffix(info.Name(), ".log") || regexp.MustCompile(`\.log(\.\d+)?$`).MatchString(info.Name())) {
			logFiles = append(logFiles, path)
		}
		return nil
	})
	if err != nil {
		fmt.Printf("An error occurred while searching for log files: %v\n", err)
	}
	return logFiles
}

func processLogs(logFiles []string, processFolder string) []string {
	var wg sync.WaitGroup
	var mu sync.Mutex
	var processedLogFiles []string

	for _, logFile := range logFiles {
		wg.Add(1)
		go func(logFile string) {
			defer wg.Done()
			baseFileName := filepath.Base(logFile)
			processedLogFile := filepath.Join(processFolder, baseFileName)
			processedLogFile = getUniqueFileName(processedLogFile)

			processLogFile(logFile, processedLogFile)
			mu.Lock()
			processedLogFiles = append(processedLogFiles, processedLogFile)
			mu.Unlock()
		}(logFile)
	}
	wg.Wait()
	return processedLogFiles
}

func getUniqueFileName(filePath string) string {
	directory := filepath.Dir(filePath)
	fileNameWithoutExtension := strings.TrimSuffix(filepath.Base(filePath), filepath.Ext(filePath))
	extension := filepath.Ext(filePath)

	count := 1
	newFilePath := filePath

	for _, err := os.Stat(newFilePath); !os.IsNotExist(err); _, err = os.Stat(newFilePath) {
		newFilePath = filepath.Join(directory, fmt.Sprintf("%s%d%s", fileNameWithoutExtension, count, extension))
		count++
	}
	return newFilePath
}

func processLogFile(inputFilePath, outputFilePath string) {
	dateTimePattern := determineDateTimePattern(inputFilePath)
	file, err := os.Open(inputFilePath)
	if err != nil {
		fmt.Printf("Error opening file: %v\n", err)
		return
	}
	defer file.Close()

	outputFile, err := os.Create(outputFilePath)
	if err != nil {
		fmt.Printf("Error creating file: %v\n", err)
		return
	}
	defer outputFile.Close()

	scanner := bufio.NewScanner(file)
	var realLogLine string
	for scanner.Scan() {
		line := scanner.Text()
		if regexp.MustCompile(dateTimePattern).MatchString(line) {
			if realLogLine != "" {
				outputFile.WriteString(realLogLine + "\n")
			}
			realLogLine = line
		} else if realLogLine != "" {
			realLogLine += "GHEGHE" + line
		}
	}
	if realLogLine != "" {
		outputFile.WriteString(realLogLine + "\n")
	}
}

func determineDateTimePattern(filePath string) string {
	return `^(\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2}\.\d{3})`
}

func mergeProcessedLogs(logFiles []string, outputFilePath string) {
	outputFile, err := os.Create(outputFilePath)
	if err != nil {
		fmt.Printf("Error creating file: %v\n", err)
		return
	}
	defer outputFile.Close()

	for _, logFile := range logFiles {
		file, err := os.Open(logFile)
		if err != nil {
			fmt.Printf("Error opening file: %v\n", err)
			continue
		}
		defer file.Close()

		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			outputFile.WriteString(scanner.Text() + "\n")
		}
	}
	fmt.Printf("Merged logs saved at: %s\n", outputFilePath)
}

func orderByDate(inputFilePath, outputFilePath, dateTimePattern string) {
	var logEntries []string
	file, err := os.Open(inputFilePath)
	if err != nil {
		fmt.Printf("Error opening file: %v\n", err)
		return
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		logEntries = append(logEntries, line)
	}

	sort.Slice(logEntries, func(i, j int) bool {
		dateI, _ := time.Parse("2006-01-02 15:04:05.000", logEntries[i][:23])
		dateJ, _ := time.Parse("2006-01-02 15:04:05.000", logEntries[j][:23])
		return dateI.Before(dateJ)
	})

	outputFile, err := os.Create(outputFilePath)
	if err != nil {
		fmt.Printf("Error creating file: %v\n", err)
		return
	}
	defer outputFile.Close()

	for _, entry := range logEntries {
		outputFile.WriteString(entry + "\n")
	}
}

func formatSupport(inputFilePath, outputFilePath, dateTimePattern string) {
	file, err := os.Open(inputFilePath)
	if err != nil {
		fmt.Printf("Error opening file: %v\n", err)
		return
	}
	defer file.Close()

	outputFile, err := os.Create(outputFilePath)
	if err != nil {
		fmt.Printf("Error creating file: %v\n", err)
		return
	}
	defer outputFile.Close()

	scanner := bufio.NewScanner(file)
	var logRow []string
	for scanner.Scan() {
		line := scanner.Text()
		if regexp.MustCompile(dateTimePattern).MatchString(line) {
			splitLines := strings.Split(line, "GHEGHE")
			logRow = append(logRow, splitLines...)
			for _, log := range logRow {
				outputFile.WriteString(log + "\n")
			}
			logRow = []string{}
		} else {
			logRow = append(logRow, line)
		}
	}
}

func cleanupProcessFolder(processFolder, finalFilePath string) {
	files, err := os.ReadDir(processFolder)
	if err != nil {
		fmt.Printf("Error reading directory: %v\n", err)
		return
	}
	for _, file := range files {
		filePath := filepath.Join(processFolder, file.Name())
		if filePath != finalFilePath {
			os.Remove(filePath)
		}
	}
	fmt.Println("All temporary files deleted, only the final formatted log file remains.")
}

package version

import "fmt"

var (
	// buildVersion — версия сборки приложения.
	buildVersion string
	// buildDate — дата сборки приложения.
	buildDate string
	// buildCommit — хеш коммита сборки.
	buildCommit string
)

// PrintBuildInfo выводит информацию о сборке приложения.
func PrintBuildInfo() {
	version := "N/A"
	if buildVersion != "" {
		version = buildVersion
	}
	date := "N/A"
	if buildDate != "" {
		date = buildDate
	}
	commit := "N/A"
	if buildCommit != "" {
		commit = buildCommit
	}

	fmt.Printf("Build version: %s\n", version)
	fmt.Printf("Build date: %s\n", date)
	fmt.Printf("Build commit: %s\n", commit)
}

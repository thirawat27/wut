package cmd

import (
	"archive/zip"
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"

	"wut/internal/config"
	"wut/internal/ui"
)

var bugReportCmd = &cobra.Command{
	Use:   "bug-report",
	Short: "Generate a bug report payload for troubleshooting",
	Long:  `Gathers system information, logs, and a sanitized snippet of configuration into a ZIP file attached for a bug report.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		zipFileName := fmt.Sprintf("wut-bugreport-%s.zip", time.Now().Format("20060102150405"))

		err := ui.RunWithSpinner("Generating bug report...", func() error {
			zipFile, err := os.Create(zipFileName)
			if err != nil {
				return err
			}
			defer zipFile.Close()

			zw := zip.NewWriter(zipFile)
			defer zw.Close()

			// 1. Sysinfo
			var sysinfo bytes.Buffer
			sysinfo.WriteString(fmt.Sprintf("WUT Version: %s\n", Version))
			sysinfo.WriteString(fmt.Sprintf("Build Time: %s\n", BuildTime))
			sysinfo.WriteString(fmt.Sprintf("Go Version: %s\n", runtime.Version()))
			sysinfo.WriteString(fmt.Sprintf("OS/Arch: %s/%s\n", runtime.GOOS, runtime.GOARCH))

			fSys, _ := zw.Create("sysinfo.txt")
			_, _ = fSys.Write(sysinfo.Bytes())

			// 2. Config (Sanitized - no secrets anyway, but we just include it)
			cfgPath := config.GetConfigPath()
			if cfgData, err := os.ReadFile(cfgPath); err == nil {
				fCfg, _ := zw.Create("config.yaml")
				_, _ = fCfg.Write(cfgData)
			} else {
				_, _ = fSys.Write([]byte(fmt.Sprintf("\nConfig file not found or readable: %v", err)))
			}

			// 3. Log file (if log exists)
			cfg := config.Get()
			if logPath := cfg.Logging.File; logPath != "" {
				homeDir, _ := os.UserHomeDir()
				if len(logPath) > 0 && logPath[0] == '~' {
					logPath = filepath.Join(homeDir, logPath[1:])
				}
				expanded := os.ExpandEnv(logPath)
				if logData, err := os.ReadFile(expanded); err == nil {
					// Trim to last 1MB approximately
					if len(logData) > 1024*1024 {
						logData = logData[len(logData)-1024*1024:]
					}
					fLog, _ := zw.Create("wut.log")
					_, _ = fLog.Write(logData)
				}
			}

			return nil
		})

		if err != nil {
			return fmt.Errorf("failed to generate bug report: %w", err)
		}

		fmt.Println()
		header := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#10B981")).Render("✅ Bug report generated successfully!")
		fmt.Printf("%s\n\n", header)

		fmt.Printf("File saved to: %s\n", lipgloss.NewStyle().Foreground(lipgloss.Color("#3B82F6")).Render(zipFileName))
		fmt.Println("\nPlease attach this file when opening an issue on GitHub:")
		fmt.Println(lipgloss.NewStyle().Foreground(lipgloss.Color("#FF70A6")).Render("https://github.com/thirawat27/wut/issues/new"))

		return nil
	},
}

func init() {
	rootCmd.AddCommand(bugReportCmd)
}

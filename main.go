// FilmVision Analyzer - Videó elemző alkalmazás
// A program képes mind parancssori (CLI), mind szöveges felhasználói felülettel (TUI) működni.
package main

import (
	"fmt"
	"os"
)

// main - A program belépési pontja
// Kezeli mind a CLI, mind a TUI módot
func main() {
	if len(os.Args) > 1 {
		// CLI mód - parancssori argumentumokkal
		config := AnalysisConfig{
			FrameRate:       1.0,
			OutputDir:       "frames_output",
			ResultsFile:     "analysis_results.txt",
			AnalyzeContrast: true,
			AnalyzeMotion:   true,
			AnalyzeShotType: true,
		}
		if err := analyzeVideo(os.Args[1], config); err != nil {
			fmt.Printf("Hiba a videó elemzése közben: %v\n", err)
			os.Exit(1)
		}
	} else {
		// TUI mód - interaktív felülettel
		runTUI()
	}
}

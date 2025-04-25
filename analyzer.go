// A FilmVision Analyzer fő elemző modulja
// Ez a modul tartalmazza a videó elemzéséhez szükséges összes funkciót
package main

import (
	"fmt"
	"image"
	"log"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/disintegration/imaging"
	"github.com/olekukonko/tablewriter"
)

// Alapértelmezett beállítások
const defaultFrameRate = 1     // Alapértelmezett képkocka sebesség (fps)
const defaultResizeWidth = 320 // Alapértelmezett szélesség átméretezéshez

// LuminanceStats tartalmazza a részletes elemzési eredményeket
type LuminanceStats struct {
	Mean   float64 // Átlagos fényerő
	Min    float64 // Minimális fényerő
	Max    float64 // Maximális fényerő
	StdDev float64 // Szórás
}

// AnalysisConfig - Az elemzés konfigurációs struktúrája
// Tartalmazza az összes szükséges paramétert a videó elemzéséhez
type AnalysisConfig struct {
	FrameRate        float64 // Képkocka sebesség (másodpercenkénti képkockák száma)
	TimeInterval     float64 // Időintervallum másodpercben
	OutputDir        string  // Kimeneti mappa útvonala
	ResultsFile      string  // Eredmény fájl útvonala
	AnalyzeContrast  bool    // Kontraszt elemzés engedélyezése
	AnalyzeMotion    bool    // Mozgás érzékelés engedélyezése
	AnalyzeShotType  bool    // Képkivágás típus felismerés engedélyezése
	AnalyzeLuminance bool    // Fényerősség elemzés engedélyezése
}

// analyzeVideo - Videó elemzése a megadott konfigurációval
// A függvény a következő lépéseket hajtja végre:
// 1. Képkockák kinyerése a videóból
// 2. Képkockák elemzése a kiválasztott módszerekkel
// 3. Eredmények megjelenítése és mentése
func analyzeVideo(videoPath string, config AnalysisConfig) error {
	baseName := strings.TrimSuffix(filepath.Base(videoPath), filepath.Ext(videoPath))
	outputDir := config.OutputDir
	if outputDir == "" {
		outputDir = "frames_" + baseName
	}

	// 1. Képkockák kinyerése FFmpeg segítségével
	fmt.Println("🎬 [1] Képkockák kinyerése...")
	os.Mkdir(outputDir, 0755)
	ffmpegCmd := exec.Command("ffmpeg",
		"-i", videoPath,
		"-vf", fmt.Sprintf("fps=%.2f,scale=%d:-1", config.FrameRate, defaultResizeWidth),
		filepath.Join(outputDir, "frame_%04d.jpg"))
	ffmpegCmd.Stdout = os.Stdout
	ffmpegCmd.Stderr = os.Stderr
	if err := ffmpegCmd.Run(); err != nil {
		return fmt.Errorf("FFmpeg hiba: %v", err)
	}

	// 2. Képkocka fájlok listázása
	files, err := filepath.Glob(filepath.Join(outputDir, "*.jpg"))
	if err != nil {
		return err
	}
	if len(files) == 0 {
		return fmt.Errorf("nem találhatók képkockák")
	}

	// 3. Képkockák elemzése
	fmt.Println("🧠 [2] Képkockák elemzése...")
	data := [][]string{}
	var prevImg image.Image

	for i, f := range files {
		img, err := imaging.Open(f)
		if err != nil {
			log.Printf("⚠️ Hiba a kép betöltése közben %s: %v", f, err)
			continue
		}

		frameNum := fmt.Sprintf("%03d", i+1)
		row := []string{frameNum, filepath.Base(f)}

		if config.AnalyzeContrast {
			contrast := calculateContrast(img)
			row = append(row, fmt.Sprintf("%.2f", contrast))
		}

		if config.AnalyzeLuminance {
			luminance := calculateLuminance(img, 2.2, false)
			row = append(row, fmt.Sprintf("%.2f", luminance.Mean))
			row = append(row, fmt.Sprintf("%.2f", luminance.StdDev))
		}

		if config.AnalyzeMotion && prevImg != nil {
			motion := detectMotion(prevImg, img)
			row = append(row, motion)
		}
		prevImg = img

		if config.AnalyzeShotType {
			shotType := detectShotType(img)
			row = append(row, shotType)
		}

		data = append(data, row)
	}

	// 4. Eredmények megjelenítése és mentése
	fmt.Println("\n📊 [3] Eredmények:")
	table := tablewriter.NewWriter(os.Stdout)
	headers := []string{"Képkocka #", "Fájl"}
	if config.AnalyzeContrast {
		headers = append(headers, "Kontraszt")
	}
	if config.AnalyzeLuminance {
		headers = append(headers, "Átl. Fényerő", "Fényerő Szórás")
	}
	if config.AnalyzeMotion {
		headers = append(headers, "Mozgás")
	}
	if config.AnalyzeShotType {
		headers = append(headers, "Képkivágás")
	}
	table.SetHeader(headers)
	table.SetBorder(true)
	table.AppendBulk(data)
	table.Render()

	// Eredmények mentése fájlba
	if config.ResultsFile != "" {
		if err := saveResultsToFile(config.ResultsFile, data, headers); err != nil {
			log.Printf("⚠️ Hiba az eredmények mentése közben: %v", err)
		}
	}

	fmt.Printf("\n✅ Kész. %d képkocka elemezve.\n", len(files))
	return nil
}

// calculateContrast - Kép kontrasztjának kiszámítása
// A függvény a képpontok szürkeárnyalatos értékeinek varianciáját számítja ki
func calculateContrast(img image.Image) float64 {
	bounds := img.Bounds()
	var sum, sumSq float64
	var count int

	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			r, g, b, _ := img.At(x, y).RGBA()
			gray := 0.299*float64(r) + 0.587*float64(g) + 0.114*float64(b)
			sum += gray
			sumSq += gray * gray
			count++
		}
	}

	mean := sum / float64(count)
	variance := (sumSq / float64(count)) - (mean * mean)
	return math.Sqrt(variance) / 65535.0
}

// calculateLuminance kiegészített verzió opcionális paraméterekkel
func calculateLuminance(img image.Image, gamma float64, debug bool) LuminanceStats {
	start := time.Now()
	defer func() {
		if debug {
			elapsed := time.Since(start)
			log.Printf("Feldolgozási idő: %s | FPS: %.1f",
				elapsed, 1/elapsed.Seconds())
		}
	}()

	bounds := img.Bounds()
	width, height := bounds.Dx(), bounds.Dy()

	stats := LuminanceStats{
		Min: math.MaxFloat64,
		Max: -math.MaxFloat64,
	}

	if width*height == 0 {
		return stats
	}

	numWorkers := runtime.NumCPU()
	var wg sync.WaitGroup
	wg.Add(numWorkers)

	type workerResult struct {
		sum      float64
		sumSq    float64
		min, max float64
		count    int
	}

	results := make(chan workerResult, numWorkers)
	segmentHeight := height / numWorkers

	for worker := 0; worker < numWorkers; worker++ {
		go func(workerID int) {
			defer wg.Done()

			local := workerResult{
				min: math.MaxFloat64,
				max: -math.MaxFloat64,
			}

			startY := bounds.Min.Y + workerID*segmentHeight
			endY := startY + segmentHeight
			if workerID == numWorkers-1 {
				endY = bounds.Max.Y
			}

			for y := startY; y < endY; y++ {
				for x := bounds.Min.X; x < bounds.Max.X; x++ {
					r, g, b, _ := img.At(x, y).RGBA()

					// Gamma korrekció
					rL := math.Pow(float64(r)/65535, gamma)
					gL := math.Pow(float64(g)/65535, gamma)
					bL := math.Pow(float64(b)/65535, gamma)

					lum := 0.299*rL + 0.587*gL + 0.114*bL

					local.sum += lum
					local.sumSq += lum * lum
					local.count++

					if lum < local.min {
						local.min = lum
					}
					if lum > local.max {
						local.max = lum
					}
				}
			}

			results <- local
		}(worker)
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	// Összesítés
	totalSum := 0.0
	totalSumSq := 0.0
	totalCount := 0
	globalMin := math.MaxFloat64
	globalMax := -math.MaxFloat64

	for res := range results {
		totalSum += res.sum
		totalSumSq += res.sumSq
		totalCount += res.count

		if res.min < globalMin {
			globalMin = res.min
		}
		if res.max > globalMax {
			globalMax = res.max
		}
	}

	// Statisztikai számítások
	mean := totalSum / float64(totalCount)
	variance := (totalSumSq / float64(totalCount)) - (mean * mean)
	stdDev := math.Sqrt(variance)

	return LuminanceStats{
		Mean:   mean,
		Min:    globalMin,
		Max:    globalMax,
		StdDev: stdDev,
	}
}

// detectMotion - Mozgás érzékelése két képkocka között
// A függvény összehasonlítja a két képkocka pixeleit és meghatározza a mozgás mértékét
func detectMotion(img1, img2 image.Image) string {
	bounds := img1.Bounds()
	var diffSum float64
	var count int

	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			r1, g1, b1, _ := img1.At(x, y).RGBA()
			r2, g2, b2, _ := img2.At(x, y).RGBA()

			diff := math.Abs(float64(r1)-float64(r2)) +
				math.Abs(float64(g1)-float64(g2)) +
				math.Abs(float64(b1)-float64(b2))
			diffSum += diff
			count++
		}
	}

	avgDiff := diffSum / float64(count)
	if avgDiff > 10000 {
		return "Magas"
	} else if avgDiff > 5000 {
		return "Közepes"
	}
	return "Alacsony"
}

// detectShotType - Képkivágás típusának felismerése
// A kép mérete alapján meghatározza a képkivágás típusát:
// - CU: Közeli (Close-up)
// - MS: Félközeli (Medium shot)
// - LS: Távoli (Long shot)
func detectShotType(img image.Image) string {
	bounds := img.Bounds()
	width := bounds.Max.X - bounds.Min.X
	height := bounds.Max.Y - bounds.Min.Y

	area := width * height
	if area < 100000 {
		return "KÖ" // Közeli
	} else if area < 200000 {
		return "FK" // Félközeli
	}
	return "TÁ" // Távoli
}

// saveResultsToFile - Eredmények mentése fájlba
// Az elemzési eredményeket TSV (tab-separated values) formátumban menti
func saveResultsToFile(filename string, data [][]string, headers []string) error {
	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	// Fejlécek írása
	fmt.Fprintf(file, "%s\n", strings.Join(headers, "\t"))

	// Adatok írása
	for _, row := range data {
		fmt.Fprintf(file, "%s\n", strings.Join(row, "\t"))
	}

	return nil
}

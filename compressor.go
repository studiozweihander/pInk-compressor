package main

import (
	"bufio"
	"bytes"
	"flag"
	"fmt"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
)

type ImageJob struct {
	SourcePath string
	DestPath   string
	OrigSize   int64
}

type ConversionResult struct {
	SourcePath string
	DestPath   string
	OrigSize   int64
	NewSize    int64
	Success    bool
	Error      error
}

type Stats struct {
	TotalFiles     int
	ProcessedFiles int
	OriginalSize   int64
	ConvertedSize  int64
	FailedFiles    int
}

const (
	outputDir = "compressed"
)

var (
	quality     int
	skip        bool
	ffmpegMutex sync.Mutex
)

func main() {
	fmt.Println(`
		█████            █████
          ░░███            ░░███
 ████████  ░███  ████████   ░███ █████     ██████   ██████  █████████████   ████████  ████████   ██████   █████   █████   ██████  ████████
░░███░░███ ░███ ░░███░░███  ░███░░███     ███░░███ ███░░███░░███░░███░░███ ░░███░░███░░███░░███ ███░░███ ███░░   ███░░   ███░░███░░███░░███
 ░███ ░███ ░███  ░███ ░███  ░██████░     ░███ ░░░ ░███ ░███ ░███ ░███ ░███  ░███ ░███ ░███ ░░░ ░███████ ░░█████ ░░█████ ░███ ░███ ░███ ░░░
 ░███ ░███ ░███  ░███ ░███  ░███░░███    ░███  ███░███ ░███ ░███ ░███ ░███  ░███ ░███ ░███     ░███░░░   ░░░░███ ░░░░███░███ ░███ ░███
 ░███████  █████ ████ █████ ████ █████   ░░██████ ░░██████  █████░███ █████ ░███████  █████    ░░██████  ██████  ██████ ░░██████  █████
 ░███░░░  ░░░░░ ░░░░ ░░░░░ ░░░░ ░░░░░     ░░░░░░   ░░░░░░  ░░░░░ ░░░ ░░░░░  ░███░░░  ░░░░░      ░░░░░░  ░░░░░░  ░░░░░░   ░░░░░░  ░░░░░
 ░███                                                                       ░███
 █████                                                                      █████
░░░░░                                                                      ░░░░░                                                           `)

	flag.IntVar(&quality, "quality", 80, "Qualidade da compressão (1-100)")
	flag.IntVar(&quality, "q", 80, "Qualidade da compressão (1-100)")
	flag.BoolVar(&skip, "skip", false, "Executar sem preview")
	flag.BoolVar(&skip, "s", false, "Executar sem preview")
	flag.Parse()

	if quality < 1 || quality > 100 {
		logError("Qualidade deve estar entre 1 e 100")
		os.Exit(1)
	}

	args := flag.Args()
	if len(args) == 0 {
		logError("Uso: go run compressor.go <pasta> [flags]")
		os.Exit(1)
	}

	inputDir := args[0]
	if inputDir == "." {
		var err error
		inputDir, err = os.Getwd()
		if err != nil {
			logError(fmt.Sprintf("Erro ao obter diretório atual: %v", err))
			os.Exit(1)
		}
	}

	if _, err := os.Stat(inputDir); os.IsNotExist(err) {
		logError(fmt.Sprintf("Diretório não encontrado: %s", inputDir))
		os.Exit(1)
	}

	outputPath := filepath.Join(inputDir, outputDir)

	images, err := scanImages(inputDir)
	if err != nil {
		logError(fmt.Sprintf("Erro ao escanear imagens: %v", err))
		os.Exit(1)
	}

	if len(images) == 0 {
		logInfo("Nenhuma imagem encontrada (PNG, JPEG, JPG, GIF)")
		return
	}

	logInfo(fmt.Sprintf("Encontradas %d imagens para processar", len(images)))
	logInfo(fmt.Sprintf("Qualidade: %d%%", quality))
	fmt.Println()

	if !skip {
		showPreview(images, outputPath)
		if !confirmExecution() {
			logInfo("Operação cancelada pelo usuário")
			return
		}
		fmt.Println()
	}

	if err := os.MkdirAll(outputPath, 0755); err != nil {
		logError(fmt.Sprintf("Erro ao criar pasta de saída: %v", err))
		os.Exit(1)
	}

	jobs := make(chan ImageJob, len(images))
	results := make(chan ConversionResult, len(images))

	var wg sync.WaitGroup
	workers := runtime.NumCPU()

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go worker(jobs, results, &wg)
	}

	for _, imgPath := range images {
		info, err := os.Stat(imgPath)
		if err != nil {
			continue
		}

		filename := filepath.Base(imgPath)
		nameWithoutExt := strings.TrimSuffix(filename, filepath.Ext(filename))
		destPath := filepath.Join(outputPath, nameWithoutExt+".webp")

		jobs <- ImageJob{
			SourcePath: imgPath,
			DestPath:   destPath,
			OrigSize:   info.Size(),
		}
	}
	close(jobs)

	go func() {
		wg.Wait()
		close(results)
	}()

	stats := Stats{TotalFiles: len(images)}

	for result := range results {
		if result.Success {
			stats.ProcessedFiles++
			stats.OriginalSize += result.OrigSize
			stats.ConvertedSize += result.NewSize

			reduction := float64(result.OrigSize-result.NewSize) / float64(result.OrigSize) * 100

			logSuccess(fmt.Sprintf("%s (%s) → %s (%s) [%.1f%% redução]",
				filepath.Base(result.SourcePath),
				formatSize(result.OrigSize),
				filepath.Base(result.DestPath),
				formatSize(result.NewSize),
				reduction,
			))
		} else {
			stats.FailedFiles++
			logError(fmt.Sprintf("%s: %v", filepath.Base(result.SourcePath), result.Error))
		}
	}

	fmt.Println()
	printSummary(stats)
}

func worker(jobs <-chan ImageJob, results chan<- ConversionResult, wg *sync.WaitGroup) {
	defer wg.Done()

	for job := range jobs {
		result := ConversionResult{
			SourcePath: job.SourcePath,
			DestPath:   job.DestPath,
			OrigSize:   job.OrigSize,
		}

		newSize, err := convertToWebP(job.SourcePath, job.DestPath, quality)
		if err != nil {
			result.Success = false
			result.Error = err
		} else {
			result.Success = true
			result.NewSize = newSize
		}

		results <- result
	}
}

func checkFFmpeg() bool {
	cmd := exec.Command("ffmpeg", "-version")
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	err := cmd.Run()

	if err != nil {
		cmd = exec.Command("where", "ffmpeg")
		if cmd.Run() != nil {
			return false
		}
	}

	return true
}

func convertToWebP(sourcePath, destPath string, quality int) (int64, error) {
	ffmpegMutex.Lock()
	defer ffmpegMutex.Unlock()

	cmd := exec.Command("ffmpeg",
		"-i", sourcePath,
		"-c:v", "libwebp",
		"-quality", fmt.Sprintf("%d", quality),
		"-y",
		destPath,
	)

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return 0, fmt.Errorf("ffmpeg error: %v - %s", err, stderr.String())
	}

	info, err := os.Stat(destPath)
	if err != nil {
		return 0, err
	}

	return info.Size(), nil
}

func scanImages(dir string) ([]string, error) {
	var images []string
	validExts := map[string]bool{
		".png":  true,
		".jpg":  true,
		".jpeg": true,
		".gif":  true,
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		ext := strings.ToLower(filepath.Ext(entry.Name()))
		if validExts[ext] {
			images = append(images, filepath.Join(dir, entry.Name()))
		}
	}

	return images, nil
}

func showPreview(images []string, outputPath string) {
	fmt.Println(strings.Repeat("─", 80))
	logInfo("PREVIEW - Arquivos que serão convertidos:")
	fmt.Println()

	var totalSize int64

	for _, imgPath := range images {
		info, err := os.Stat(imgPath)
		if err != nil {
			continue
		}

		filename := filepath.Base(imgPath)
		nameWithoutExt := strings.TrimSuffix(filename, filepath.Ext(filename))
		destFilename := nameWithoutExt + ".webp"

		totalSize += info.Size()

		fmt.Printf("  %s (%s) → %s\n",
			filename,
			formatSize(info.Size()),
			destFilename,
		)
	}

	fmt.Println()
	logInfo(fmt.Sprintf("Pasta de destino: %s", outputPath))
	logInfo(fmt.Sprintf("Tamanho total: %s", formatSize(totalSize)))
	fmt.Println(strings.Repeat("─", 80))
}

func confirmExecution() bool {
	reader := bufio.NewReader(os.Stdin)
	fmt.Print("\nDeseja continuar? (S/n): ")

	response, err := reader.ReadString('\n')
	if err != nil {
		return false
	}

	response = strings.TrimSpace(strings.ToLower(response))
	return response == "" || response == "s" || response == "sim"
}

func formatSize(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

func logInfo(msg string) {
	fmt.Printf("\033[36m[INFO]\033[0m %s\n", msg)
}

func logSuccess(msg string) {
	fmt.Printf("\033[32m[✓]\033[0m %s\n", msg)
}

func logError(msg string) {
	fmt.Printf("\033[31m[✗]\033[0m %s\n", msg)
}

func printSummary(stats Stats) {
	fmt.Println(strings.Repeat("─", 80))
	logInfo(fmt.Sprintf("Total de arquivos: %d", stats.TotalFiles))
	logSuccess(fmt.Sprintf("Processados: %d", stats.ProcessedFiles))

	if stats.FailedFiles > 0 {
		logError(fmt.Sprintf("Falhas: %d", stats.FailedFiles))
	}

	if stats.ProcessedFiles > 0 {
		reduction := float64(stats.OriginalSize-stats.ConvertedSize) / float64(stats.OriginalSize) * 100
		fmt.Printf("\033[36m[STATS]\033[0m Tamanho original: %s → Convertido: %s (%.1f%% redução)\n",
			formatSize(stats.OriginalSize),
			formatSize(stats.ConvertedSize),
			reduction,
		)
	}

	fmt.Println(strings.Repeat("─", 80))
}

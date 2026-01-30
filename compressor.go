package main

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
)

const (
	outputDir  = "compressed"
	asciiColor = "\033[38;5;212m"
	resetColor = "\033[0m"
	separator  = "────────────────────────────────────────────────────────────"
)

type Config struct {
	WorkDir string
	Quality int
	Skip    bool
	Replace bool
}

type Job struct {
	Source string
	Dest   string
}

type Stats struct {
	Total      int64
	Converted  int64
	Failed     int64
	SizeBefore int64
	SizeAfter  int64
}

var ffmpegMutex sync.Mutex

func main() {
	printASCII()

	cfg, err := parseArgs()
	if err != nil {
		fmt.Println()
		logError("%v", err)
		os.Exit(1)
	}

	if err := run(cfg); err != nil {
		fmt.Println()
		logError("%v", err)
		os.Exit(1)
	}
}

func run(cfg Config) error {
	logInfo("Diretório de trabalho: %s", cfg.WorkDir)

	if !hasCommand("ffmpeg") {
		return fmt.Errorf("ffmpeg não encontrado no sistema")
	}

	entries, err := os.ReadDir(cfg.WorkDir)
	if err != nil {
		return fmt.Errorf("Falha ao ler diretório")
	}

	var jobs []Job

	for _, e := range entries {
		if e.IsDir() {
			continue
		}

		name := e.Name()
		if strings.HasPrefix(name, ".") {
			continue
		}

		if !isImage(name) {
			continue
		}

		src := filepath.Join(cfg.WorkDir, name)

		var dst string
		if cfg.Replace {
			dst = filepath.Join(
				cfg.WorkDir,
				strings.TrimSuffix(name, filepath.Ext(name))+".webp",
			)
		} else {
			dst = filepath.Join(
				cfg.WorkDir,
				outputDir,
				strings.TrimSuffix(name, filepath.Ext(name))+".webp",
			)
		}

		jobs = append(jobs, Job{Source: src, Dest: dst})
	}

	if len(jobs) == 0 {
		return fmt.Errorf("Nenhum arquivo elegível para conversão encontrado")
	}

	if !cfg.Replace {
		if err := os.MkdirAll(filepath.Join(cfg.WorkDir, outputDir), 0755); err != nil {
			return fmt.Errorf("Falha ao criar pasta de saída")
		}
	}

	if !cfg.Skip {
		fmt.Println()
		fmt.Println(separator)
		fmt.Println()

		logInfo("Preview dos arquivos que serão convertidos")
		fmt.Println()

		for _, j := range jobs {
			fmt.Printf(
				"  %s → %s\n",
				filepath.Base(j.Source),
				filepath.Base(j.Dest),
			)
		}

		fmt.Println()
		if !confirmExecution() {
			logInfo("Operação cancelada pelo usuário")
			return nil
		}

		fmt.Println()
		fmt.Println(separator)
		fmt.Println()
	}

	var stats Stats
	workers := runtime.NumCPU()
	jobCh := make(chan Job, len(jobs))
	var wg sync.WaitGroup

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go worker(jobCh, &wg, cfg, &stats)
	}

	for _, j := range jobs {
		jobCh <- j
	}
	close(jobCh)

	wg.Wait()

	fmt.Println()
	printStats(stats)

	fmt.Println()
	logSuccess("Processo finalizado com sucesso.")

	return nil
}

func worker(jobs <-chan Job, wg *sync.WaitGroup, cfg Config, stats *Stats) {
	defer wg.Done()

	for job := range jobs {
		atomic.AddInt64(&stats.Total, 1)

		if info, err := os.Stat(job.Source); err == nil {
			atomic.AddInt64(&stats.SizeBefore, info.Size())
		}

		if err := convert(job.Source, job.Dest, cfg.Quality); err != nil {
			fmt.Println()
			logError("Erro ao converter %s", filepath.Base(job.Source))
			atomic.AddInt64(&stats.Failed, 1)
			continue
		}

		if info, err := os.Stat(job.Dest); err == nil {
			atomic.AddInt64(&stats.SizeAfter, info.Size())
		}

		if cfg.Replace {
			_ = os.Remove(job.Source)
		}

		fmt.Printf(
			"%s → %s\n",
			filepath.Base(job.Source),
			filepath.Base(job.Dest),
		)

		atomic.AddInt64(&stats.Converted, 1)
	}
}

func convert(src, dst string, quality int) error {
	ffmpegMutex.Lock()
	defer ffmpegMutex.Unlock()

	cmd := exec.Command(
		"ffmpeg",
		"-y",
		"-i", src,
		"-qscale", strconv.Itoa(quality),
		dst,
	)

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		if stderr.Len() > 0 {
			return fmt.Errorf("%s", strings.TrimSpace(stderr.String()))
		}
		return err
	}

	return nil
}

func parseArgs() (Config, error) {
	var cfg Config
	cfg.Quality = 80

	args := os.Args[1:]
	var rest []string

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "-q", "-quality":
			if i+1 >= len(args) {
				return cfg, fmt.Errorf("Qualidade inválida")
			}
			v, err := strconv.Atoi(args[i+1])
			if err != nil || v < 1 || v > 100 {
				return cfg, fmt.Errorf("Qualidade deve estar entre 1 e 100")
			}
			cfg.Quality = v
			i++
		case "-s", "-skip":
			cfg.Skip = true
		case "-replace", "-r":
			cfg.Replace = true
		default:
			rest = append(rest, args[i])
		}
	}

	if len(rest) > 0 {
		abs, err := filepath.Abs(rest[0])
		if err != nil {
			return cfg, fmt.Errorf("Diretório inválido")
		}
		info, err := os.Stat(abs)
		if err != nil || !info.IsDir() {
			return cfg, fmt.Errorf("Diretório não encontrado: %s", abs)
		}
		cfg.WorkDir = abs
	} else {
		dir, err := os.Getwd()
		if err != nil {
			return cfg, fmt.Errorf("Falha ao obter diretório atual")
		}
		cfg.WorkDir = dir
	}

	return cfg, nil
}

func confirmExecution() bool {
	reader := bufio.NewReader(os.Stdin)
	fmt.Print("Deseja continuar? (s/N): ")

	resp, err := reader.ReadString('\n')
	if err != nil {
		return false
	}

	resp = strings.TrimSpace(strings.ToLower(resp))
	return resp == "s" || resp == "sim"
}

func isImage(name string) bool {
	switch strings.ToLower(filepath.Ext(name)) {
	case ".png", ".jpg", ".jpeg", ".gif", ".avif":
		return true
	default:
		return false
	}
}

func hasCommand(cmd string) bool {
	_, err := exec.LookPath(cmd)
	return err == nil
}

func printStats(s Stats) {
	logInfo("Convertidos: %d", s.Converted)

	if s.Failed > 0 {
		logInfo("Falhas: %d", s.Failed)
	}

	logInfo("Tamanho antes: %.2f MB", bytesToMB(s.SizeBefore))
	logInfo("Tamanho depois: %.2f MB", bytesToMB(s.SizeAfter))

	if s.SizeBefore > 0 {
		reduction := float64(s.SizeBefore-s.SizeAfter) / float64(s.SizeBefore) * 100
		logInfo("Redução total: %.2f%%", reduction)
	}
}

func bytesToMB(b int64) float64 {
	return float64(b) / 1024 / 1024
}

func printASCII() {
	fmt.Print(asciiColor)
	fmt.Print(`
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
░░░░░                                                                      ░░░░░                                                           

`)
	fmt.Print(resetColor)
}

func logInfo(msg string, args ...any) {
	fmt.Printf("\033[34m[INFO]\033[0m "+msg+"\n", args...)
}

func logSuccess(msg string, args ...any) {
	fmt.Printf("\033[32m[SUCCESS]\033[0m "+msg+"\n", args...)
}

func logError(msg string, args ...any) {
	fmt.Printf("\033[31m[ERROR]\033[0m "+msg+"\n", args...)
}

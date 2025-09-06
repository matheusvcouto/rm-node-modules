package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/matheusvcouto/rm-node-modules/lib/log"
)

// Definições de constantes para configuração do script
// - MAX_SEARCH_WORKERS: 10 (busca)
// - MAX_DELETE_WORKERS: 10 (remoção)
// Se quiser ajustar para máquinas mais fracas, pode reduzir esses valores.
const (
	MAX_SEARCH_WORKERS int = 10  // Máximo de workers para busca de node_modules
	MAX_DELETE_WORKERS int = 10  // Máximo de workers para remoção de node_modules
	RESULTS_BUFFER     int = 100 // Tamanho do buffer dos canais de resultados
	JOBS_BUFFER        int = 100 // Tamanho do buffer dos canais de jobs
)

type nodeModulesDir struct {
	path string
	size int64
}

// Busca recursiva controlada pelo main, não pelos workers
func walkDirs(root string, jobs chan<- string) {
	stack := []string{root}
	for len(stack) > 0 {
		dir := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		entries, err := os.ReadDir(dir)
		if err != nil {
			log.Error(dir, err)
			continue
		}
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			name := entry.Name()
			fullPath := filepath.Join(dir, name)
			if name == "node_modules" {
				// Ignora node_modules dentro de outro node_modules
				if strings.Contains(dir, string(os.PathSeparator)+"node_modules") {
					continue
				}
				jobs <- fullPath
				continue
			}
			stack = append(stack, fullPath)
		}
	}
}

// Calcula o tamanho total de um diretório de forma segura
func dirSize(path string) (int64, error) {
	var size int64
	err := filepath.Walk(path, func(_ string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			size += info.Size()
		}
		return nil
	})
	return size, err
}

// worker para remoção de node_modules
// O parâmetro id foi removido pois não era utilizado
func deleteWorker(jobs <-chan nodeModulesDir, wg *sync.WaitGroup) {
	defer wg.Done()

	for dir := range jobs {
		start := time.Now()
		log.Deleting(dir.path)
		err := os.RemoveAll(dir.path)

		if err != nil {
			log.Error(dir.path, err)
		} else {
			elapsed := time.Since(start)
			log.Deleted(dir.path, formatSize(dir.size), elapsed)
		}
	}
}

func main() {
	fmt.Println()
	log.Section("Busca e remoção de node_modules")
	fmt.Println()

	// Verificação para impedir execução na raiz do sistema operacional e no diretório home do usuário
	startDir, err := os.Getwd()
	if err != nil {
		fmt.Printf("[ERRO] Não foi possível obter o diretório atual: %v\n", err)
		return
	}
	homeDir, err := os.UserHomeDir()
	if err != nil {
		fmt.Printf("[ERRO] Não foi possível obter o diretório home do usuário: %v\n", err)
		return
	}
	isRoot := false
	isHome := false
	if startDir == string(os.PathSeparator) {
		isRoot = true // Unix-like raiz
	}
	// Windows: verifica se está em C:\, D:\, etc
	if len(startDir) == 3 && startDir[1] == ':' && (startDir[2] == '\\' || startDir[2] == '/') {
		isRoot = true
	}
	// Verifica se está no diretório home do usuário
	if startDir == homeDir {
		isHome = true
	}
	if isRoot || isHome {
		fmt.Println("[ERRO] Não é permitido executar este script no diretório raiz do sistema operacional nem no diretório home do usuário!")
		return
	}

	startTime := time.Now()
	// Canal para receber os node_modules encontrados
	results := make(chan nodeModulesDir, RESULTS_BUFFER)
	// Canal de jobs para busca
	jobs := make(chan string, JOBS_BUFFER)

	var wg sync.WaitGroup

	// Envia diretório inicial
	if _, err := os.Stat(startDir); os.IsNotExist(err) {
		fmt.Printf("Diretório inicial não existe: %s\n", startDir)
		return
	}
	// Busca recursiva controlada pelo main
	go func() {
		walkDirs(startDir, jobs)
		close(jobs)
	}()

	// Inicia workers de busca
	for range MAX_SEARCH_WORKERS {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for path := range jobs {
				size, err := dirSize(path)
				if err != nil {
					log.Error(path, err)
					// Não adiciona ao canal de resultados se houver erro
					continue
				}
				results <- nodeModulesDir{path, size}
				log.Found(path, formatSize(size))
			}
		}()
	}

	// Outra goroutine para fechar results depois que jobs for fechado
	go func() {
		wg.Wait()
		close(results)
	}()

	// Coleta todos os node_modules encontrados
	nmDirs := make([]nodeModulesDir, 0, 10)
	totalSize := int64(0)
	for dir := range results {
		nmDirs = append(nmDirs, dir)
		totalSize += dir.size
	}

	totalTime := time.Since(startTime)

	if len(nmDirs) == 0 {
		fmt.Println("No node_modules directories found.")
		fmt.Println()
		return
	}

	fmt.Println() // Espaço extra para separar visualmente
	log.Separator("")
	fmt.Printf("Total encontrados: %d\n", len(nmDirs))
	fmt.Printf("Espaço total: %s / %.2f GB\n", formatSize(totalSize), float64(totalSize)/(1024*1024*1024))
	fmt.Printf("Tempo de busca: %s\n", totalTime.Round(time.Millisecond))
	log.Separator("")
	fmt.Println() // Espaço extra para separar visualmente
	fmt.Print("Deseja apagar esses diretórios? (y/n): ")

	var answer string
	fmt.Scanln(&answer)

	if strings.ToLower(answer) != "y" {
		fmt.Println("Operação cancelada.")
		return
	}

	fmt.Println() // Espaço extra para separar visualmente
	log.Section("Remoção de node_modules")
	fmt.Println() // Espaço extra para separar visualmente

	// Canal de jobs para deleção
	deleteJobs := make(chan nodeModulesDir, MAX_DELETE_WORKERS)
	var delWg sync.WaitGroup
	for i := 0; i < MAX_DELETE_WORKERS; i++ {
		delWg.Add(1)
		go deleteWorker(deleteJobs, &delWg)
	}

	// Envia jobs de deleção
	for _, dir := range nmDirs {
		deleteJobs <- dir
	}
	close(deleteJobs)
	delWg.Wait()
	fmt.Println()
	log.Section("Limpeza concluída!")
	fmt.Println()
}

func formatSize(bytes int64) string {
	const (
		KB = 1024
		MB = KB * 1024
		GB = MB * 1024
	)

	switch {
	case bytes >= GB:
		return fmt.Sprintf("%.2f GB", float64(bytes)/float64(GB))
	case bytes >= MB:
		return fmt.Sprintf("%.2f MB", float64(bytes)/float64(MB))
	case bytes >= KB:
		return fmt.Sprintf("%.2f KB", float64(bytes)/float64(KB))
	default:
		return strconv.FormatInt(bytes, 10) + " B"
	}
}

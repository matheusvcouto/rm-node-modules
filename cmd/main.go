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
// Recomendado para computadores com 16GB de RAM:
// - MAX_SEARCH_WORKERS: 10 (busca)
// - MAX_DELETE_WORKERS: 5 (remoção)
// Se quiser ajustar para máquinas mais fracas, pode reduzir esses valores.
const (
	MAX_SEARCH_WORKERS int = 10  // Máximo de workers para busca de node_modules
	MAX_DELETE_WORKERS int = 5   // Máximo de workers para remoção de node_modules
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
func dirSize(path string) int64 {
	var size int64
	_ = filepath.Walk(path, func(_ string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() {
			size += info.Size()
		}
		return nil
	})
	return size
}

// worker para remoção de node_modules
// O parâmetro id foi removido pois não era utilizado
func deleteWorker(jobs <-chan nodeModulesDir, wg *sync.WaitGroup, deleted chan<- string) {
	defer wg.Done()

	for dir := range jobs {
		start := time.Now()
		log.Deleting(dir.path)
		err := os.RemoveAll(dir.path)

		if err != nil {
			log.Error(dir.path, err)
			deleted <- fmt.Sprintf("[ERRO] %s: %v", dir.path, err)
		} else {
			elapsed := time.Since(start)
			log.Deleted(dir.path, formatSize(dir.size), elapsed)
			deleted <- fmt.Sprintf("Deleted: %s (%s, took %s)", dir.path, formatSize(dir.size), elapsed.Round(time.Millisecond))
		}
	}
}

func main() {
	fmt.Println()
	log.Section("Busca e remoção de node_modules")
	fmt.Println()
	// Canal para mostrar em tempo real os node_modules encontrados
	foundChan := make(chan string)
	// Canal para receber os node_modules encontrados
	results := make(chan nodeModulesDir, RESULTS_BUFFER)
	// Canal de jobs para busca
	jobs := make(chan string, JOBS_BUFFER)

	var wg sync.WaitGroup

	// Envia diretório inicial
	startDir, _ := os.Getwd()
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
	for i := range MAX_SEARCH_WORKERS {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for path := range jobs {
				size := dirSize(path)
				results <- nodeModulesDir{path, size}
				log.Found(path, formatSize(size))
				foundChan <- fmt.Sprintf("Found: %s (%s)", path, formatSize(size))
			}
		}(i)
	}

	// Goroutine para mostrar em tempo real os encontrados
	var foundWg sync.WaitGroup
	foundWg.Add(1)
	go func() {
		defer foundWg.Done()
		for msg := range foundChan {
			fmt.Println(msg) // substitua por printFound se quiser customizar ainda mais
		}
	}()

	// Outra goroutine para fechar results depois que jobs for fechado
	go func() {
		wg.Wait()
		close(results)
		close(foundChan)
	}()

	// Coleta todos os node_modules encontrados
	nmDirs := make([]nodeModulesDir, 0, 10)
	totalSize := int64(0)
	for dir := range results {
		nmDirs = append(nmDirs, dir)
		totalSize += dir.size
	}

	foundWg.Wait()

	if len(nmDirs) == 0 {
		fmt.Println("No node_modules directories found.")
		fmt.Println()
		return
	}

	fmt.Println() // Espaço extra para separar visualmente
	log.Separator("")
	fmt.Printf("Total encontrados: %d\n", len(nmDirs))
	fmt.Printf("Espaço total: %s / %.2f GB\n", formatSize(totalSize), float64(totalSize)/(1024*1024*1024))
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

	// Canal para mostrar em tempo real os deletados
	deletedChan := make(chan string)
	// Canal de jobs para deleção
	deleteJobs := make(chan nodeModulesDir, MAX_DELETE_WORKERS)
	var delWg sync.WaitGroup
	for i := 0; i < MAX_DELETE_WORKERS; i++ {
		delWg.Add(1)
		go deleteWorker(deleteJobs, &delWg, deletedChan)
	}

	// Goroutine para mostrar em tempo real os deletados
	// O canal deletedChan só é fechado após todos os workers terminarem,
	// garantindo que não haverá envio para canal fechado.
	var deletedWg sync.WaitGroup
	deletedWg.Add(1)
	go func() {
		defer deletedWg.Done()
		for msg := range deletedChan {
			fmt.Println(msg) // substitua por printDeleted/printError se quiser customizar ainda mais
		}
	}()

	// Envia jobs de deleção
	for _, dir := range nmDirs {
		deleteJobs <- dir
	}
	close(deleteJobs)
	delWg.Wait()
	close(deletedChan)
	deletedWg.Wait()
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

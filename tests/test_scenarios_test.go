package tests

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
)

// Teste de concorrência: cria muitos node_modules e garante que todos são encontrados
func TestManyNodeModules(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "test-many-nm")
	if err != nil {
		t.Fatalf("Erro ao criar diretório temporário: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	n := 50
	for i := 0; i < n; i++ {
		dir := filepath.Join(tmpDir, "proj"+strconv.Itoa(i), "node_modules")
		err := os.MkdirAll(dir, 0755)
		if err != nil {
			t.Fatalf("Erro ao criar node_modules: %v", err)
		}
	}

	jobs := make(chan string, 10)
	results := make(chan string, 10)
	var wg sync.WaitGroup
	maxWorkers := 5

	for i := 0; i < maxWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for path := range jobs {
				results <- path
			}
		}()
	}

	go func() {
		stack := []string{tmpDir}
		for len(stack) > 0 {
			dir := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			entries, _ := os.ReadDir(dir)
			for _, entry := range entries {
				if !entry.IsDir() {
					continue
				}
				fullPath := filepath.Join(dir, entry.Name())
				if entry.Name() == "node_modules" {
					if strings.Count(fullPath, string(os.PathSeparator)+"node_modules") > 1 {
						continue
					}
					jobs <- fullPath
					continue
				}
				stack = append(stack, fullPath)
			}
		}
		close(jobs)
	}()

	found := make(map[string]bool)
	go func() {
		wg.Wait()
		close(results)
	}()
	for path := range results {
		found[path] = true
	}

	if len(found) != n {
		t.Errorf("Esperado %d node_modules, encontrado %d", n, len(found))
	}
}

// Testa que node_modules_backup não é removido
func TestNodeModulesBackupNotRemoved(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "test-nm-backup")
	if err != nil {
		t.Fatalf("Erro ao criar diretório temporário: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	nm := filepath.Join(tmpDir, "node_modules")
	nmb := filepath.Join(tmpDir, "node_modules_backup")
	os.Mkdir(nm, 0755)
	os.Mkdir(nmb, 0755)

	jobs := make(chan string, 10)
	results := make(chan string, 10)
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for path := range jobs {
			results <- path
		}
	}()

	go func() {
		stack := []string{tmpDir}
		for len(stack) > 0 {
			dir := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			entries, _ := os.ReadDir(dir)
			for _, entry := range entries {
				if !entry.IsDir() {
					continue
				}
				fullPath := filepath.Join(dir, entry.Name())
				if entry.Name() == "node_modules" {
					jobs <- fullPath
					continue
				}
				stack = append(stack, fullPath)
			}
		}
		close(jobs)
	}()

	var found []string
	go func() {
		wg.Wait()
		close(results)
	}()
	for path := range results {
		found = append(found, filepath.Base(path))
	}

	if len(found) != 1 || found[0] != "node_modules" {
		t.Errorf("Só node_modules deveria ser encontrado, achou: %v", found)
	}
}

// Testa que não há erro se não existir node_modules
func TestNoNodeModules(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "test-no-nm")
	if err != nil {
		t.Fatalf("Erro ao criar diretório temporário: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	jobs := make(chan string, 10)
	results := make(chan string, 10)
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for path := range jobs {
			results <- path
		}
	}()

	go func() {
		stack := []string{tmpDir}
		for len(stack) > 0 {
			dir := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			entries, _ := os.ReadDir(dir)
			for _, entry := range entries {
				if !entry.IsDir() {
					continue
				}
				fullPath := filepath.Join(dir, entry.Name())
				if entry.Name() == "node_modules" {
					jobs <- fullPath
					continue
				}
				stack = append(stack, fullPath)
			}
		}
		close(jobs)
	}()

	var found []string
	go func() {
		wg.Wait()
		close(results)
	}()
	for path := range results {
		found = append(found, path)
	}

	if len(found) != 0 {
		t.Errorf("Não deveria encontrar node_modules, achou: %v", found)
	}
}

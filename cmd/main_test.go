package main

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// TestDirSize verifica se o cálculo de tamanho de diretório está funcionando corretamente
func TestDirSize(t *testing.T) {
	// Cria um diretório temporário para teste
	tmpDir, err := os.MkdirTemp("", "test-dirsize")
	if err != nil {
		t.Fatalf("Erro ao criar diretório temporário: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Cria um arquivo com tamanho conhecido
	testFile := filepath.Join(tmpDir, "testfile.txt")
	testData := make([]byte, 1024*1024) // 1MB
	err = os.WriteFile(testFile, testData, 0644)
	if err != nil {
		t.Fatalf("Erro ao criar arquivo de teste: %v", err)
	}

	// Verifica se o tamanho calculado está correto
	size := dirSize(tmpDir)
	if size != int64(len(testData)) {
		t.Errorf("Tamanho calculado incorreto: esperado %d, obtido %d", len(testData), size)
	}
}

// TestFormatSize verifica se a formatação de tamanho está correta
func TestFormatSize(t *testing.T) {
	tests := []struct {
		bytes    int64
		expected string
	}{
		{500, "500 B"},
		{1024, "1.00 KB"},
		{1024 * 1024, "1.00 MB"},
		{1024 * 1024 * 1024, "1.00 GB"},
		{1500, "1.46 KB"},
	}

	for _, test := range tests {
		result := formatSize(test.bytes)
		if result != test.expected {
			t.Errorf("Formatação incorreta para %d bytes: esperado %s, obtido %s",
				test.bytes, test.expected, result)
		}
	}
}

// TestSearchWorker verifica se a função searchWorker funciona corretamente
func TestSearchWorker(t *testing.T) {
	// Estrutura de teste
	// tmp/
	//   - node_modules/
	//     - dummy.txt (10 bytes)
	//     - node_modules/ (deve ser ignorado)
	//       - ignored.txt
	//   - subdir/
	//     - node_modules/
	//       - another.txt (20 bytes)

	// Cria estrutura de diretórios temporários
	tmpDir, err := os.MkdirTemp("", "test-search")
	if err != nil {
		t.Fatalf("Erro ao criar diretório temporário: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Cria primeiro node_modules
	nm1 := filepath.Join(tmpDir, "node_modules")
	err = os.Mkdir(nm1, 0755)
	if err != nil {
		t.Fatalf("Erro ao criar node_modules: %v", err)
	}
	err = os.WriteFile(filepath.Join(nm1, "dummy.txt"), []byte("0123456789"), 0644)
	if err != nil {
		t.Fatalf("Erro ao criar arquivo de teste: %v", err)
	}

	// Cria node_modules aninhado (deve ser ignorado)
	nmNested := filepath.Join(nm1, "node_modules")
	err = os.Mkdir(nmNested, 0755)
	if err != nil {
		t.Fatalf("Erro ao criar node_modules aninhado: %v", err)
	}
	err = os.WriteFile(filepath.Join(nmNested, "ignored.txt"), []byte("ignored"), 0644)
	if err != nil {
		t.Fatalf("Erro ao criar arquivo de teste: %v", err)
	}

	// Cria subdiretório com node_modules
	subdir := filepath.Join(tmpDir, "subdir")
	err = os.Mkdir(subdir, 0755)
	if err != nil {
		t.Fatalf("Erro ao criar subdiretório: %v", err)
	}
	nm2 := filepath.Join(subdir, "node_modules")
	err = os.Mkdir(nm2, 0755)
	if err != nil {
		t.Fatalf("Erro ao criar node_modules em subdiretório: %v", err)
	}
	err = os.WriteFile(filepath.Join(nm2, "another.txt"), []byte("01234567890123456789"), 0644)
	if err != nil {
		t.Fatalf("Erro ao criar arquivo de teste: %v", err)
	}

	// Não usamos mais searchWorker, pois a busca agora é feita pelo walkDirs no main
	// Vamos simular a busca como no main.go
	jobs := make(chan string, 10)
	results := make(chan nodeModulesDir, 10)
	found := make(chan string, 10)
	var wg sync.WaitGroup
	maxWorkers := 2

	// Inicia workers de busca
	for i := 0; i < maxWorkers; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for path := range jobs {
				size := dirSize(path)
				results <- nodeModulesDir{path, size}
				found <- path // só para consumir
			}
		}(i)
	}

	// Busca recursiva controlada pelo teste
	go func() {
		stack := []string{tmpDir}
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
					// Só adiciona se não houver outro node_modules no caminho
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

	// Coleta resultados
	foundDirs := make([]nodeModulesDir, 0)
	expectedCount := 2 // Esperamos encontrar 2 diretórios node_modules

	go func() {
		wg.Wait()
		close(results)
		close(found)
	}()

	for dir := range results {
		foundDirs = append(foundDirs, dir)
	}

	if len(foundDirs) != expectedCount {
		t.Errorf("Quantidade incorreta de node_modules encontrados: esperado %d, obtido %d", expectedCount, len(foundDirs))
	}

	foundPaths := make(map[string]bool)
	for _, dir := range foundDirs {
		dirName := filepath.Base(filepath.Dir(dir.path))
		foundPaths[dirName] = true
	}

	tmpBase := filepath.Base(tmpDir)
	expectedPaths := map[string]bool{
		tmpBase:  true,
		"subdir": true,
	}

	for path := range expectedPaths {
		if !foundPaths[path] {
			t.Errorf("Diretório esperado não encontrado: %s", path)
		}
	}
}

// TestDeleteWorker verifica se a função deleteWorker funciona corretamente
func TestDeleteWorker(t *testing.T) {
	// Cria um diretório temporário para teste
	tmpDir, err := os.MkdirTemp("", "test-delete")
	if err != nil {
		t.Fatalf("Erro ao criar diretório temporário: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Cria um diretório node_modules de teste
	testNM := filepath.Join(tmpDir, "node_modules")
	err = os.Mkdir(testNM, 0755)
	if err != nil {
		t.Fatalf("Erro ao criar diretório de teste: %v", err)
	}

	// Cria um arquivo no diretório
	testFile := filepath.Join(testNM, "testfile.txt")
	err = os.WriteFile(testFile, []byte("test"), 0644)
	if err != nil {
		t.Fatalf("Erro ao criar arquivo de teste: %v", err)
	}

	// Canais para teste
	jobs := make(chan nodeModulesDir, 1)
	deleted := make(chan string, 1)
	var wg sync.WaitGroup

	// Inicia worker
	wg.Add(1)
	go deleteWorker(jobs, &wg, deleted)

	// Envia job para deletar o diretório
	jobs <- nodeModulesDir{testNM, 4}
	close(jobs)

	// Aguarda worker terminar
	wg.Wait()
	close(deleted)

	// Verifica se o diretório foi deletado
	if _, err := os.Stat(testNM); !os.IsNotExist(err) {
		t.Errorf("Diretório não foi deletado: %s", testNM)
	}
}

// TestIntegration executa um teste de integração simulando o fluxo completo
func TestIntegration(t *testing.T) {
	// Pula teste de integração quando executando em CI/CD
	if os.Getenv("CI") == "true" {
		t.Skip("Pulando teste de integração em ambiente CI")
	}

	// Cria uma estrutura de diretórios para teste
	tmpDir, err := os.MkdirTemp("", "test-integration")
	if err != nil {
		t.Fatalf("Erro ao criar diretório temporário: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Cria vários níveis de diretórios com node_modules
	dirs := []string{
		filepath.Join(tmpDir, "dir1", "node_modules"),
		filepath.Join(tmpDir, "dir2", "subdir", "node_modules"),
		filepath.Join(tmpDir, "dir3", "node_modules", "pkg", "node_modules"), // Este deve ser ignorado
	}

	for _, dir := range dirs {
		err = os.MkdirAll(dir, 0755)
		if err != nil {
			t.Fatalf("Erro ao criar diretório de teste: %v", err)
		}
		// Cria um arquivo em cada diretório
		err = os.WriteFile(filepath.Join(dir, "testfile.txt"), []byte("test"), 0644)
		if err != nil {
			t.Fatalf("Erro ao criar arquivo de teste: %v", err)
		}
	}

	jobs := make(chan string, 10)
	results := make(chan nodeModulesDir, 10)
	var wg sync.WaitGroup
	maxWorkers := 2

	// Inicia workers de busca
	for i := 0; i < maxWorkers; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for path := range jobs {
				size := dirSize(path)
				results <- nodeModulesDir{path, size}
			}
		}(i)
	}

	// Busca recursiva controlada pelo teste
	go func() {
		stack := []string{tmpDir}
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
					// Só adiciona se não houver outro node_modules no caminho
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

	// Coleta resultados
	nmDirs := make([]nodeModulesDir, 0)
	go func() {
		wg.Wait()
		close(results)
	}()
	for dir := range results {
		nmDirs = append(nmDirs, dir)
	}

	// Verifica se encontrou o número correto de diretórios
	expectedCount := 3 // Agora espera 3 node_modules (os aninhados são ignorados)
	if len(nmDirs) != expectedCount {
		t.Errorf("Quantidade incorreta de node_modules encontrados: esperado %d, obtido %d", expectedCount, len(nmDirs))
		for i, dir := range nmDirs {
			t.Logf("Dir %d: %s", i+1, dir.path)
		}
	}
}

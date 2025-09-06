package main

import (
	"os"
	"path/filepath"
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
	size, err := dirSize(tmpDir)
	if err != nil {
		t.Errorf("dirSize retornou um erro inesperado: %v", err)
	}
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

// TestWalkAndSearch verifica se a busca de diretórios e o cálculo de tamanho
// funcionam corretamente em conjunto.
func TestWalkAndSearch(t *testing.T) {
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

	jobs := make(chan string, 10)
	results := make(chan nodeModulesDir, 10)
	var wg sync.WaitGroup
	maxWorkers := 2

	// Inicia workers de busca
	for i := 0; i < maxWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for path := range jobs {
				size, err := dirSize(path)
				if err != nil {
					t.Errorf("dirSize failed for %s: %v", path, err)
				}
				results <- nodeModulesDir{path, size}
			}
		}()
	}

	// Roda a função de busca
	go func() {
		walkDirs(tmpDir, jobs)
		close(jobs)
	}()

	// Coleta resultados
	foundDirs := make(map[string]int64)
	go func() {
		wg.Wait()
		close(results)
	}()

	for dir := range results {
		foundDirs[dir.path] = dir.size
	}

	expectedCount := 2 // Esperamos encontrar 2 diretórios node_modules
	if len(foundDirs) != expectedCount {
		t.Errorf("Quantidade incorreta de node_modules encontrados: esperado %d, obtido %d", expectedCount, len(foundDirs))
	}

	if size, ok := foundDirs[nm1]; !ok || size != 17 { // 10 bytes for dummy.txt + 7 bytes for ignored.txt in nested dir
		t.Errorf("Diretório %s não encontrado ou com tamanho incorreto. Encontrado: %t, Tamanho: %d", nm1, ok, size)
	}
	if size, ok := foundDirs[nm2]; !ok || size != 20 {
		t.Errorf("Diretório %s não encontrado ou com tamanho incorreto. Encontrado: %t, Tamanho: %d", nm2, ok, size)
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
	var wg sync.WaitGroup

	// Inicia worker
	wg.Add(1)
	go deleteWorker(jobs, &wg)

	// Envia job para deletar o diretório
	jobs <- nodeModulesDir{testNM, 4}
	close(jobs)

	// Aguarda worker terminar
	wg.Wait()

	// Verifica se o diretório foi deletado
	if _, err := os.Stat(testNM); !os.IsNotExist(err) {
		t.Errorf("Diretório não foi deletado: %s", testNM)
	}
}

// TestIntegration executa um teste de integração simulando o fluxo completo de busca.
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

	// Cria vários níveis de diretórios com node_modules.
	// O diretório aninhado (em dir3/node_modules/pkg/node_modules) deve ser ignorado.
	topLevelNM1 := filepath.Join(tmpDir, "dir1", "node_modules")
	topLevelNM2 := filepath.Join(tmpDir, "dir2", "subdir", "node_modules")
	topLevelNM3 := filepath.Join(tmpDir, "dir3", "node_modules")
	nestedNM := filepath.Join(topLevelNM3, "pkg", "node_modules")

	dirsToCreate := []string{topLevelNM1, topLevelNM2, nestedNM}

	for _, dir := range dirsToCreate {
		err = os.MkdirAll(dir, 0755)
		if err != nil {
			t.Fatalf("Erro ao criar diretório de teste: %v", err)
		}
		// Cria um arquivo em cada diretório para ter um tamanho > 0
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
		go func() {
			defer wg.Done()
			for path := range jobs {
				size, err := dirSize(path)
				if err != nil {
					t.Errorf("dirSize failed for %s: %v", path, err)
				}
				results <- nodeModulesDir{path, size}
			}
		}()
	}

	// Roda a função de busca
	go func() {
		walkDirs(tmpDir, jobs)
		close(jobs)
	}()

	// Coleta resultados
	foundDirs := make(map[string]bool)
	go func() {
		wg.Wait()
		close(results)
	}()
	for dir := range results {
		foundDirs[dir.path] = true
	}

	// Verifica se encontrou o número correto de diretórios.
	// Apenas os 3 node_modules de nível superior devem ser encontrados.
	expectedCount := 3
	if len(foundDirs) != expectedCount {
		t.Errorf("Quantidade incorreta de node_modules encontrados: esperado %d, obtido %d", expectedCount, len(foundDirs))
		for path := range foundDirs {
			t.Logf("Encontrado: %s", path)
		}
	}

	// Verifica se os diretórios corretos foram encontrados
	if !foundDirs[topLevelNM1] {
		t.Errorf("Diretório esperado não encontrado: %s", topLevelNM1)
	}
	if !foundDirs[topLevelNM2] {
		t.Errorf("Diretório esperado não encontrado: %s", topLevelNM2)
	}
	if !foundDirs[topLevelNM3] {
		t.Errorf("Diretório esperado não encontrado: %s", topLevelNM3)
	}
	if foundDirs[nestedNM] {
		t.Errorf("Diretório aninhado foi encontrado indevidamente: %s", nestedNM)
	}
}

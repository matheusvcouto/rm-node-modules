package log

import (
	"fmt"
	"strings"
	"time"
)

// Centraliza o texto entre '=' de acordo com o tamanho do título
func Section(title string) {
	totalWidth := 40 // ajuste conforme preferir
	titleLen := len(title)
	side := (totalWidth - titleLen) / 2
	fmt.Println(strings.Repeat("=", totalWidth))
	fmt.Printf("%s%s%s\n", strings.Repeat(" ", side), title, strings.Repeat(" ", totalWidth-titleLen-side))
	fmt.Println(strings.Repeat("=", totalWidth))
}

// Cria um separador ajustado ao texto
func Separator(text string) {
	totalWidth := 40 // ajuste conforme preferir
	textLen := len(text)
	if textLen == 0 {
		fmt.Println(strings.Repeat("=", totalWidth))
		return
	}
	side := (totalWidth - textLen) / 2
	fmt.Printf("%s%s%s\n", strings.Repeat("=", side), text, strings.Repeat("=", totalWidth-textLen-side))
}

func Found(path string, size string) {
	fmt.Printf("[✔] Found: %s (%s)\n", path, size)
}

func Deleting(path string) {
	fmt.Printf("[⏳] Deletando: %s...\n", path)
}

func Deleted(path string, size string, elapsed time.Duration) {
	fmt.Printf("[✔] Deletado: %s (%s, took %s)\n", path, size, elapsed.Round(time.Millisecond))
}

func Error(path string, err error) {
	fmt.Printf("[✖] ERRO ao deletar %s: %v\n", path, err)
}

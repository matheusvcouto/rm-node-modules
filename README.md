# rm-node-modules

Este script em Go busca e apaga todas as pastas `node_modules` de um diretório e suas subpastas, de forma rápida, segura e eficiente.

## Como funciona

- Procura por todas as pastas `node_modules` (exceto as que estão dentro de outras `node_modules`).
- Mostra em tempo real cada pasta encontrada e o tamanho dela.
- Exibe o total de espaço ocupado e pede confirmação antes de apagar.
- Apaga as pastas em paralelo, mostrando em tempo real cada remoção.
- Usa goroutines e workers para ser rápido e não travar o computador (máximo 10 para busca, 5 para remoção).

## Como usar

1. Compile o programa:
   ```sh
   go build -o rm-nm.exe
   ```
2. Execute na pasta que deseja limpar:
   ```sh
   ./rm-nm.exe
   ```
3. Siga as instruções no terminal.

## Segurança

- O script nunca apaga nada sem pedir confirmação.
- Não entra em pastas `node_modules` dentro de outras `node_modules`.
- Usa boas práticas de concorrência e gerenciamento de memória.

## Requisitos

- Go 1.18 ou superior
- Windows, Linux ou Mac

---

> Use por sua conta e risco. Sempre confira o que será apagado antes de confirmar!

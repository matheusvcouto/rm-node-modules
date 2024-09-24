# rm_node_modules

This Rust project scans the current directory and its subdirectories for `node_modules` folders, calculates their total size, and prompts the user to delete them.

**pt-br:** Este projeto em Rust escaneia o diretório atual e suas subpastas em busca de pastas `node_modules`, calcula seu tamanho total e solicita ao usuário a exclusão delas.

## Compilação

Use o comando:

```bash
cargo build --release
```

## Configuração no Fish Shell

Adicione o caminho do binário ao seu `PATH` no Fish shell:

```fish
set -g PATH $PATH $HOME/scripts/rm_node_modules/target/release/
```

**Obs:** O binário é encontrado automaticamente e é usado o nome do projeto para executá-lo

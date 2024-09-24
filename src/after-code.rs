use std::fs;
use std::path::Path;

fn main() -> std::io::Result<()> {
    let current_dir = std::env::current_dir()?;
    delete_node_modules(&current_dir)?;
    println!("Processo concluído.");
    Ok(())
}

fn delete_node_modules(dir: &Path) -> std::io::Result<()> {
    if dir.is_dir() {
        for entry in fs::read_dir(dir)? {
            let entry = entry?;
            let path = entry.path();
            
            if path.is_dir() {
                if path.file_name().unwrap() == "node_modules" {
                    println!("Apagando: {}", path.display());
                    fs::remove_dir_all(&path)?;
                } else {
                    delete_node_modules(&path)?;
                }
            }
        }
    }
    Ok(())
}
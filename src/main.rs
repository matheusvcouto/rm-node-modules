use std::fs;
use std::io::{self, Write};
use std::path::Path;

struct DirStats {
    size: u64,
    count: u64,
}

fn main() -> std::io::Result<()> {
    let current_dir = std::env::current_dir()?;
    let home_dir = home::home_dir().unwrap();

    if current_dir != home_dir {
        let stats = calculate_node_modules_stats(&current_dir)?;

        println!("Found {} node_modules directories", stats.count);
        println!(
            "Total size: {:.2} MB / {:.2} GB",
            stats.size as f64 / 1_048_576.0,
            stats.size as f64 / 1_073_741_824.0
        );

        print!("Do you want to delete these directories? (y/n): ");
        io::stdout().flush()?;

        let mut input = String::new();
        io::stdin().read_line(&mut input)?;

        if input.trim().to_lowercase() == "y" {
            delete_node_modules(&current_dir)?;
            println!("Process completed. All node_modules directories have been deleted.");
        } else {
            println!("Operation cancelled. No directories were deleted.");
        }
    } else {
        println!("Cannot execute in the home directory. This operation is restricted for safety reasons to prevent accidental deletion of important files.");
    }

    Ok(())
}

fn calculate_node_modules_stats(dir: &Path) -> std::io::Result<DirStats> {
    let mut stats = DirStats { size: 0, count: 0 };

    if dir.is_dir() {
        for entry in fs::read_dir(dir)? {
            let entry = entry?;
            let path = entry.path();

            if path.is_dir() {
                if path.file_name().unwrap() == "node_modules" {
                    let dir_size = calculate_dir_size(&path)?;
                    stats.size += dir_size;
                    stats.count += 1;
                    println!(
                        "Found: {} ({:.2} MB)",
                        path.display(),
                        dir_size as f64 / 1_048_576.0
                    );
                } else {
                    let sub_stats = calculate_node_modules_stats(&path)?;
                    stats.size += sub_stats.size;
                    stats.count += sub_stats.count;
                }
            }
        }
    }
    Ok(stats)
}

fn calculate_dir_size(dir: &Path) -> std::io::Result<u64> {
    let mut total_size = 0;
    for entry in fs::read_dir(dir)? {
        let entry = entry?;
        let path = entry.path();
        if path.is_file() {
            total_size += fs::metadata(&path)?.len();
        } else if path.is_dir() {
            total_size += calculate_dir_size(&path)?;
        }
    }
    Ok(total_size)
}

fn delete_node_modules(dir: &Path) -> std::io::Result<()> {
    if dir.is_dir() {
        for entry in fs::read_dir(dir)? {
            let entry = entry?;
            let path = entry.path();

            if path.is_dir() {
                if path.file_name().unwrap() == "node_modules" {
                    println!("Deleting: {}", path.display());
                    fs::remove_dir_all(&path)?;
                } else {
                    delete_node_modules(&path)?;
                }
            }
        }
    }
    Ok(())
}

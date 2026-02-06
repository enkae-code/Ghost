// Author: Enkae (enkae.dev@pm.me)
use anyhow::{Context, Result};
use notify::{Config, Event, RecommendedWatcher, RecursiveMode, Watcher};
use serde::{Deserialize, Serialize};
use std::collections::HashMap;
use std::path::{Path, PathBuf};
use std::sync::mpsc::{channel, Receiver, Sender};
use std::sync::{Arc, Mutex};
use std::time::SystemTime;
use walkdir::WalkDir;

/// FileEntry represents an indexed file in the Librarian's memory
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct FileEntry {
    pub path: PathBuf,
    pub file_name: String,
    pub extension: Option<String>,
    pub size_bytes: u64,
    pub modified: SystemTime,
}

/// Librarian indexes and watches file system for semantic file search
pub struct Librarian {
    /// In-memory file index (path -> FileEntry)
    index: Arc<Mutex<HashMap<PathBuf, FileEntry>>>,
    /// Directories being watched
    watched_dirs: Vec<PathBuf>,
    /// API endpoint to submit file artifacts
    api_url: String,
}

impl Librarian {
    /// Create a new Librarian instance
    pub fn new(api_url: String) -> Self {
        Self {
            index: Arc::new(Mutex::new(HashMap::new())),
            watched_dirs: Vec::new(),
            api_url,
        }
    }

    /// Add a directory to watch and index
    pub fn watch_directory(&mut self, path: PathBuf) -> Result<()> {
        println!("[LIBRARIAN] Adding watch directory: {}", path.display());

        // Initial index of the directory
        self.index_directory(&path)?;

        self.watched_dirs.push(path);
        Ok(())
    }

    /// Index all files in a directory recursively
    pub fn index_directory(&self, path: &Path) -> Result<()> {
        println!("[LIBRARIAN] Indexing directory: {}", path.display());

        let mut count = 0;

        for entry in WalkDir::new(path)
            .follow_links(false)
            .into_iter()
            .filter_map(|e| e.ok())
        {
            // Skip directories, only index files
            if !entry.file_type().is_file() {
                continue;
            }

            let path = entry.path().to_path_buf();

            // Skip hidden files and common exclusions
            if self.should_skip(&path) {
                continue;
            }

            match self.create_file_entry(&path) {
                Ok(file_entry) => {
                    let mut index = self.index.lock().unwrap();
                    index.insert(path.clone(), file_entry.clone());
                    count += 1;

                    // Send to Go backend as artifact
                    if let Err(e) = self.send_file_artifact(&file_entry) {
                        eprintln!("[LIBRARIAN] Failed to send artifact: {}", e);
                    }
                }
                Err(e) => {
                    eprintln!("[LIBRARIAN] Failed to index {}: {}", path.display(), e);
                }
            }
        }

        println!("[LIBRARIAN] Indexed {} files from {}", count, path.display());
        Ok(())
    }

    /// Create a FileEntry from a path
    fn create_file_entry(&self, path: &Path) -> Result<FileEntry> {
        let metadata = std::fs::metadata(path)
            .context("Failed to read file metadata")?;

        let file_name = path
            .file_name()
            .and_then(|n| n.to_str())
            .unwrap_or("unknown")
            .to_string();

        let extension = path
            .extension()
            .and_then(|e| e.to_str())
            .map(|s| s.to_string());

        Ok(FileEntry {
            path: path.to_path_buf(),
            file_name,
            extension,
            size_bytes: metadata.len(),
            modified: metadata.modified().unwrap_or(SystemTime::now()),
        })
    }

    /// Check if a file should be skipped during indexing
    fn should_skip(&self, path: &Path) -> bool {
        let path_str = path.to_string_lossy().to_lowercase();

        // Skip hidden files (starting with .)
        if let Some(file_name) = path.file_name() {
            if file_name.to_string_lossy().starts_with('.') {
                return true;
            }
        }

        // Skip common system/build directories
        let skip_dirs = [
            "node_modules",
            "target",
            ".git",
            ".vscode",
            "dist",
            "build",
            "__pycache__",
            ".next",
            ".cache",
        ];

        for skip_dir in &skip_dirs {
            if path_str.contains(skip_dir) {
                return true;
            }
        }

        // Skip very large files (> 100MB)
        if let Ok(metadata) = std::fs::metadata(path) {
            if metadata.len() > 100 * 1024 * 1024 {
                return true;
            }
        }

        false
    }

    /// Send a file entry to the Go backend as an artifact
    fn send_file_artifact(&self, file_entry: &FileEntry) -> Result<()> {
        let artifact_type = match file_entry.extension.as_deref() {
            Some("pdf") | Some("docx") | Some("txt") | Some("md") => "DOCUMENT",
            Some("jpg") | Some("png") | Some("gif") | Some("bmp") => "IMAGE",
            Some("mp3") | Some("wav") | Some("flac") => "AUDIO",
            Some("mp4") | Some("avi") | Some("mkv") => "VIDEO",
            Some("zip") | Some("rar") | Some("7z") => "ARCHIVE",
            Some("exe") | Some("msi") | Some("app") => "EXECUTABLE",
            Some("sav") | Some("dat") | Some("save") => "GAME_SAVE",
            _ => "FILE",
        };

        // Create artifact content with full path for RAG
        let content = format!(
            "{} | {} | {}",
            file_entry.path.display(),
            file_entry.file_name,
            artifact_type
        );

        let payload = serde_json::json!({
            "type": artifact_type,
            "content": content,
            "metadata": {
                "file_path": file_entry.path.display().to_string(),
                "file_name": file_entry.file_name,
                "extension": file_entry.extension,
                "size_bytes": file_entry.size_bytes,
            }
        });

        // Send to Go backend
        let client = reqwest::blocking::Client::new();
        client
            .post(&self.api_url)
            .json(&payload)
            .send()
            .context("Failed to send artifact to Go backend")?;

        Ok(())
    }

    /// Start watching directories for file system changes
    pub fn start_watching(self) -> Result<()> {
        let index = Arc::clone(&self.index);
        let api_url = self.api_url.clone();

        // Create channel for file system events
        let (tx, rx): (Sender<Result<Event, notify::Error>>, Receiver<Result<Event, notify::Error>>) = channel();

        // Create watcher
        let mut watcher = RecommendedWatcher::new(
            move |res| {
                let _ = tx.send(res);
            },
            Config::default(),
        )?;

        // Watch all directories
        for dir in &self.watched_dirs {
            watcher.watch(dir, RecursiveMode::Recursive)?;
            println!("[LIBRARIAN] Watching: {}", dir.display());
        }

        println!("[LIBRARIAN] File system watcher started");

        // Process events
        loop {
            match rx.recv() {
                Ok(Ok(event)) => {
                    Self::handle_fs_event(&index, &api_url, event);
                }
                Ok(Err(e)) => {
                    eprintln!("[LIBRARIAN] Watch error: {}", e);
                }
                Err(e) => {
                    eprintln!("[LIBRARIAN] Channel error: {}", e);
                    break;
                }
            }
        }

        Ok(())
    }

    /// Handle file system events
    fn handle_fs_event(
        index: &Arc<Mutex<HashMap<PathBuf, FileEntry>>>,
        api_url: &str,
        event: Event,
    ) {
        use notify::EventKind;

        match event.kind {
            EventKind::Create(_) | EventKind::Modify(_) => {
                for path in event.paths {
                    if path.is_file() && !Self::should_skip_static(&path) {
                        // Re-index the file
                        if let Ok(file_entry) = Self::create_file_entry_static(&path) {
                            let mut idx = index.lock().unwrap();
                            idx.insert(path.clone(), file_entry.clone());
                            drop(idx);

                            println!("[LIBRARIAN] Indexed: {}", path.display());

                            // Send to backend
                            if let Err(e) = Self::send_file_artifact_static(api_url, &file_entry) {
                                eprintln!("[LIBRARIAN] Failed to send artifact: {}", e);
                            }
                        }
                    }
                }
            }
            EventKind::Remove(_) => {
                for path in event.paths {
                    let mut idx = index.lock().unwrap();
                    idx.remove(&path);
                    println!("[LIBRARIAN] Removed from index: {}", path.display());
                }
            }
            _ => {}
        }
    }

    // Static versions of methods for use in event handler
    fn should_skip_static(path: &Path) -> bool {
        let path_str = path.to_string_lossy().to_lowercase();

        if let Some(file_name) = path.file_name() {
            if file_name.to_string_lossy().starts_with('.') {
                return true;
            }
        }

        let skip_dirs = [
            "node_modules", "target", ".git", ".vscode", "dist",
            "build", "__pycache__", ".next", ".cache",
        ];

        for skip_dir in &skip_dirs {
            if path_str.contains(skip_dir) {
                return true;
            }
        }

        if let Ok(metadata) = std::fs::metadata(path) {
            if metadata.len() > 100 * 1024 * 1024 {
                return true;
            }
        }

        false
    }

    fn create_file_entry_static(path: &Path) -> Result<FileEntry> {
        let metadata = std::fs::metadata(path)?;

        let file_name = path
            .file_name()
            .and_then(|n| n.to_str())
            .unwrap_or("unknown")
            .to_string();

        let extension = path
            .extension()
            .and_then(|e| e.to_str())
            .map(|s| s.to_string());

        Ok(FileEntry {
            path: path.to_path_buf(),
            file_name,
            extension,
            size_bytes: metadata.len(),
            modified: metadata.modified().unwrap_or(SystemTime::now()),
        })
    }

    fn send_file_artifact_static(api_url: &str, file_entry: &FileEntry) -> Result<()> {
        let artifact_type = match file_entry.extension.as_deref() {
            Some("pdf") | Some("docx") | Some("txt") | Some("md") => "DOCUMENT",
            Some("jpg") | Some("png") | Some("gif") | Some("bmp") => "IMAGE",
            Some("mp3") | Some("wav") | Some("flac") => "AUDIO",
            Some("mp4") | Some("avi") | Some("mkv") => "VIDEO",
            Some("zip") | Some("rar") | Some("7z") => "ARCHIVE",
            Some("exe") | Some("msi") | Some("app") => "EXECUTABLE",
            Some("sav") | Some("dat") | Some("save") => "GAME_SAVE",
            _ => "FILE",
        };

        let content = format!(
            "{} | {} | {}",
            file_entry.path.display(),
            file_entry.file_name,
            artifact_type
        );

        let payload = serde_json::json!({
            "type": artifact_type,
            "content": content,
            "metadata": {
                "file_path": file_entry.path.display().to_string(),
                "file_name": file_entry.file_name,
                "extension": file_entry.extension,
                "size_bytes": file_entry.size_bytes,
            }
        });

        let client = reqwest::blocking::Client::new();
        client.post(api_url).json(&payload).send()?;

        Ok(())
    }
}

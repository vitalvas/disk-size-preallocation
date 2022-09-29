package main

import (
	"crypto/rand"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"path"
	"path/filepath"
	"sort"
	"syscall"
	"time"
)

const (
	KB = uint64(1024)
	MB = uint64(1024 * 1024)
	GB = uint64(1024 * 1024 * 1024)
)

func main() {
	var size = flag.Int("size", 256, "minimum disk usage in GB")
	var path = flag.String("path", "/mnt", "path to disk")
	var random = flag.Bool("random", false, "allocate with random data")

	flag.Parse()

	diskUsed := getDiskSize(*path)
	newChunks := *size - int(diskUsed)

	if newChunks > 0 {
		log.Printf("Try to allocate %d files of 1GB", newChunks)

		for x := 0; x < newChunks; x++ {
			ts := time.Now()
			if err := allocateChunk(*path, *random); err != nil {
				log.Fatal(err)
			}

			since := time.Since(ts)
			log.Printf("File #%d: %s", x+1, since.String())

			if since > 10*time.Minute {
				log.Println("Disk too slow.")
				return
			}
		}
	} else if newChunks < 0 {
		newChunks = newChunks * -1
		log.Printf("Try to delete %d files of 1GB", newChunks)

		for x := 0; x < newChunks; x++ {
			ts := time.Now()
			if err := deleteChunk(*path); err != nil {
				log.Fatal(err)
			}

			since := time.Since(ts)
			log.Printf("File #%d: %s", x+1, since.String())

			if since > 10*time.Minute {
				log.Println("Disk too slow.")
				return
			}
		}
	}
}

func allocateChunk(volumePath string, random bool) error {
	dir := filepath.Join(volumePath, ".preallocation")

	if stat, err := os.Stat(dir); err != nil {
		if err := os.Mkdir(dir, 0755); err != nil {
			return err
		}

	} else if !stat.IsDir() {
		return errors.New("error create preallocation directory")
	}

	filePath := filepath.Join(dir, fmt.Sprintf("%x", time.Now().UnixNano()))

	file, err := os.Create(filePath)
	if err != nil {
		return err
	}

	defer file.Close()

	for x := 0; x < 1024; x++ {
		data := make([]byte, MB)
		if random {
			if _, err := rand.Read(data); err != nil {
				return err
			}
		}

		if _, err := file.Write(data); err != nil {
			return err
		}
	}

	return file.Sync()
}

func deleteChunk(volumePath string) error {
	dir := filepath.Join(volumePath, ".preallocation")

	if stat, err := os.Stat(dir); err != nil {
		return errors.New("preallocation directory not found")

	} else if !stat.IsDir() {
		return errors.New("preallocation directory is not a directory")
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}

	var files []string

	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil {
			return err
		}

		if !info.IsDir() {
			if uint64(info.Size()) >= (GB - 128*MB) {
				files = append(files, info.Name())
			}
		}
	}
	if len(files) == 0 {
		log.Println("no files to delete")
		return nil
	}

	sort.Strings(files)

	filePath := path.Join(dir, files[len(files)-1])

	if err := os.Remove(filePath); err != nil {
		return err
	}

	return nil
}

// size in GB
func getDiskSize(volumePath string) uint64 {
	var stat syscall.Statfs_t

	if err := syscall.Statfs(volumePath, &stat); err != nil {
		log.Fatal(err)
	}

	size := stat.Blocks * uint64(stat.Bsize)
	free := stat.Bfree * uint64(stat.Bsize)

	used := size - free

	return used / GB
}

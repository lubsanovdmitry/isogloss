// Command datagen downloads the source datasets (PHOIBLE, Concepticon, CLICS,
// WALS) into a cache directory and derives the compact tables embedded in
// engine/data. The engine never imports this package and never touches the
// network; generated files are committed.
package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
)

var (
	outDir   = flag.String("out", "engine/data", "output directory for generated tables")
	cacheDir = flag.String("cache", defaultCache(), "download cache directory")
	refresh  = flag.Bool("refresh", false, "re-download sources even if cached")
)

func defaultCache() string {
	d, err := os.UserCacheDir()
	if err != nil {
		return ".cache"
	}
	return filepath.Join(d, "isogloss")
}

func main() {
	flag.Parse()
	if err := os.MkdirAll(*cacheDir, 0o755); err != nil {
		log.Fatal(err)
	}
	if err := genPhoible(); err != nil {
		log.Fatal("phoible: ", err)
	}
	concepts, err := genConcepts()
	if err != nil {
		log.Fatal("concepts: ", err)
	}
	if err := genCLICS(concepts); err != nil {
		log.Fatal("clics: ", err)
	}
	if err := genWALS(); err != nil {
		log.Fatal("wals: ", err)
	}
	if err := genRomance(); err != nil {
		log.Fatal("romance: ", err)
	}
}

// fetch returns the local path of url, downloading it into the cache if needed.
func fetch(name, url string) (string, error) {
	p := filepath.Join(*cacheDir, name)
	if _, err := os.Stat(p); err == nil && !*refresh {
		return p, nil
	}
	log.Printf("fetching %s", url)
	resp, err := http.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("%s: %s", url, resp.Status)
	}
	tmp := p + ".part"
	f, err := os.Create(tmp)
	if err != nil {
		return "", err
	}
	if _, err := io.Copy(f, resp.Body); err != nil {
		f.Close()
		return "", err
	}
	if err := f.Close(); err != nil {
		return "", err
	}
	return p, os.Rename(tmp, p)
}

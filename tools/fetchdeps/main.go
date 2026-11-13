// Command fetchdeps downloads the DuckDB files that the build needs and that
// are not Go modules: the official extensions embedded in the binary and, on
// Windows, the DuckDB library itself.
//
// It runs at build time, from the Makefile. Nothing is downloaded when the
// program runs. Every file is pinned by its SHA-256 below, so a different file
// at the same address is rejected.
//
// To move to another DuckDB version, change duckdbVersion and the checksums
// together with the duckdb-go module in go.mod: an extension only loads into
// the exact DuckDB version it was built for.
package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"time"
)

const (
	duckdbVersion = "v2.0.0"

	// extensionsDir is embedded by internal/engine.
	extensionsDir = "internal/engine/extensions"
	// libraryDir is where the Windows build looks for duckdb.dll, to link
	// against it and to run next to it.
	libraryDir = "bin"

	extensionURL = "https://extensions.duckdb.org/" + duckdbVersion + "/%s/%s.duckdb_extension.gz"
	libraryURL   = "https://github.com/duckdb/duckdb/releases/download/" + duckdbVersion + "/duckdb-shared-libs-windows-amd64.tar.gz"
)

// platform describes what one GOOS/GOARCH needs.
type platform struct {
	// duckdb is the name DuckDB gives the platform in extension addresses.
	duckdb string
	// extensions maps an extension to the SHA-256 of its .gz file.
	extensions map[string]string
	// library is the SHA-256 of the archive with the DuckDB library, on the
	// platforms that link to it dynamically.
	library string
}

var platforms = map[string]platform{
	"linux/amd64": {duckdb: "linux_amd64", extensions: map[string]string{
		"excel":            "40a170558c7110eed04b631551f9ac13f854747a293986612e9922b6044b3821",
		"postgres_scanner": "54b3d751cafc9ffa439886b1607d0ee758dab4dc070f0171e779b95f2f3e554f",
	}},
	"linux/arm64": {duckdb: "linux_arm64", extensions: map[string]string{
		"excel":            "c404d79e9ad928681db6ccf9dd73723922b02fc1c41a0c4c770baeef8f29911f",
		"postgres_scanner": "afe7f2bd798b61e4c2e1f96deedaa70a21d15eebc8cd9470beb923951d0ce58a",
	}},
	"darwin/amd64": {duckdb: "osx_amd64", extensions: map[string]string{
		"excel":            "956e419e082034f54a9b535f7d35aa595a19580bb79a479279f9d588f7e2b847",
		"postgres_scanner": "be095bd7b33b970467d09ac0ab8f84ba2e75b2368857a3b07eab6f5096c20a93",
	}},
	"darwin/arm64": {duckdb: "osx_arm64", extensions: map[string]string{
		"excel":            "a9e38c4bf55bd482073f2d40c3a0c1334fadfa819ca1b7a0eb599a38936c4fd1",
		"postgres_scanner": "496a338695582a99db2826597affc03307a4d7ed7b92c972e261adfe40f03fe3",
	}},
	// Windows links to duckdb.dll: the static library DuckDB publishes for Go
	// on Windows is a MinGW build, and there is no Postgres extension for it.
	"windows/amd64": {duckdb: "windows_amd64", library: "2b83169128239de5f45fe21de80f99bf436e53aa4bb24020dd951a1ba8d4b95f", extensions: map[string]string{
		"excel":            "e524d6f66e8d7b546c1038bf402241dc18bd556462f3de677c94f05ceb3a61ec",
		"postgres_scanner": "9b04ad866fd877836342f88913fe008b7e56bd2903dd013a5067f6becaf05d2b",
	}},
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "fetchdeps:", err)
		os.Exit(1)
	}
}

func run() error {
	target := runtime.GOOS + "/" + runtime.GOARCH
	p, ok := platforms[target]
	if !ok {
		return fmt.Errorf("no DuckDB files are pinned for %s", target)
	}

	for name, sum := range p.extensions {
		file := filepath.Join(extensionsDir, name+".duckdb_extension.gz")
		if err := fetch(fmt.Sprintf(extensionURL, p.duckdb, name), sum, file, ""); err != nil {
			return err
		}
	}
	if p.library != "" {
		if err := fetch(libraryURL, p.library, filepath.Join(libraryDir, "duckdb.dll"), "duckdb.dll"); err != nil {
			return err
		}
	}
	return nil
}

// fetch makes sure file holds what is at url, whose SHA-256 must be sum. When
// member is set, url is a .tar.gz and file receives that member of it.
//
// A marker next to the file records the checksum it came from, so a build
// downloads again only after the pinned version changes.
func fetch(url, sum, file, member string) error {
	marker := file + ".sha256"
	if recorded, err := os.ReadFile(marker); err == nil && string(recorded) == sum {
		if _, err := os.Stat(file); err == nil {
			return nil
		}
	}

	fmt.Println("fetchdeps: downloading", url)
	client := &http.Client{Timeout: 10 * time.Minute}
	resp, err := client.Get(url)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%s: %s", url, resp.Status)
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("%s: %w", url, err)
	}
	if got := sha256.Sum256(data); hex.EncodeToString(got[:]) != sum {
		return fmt.Errorf("%s: checksum is %x, expected %s; refusing to use the file", url, got, sum)
	}

	if member != "" {
		if data, err = extract(data, member); err != nil {
			return fmt.Errorf("%s: %w", url, err)
		}
	}
	if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(file, data, 0o755); err != nil {
		return err
	}
	return os.WriteFile(marker, []byte(sum), 0o644)
}

// extract returns one file of a .tar.gz archive.
func extract(archive []byte, member string) ([]byte, error) {
	unzipped, err := gzip.NewReader(bytes.NewReader(archive))
	if err != nil {
		return nil, err
	}
	files := tar.NewReader(unzipped)
	for {
		header, err := files.Next()
		if errors.Is(err, io.EOF) {
			return nil, fmt.Errorf("%s is not in the archive", member)
		}
		if err != nil {
			return nil, err
		}
		if filepath.Base(header.Name) == member {
			return io.ReadAll(files)
		}
	}
}

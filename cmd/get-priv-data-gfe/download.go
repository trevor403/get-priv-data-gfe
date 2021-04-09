package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"path"
	"sort"

	"io"

	"github.com/gen2brain/go-unarr"
	"gopkg.in/yaml.v3"

	"log"
	"os"
	"path/filepath"
)

const dllname = "_nvspcaps64.dll"

var szfinder = []byte("!@InstallEnd@!")
var szmagic = []byte("7z")

func scanFile(f io.ReadSeeker, search []byte, count int) int64 {
	ix := 0
	r := bufio.NewReader(f)
	offset := int64(0)
	for nth := 0; nth < count; nth++ {
		for ix < len(search) {
			b, err := r.ReadByte()
			if err != nil {
				return -1
			}
			if search[ix] == b {
				ix++
			} else {
				ix = 0
			}
			offset++
		}
		ix = 0
	}
	f.Seek(offset, os.SEEK_SET)
	return offset
}

func seekTo7z(file io.ReadSeeker) error {
	offset := scanFile(file, szfinder, 2)

	buf := make([]byte, 2)
	var skip int64
	for skip = 0; skip < 0x10; skip += 0x2 {
		_, err := file.Read(buf)
		if err != nil {
			log.Fatal(err)
		} else if bytes.Equal(buf, szmagic) {
			break
		}
	}
	offset += skip
	file.Seek(offset, os.SEEK_SET)

	return nil
}

func createCacheDir() (string, error) {
	cacheDir, err := os.UserCacheDir()
	if err != nil {
		return "", fmt.Errorf("get os.UserCacheDir failed: %s", err)
	}

	execName := filepath.Base(os.Args[0])
	cacheDir = filepath.Join(cacheDir, execName)
	err = os.MkdirAll(cacheDir, 0755)
	if err != nil {
		return "", fmt.Errorf("create dir in os.UserCacheDir failed: %s", err)
	}

	return cacheDir, nil
}

func pickVersion(repoContents []GitHubRepoContents) string {
	fileNames := make([]string, len(repoContents))
	for i, contents := range repoContents {
		fileNames[i] = contents.Name
	}

	sort.Strings(fileNames)
	newest := fileNames[len(fileNames)-1]

	return newest
}

func pickDownloadURL(repoContents []GitHubRepoContents) string {
	return repoContents[0].DownloadURL
}

func getInstaller(gfePath string) error {
	const wingetURL = "https://api.github.com/repos/microsoft/winget-pkgs/contents/manifests/n/Nvidia/GeForceExperience"

	// get github folder contents
	resp, err := http.Get(wingetURL)
	if err != nil {
		return err
	}
	b, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	resp.Body.Close()

	// unmarshal json
	var repoContents []GitHubRepoContents
	err = json.Unmarshal(b, &repoContents)
	if err != nil {
		return err
	} else if len(repoContents) == 0 {
		return fmt.Errorf("too few Contents entries")
	}

	version := pickVersion(repoContents)

	// get github folder contents
	resp, err = http.Get(fmt.Sprintf("%s/%s", wingetURL, version))
	if err != nil {
		return err
	}
	b, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	resp.Body.Close()

	// unmarshal json
	err = json.Unmarshal(b, &repoContents)
	if err != nil {
		return err
	} else if len(repoContents) == 0 {
		return fmt.Errorf("too few Contents entries")
	}

	downloadURL := pickDownloadURL(repoContents)

	// get winpkg yaml
	resp, err = http.Get(downloadURL)
	if err != nil {
		return err
	}
	b, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	resp.Body.Close()

	// unmarshal yaml
	var winget WinGetPkg
	err = yaml.Unmarshal(b, &winget)
	if err != nil {
		return err
	} else if len(winget.Installers) == 0 {
		return fmt.Errorf("too few Intaller entries")
	}

	// get GeForceNow exe
	resp, err = http.Get(winget.Installers[0].InstallerURL)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	out, err := os.Create(gfePath)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	if err != nil {
		return err
	}

	return nil
}

func getArchive(gfePath, szPath string) error {
	file, err := os.Open(gfePath)
	if err != nil {
		return err
	}
	defer file.Close()

	err = seekTo7z(file)
	if err != nil {
		return err
	}

	out, err := os.Create(szPath)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, file)
	if err != nil {
		return err
	}

	return nil
}

func getDll(szPath, dllPath string) error {
	a, err := unarr.NewArchive(szPath)
	if err != nil {
		return err
	}
	defer a.Close()

	list, err := a.List()
	if err != nil {
		return err
	}

	var archivePath string
	for _, archivePath = range list {
		if path.Base(archivePath) == dllname {
			break
		}
	}

	err = a.EntryFor(archivePath)
	if err != nil {
		return err
	}

	data, err := a.ReadAll()
	if err != nil {
		return err
	}

	err = ioutil.WriteFile(dllPath, data, 0666)
	if err != nil {
		return err
	}

	return nil
}

func getDownloadDllPath() (string, error) {
	cacheDir, err := createCacheDir()
	if err != nil {
		return "", err
	}

	gfePath := filepath.Join(cacheDir, "gfe.exe")
	if _, err := os.Stat(gfePath); err != nil && os.IsNotExist(err) {
		log.Printf("Downloading Geforce Experience installer to %s", gfePath)
		err = getInstaller(gfePath)
		if err != nil {
			return "", err
		}
	}

	szPath := filepath.Join(cacheDir, "gfe.7z")
	if _, err := os.Stat(szPath); err != nil && os.IsNotExist(err) {
		err = getArchive(gfePath, szPath)
		if err != nil {
			return "", err
		}
	}

	dllPath := filepath.Join(cacheDir, dllname)
	if _, err := os.Stat(dllPath); err != nil && os.IsNotExist(err) {
		log.Printf("Extracting dll from Geforce Experience to %s", dllPath)
		err = getDll(szPath, dllPath)
		if err != nil {
			return "", err
		}
	}

	return dllPath, nil
}

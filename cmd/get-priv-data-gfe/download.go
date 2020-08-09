package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"sort"

	"io"

	"github.com/itchio/boar/szextractor"
	"github.com/itchio/headway/state"
	"github.com/itchio/sevenzip-go/sz"
	"gopkg.in/yaml.v2"

	"log"
	"os"
	"path"
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

func pickDownloadURL(repoContents []GitHubRepoContents) string {
	fileNames := make([]string, len(repoContents))
	for i, contents := range repoContents {
		fileNames[i] = contents.Name
	}

	sort.Strings(fileNames)
	newest := fileNames[len(fileNames)-1]

	for _, contents := range repoContents {
		if contents.Name == newest {
			return contents.DownloadURL
		}
	}

	return repoContents[0].DownloadURL
}

func getInstaller(gfePath string) error {
	// get github folder contents
	resp, err := http.Get("https://api.github.com/repos/microsoft/winget-pkgs/contents/manifests/Nvidia/GeForceExperience")
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
	resp, err = http.Get(winget.Installers[0].URL)
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
	file, err := os.Open(szPath)
	if err != nil {
		return err
	}
	defer file.Close()

	stats, err := file.Stat()
	if err != nil {
		return err
	}
	size := stats.Size()

	// lib, err := sz.NewLib()
	consumer := &state.Consumer{}
	lib, err := szextractor.GetLib(consumer)
	if err != nil {
		return err
	}

	defer lib.Free()

	is, err := sz.NewInStream(file, "7z", size)
	if err != nil {
		return err
	}

	is.Stats = &sz.ReadStats{}

	a, err := lib.OpenArchive(is, true)
	if err != nil {
		return err
	}

	itemCount, err := a.GetItemCount()
	if err != nil {
		return err
	}

	var item *sz.Item
	for i := int64(0); i < itemCount; i++ {
		item = a.GetItem(i)
		s, _ := item.GetStringProperty(sz.PidPath)
		if path.Base(s) == dllname {
			break
		}
		item.Free()
		item = nil
	}
	if item == nil {
		return fmt.Errorf("could not find %s in archive", dllname)
	}
	defer item.Free()

	out, err := os.Create(dllPath)
	if err != nil {
		return err
	}
	defer out.Close()

	outstream, err := sz.NewOutStream(out)
	if err != nil {
		return err
	}

	err = a.Extract(item, outstream)
	if err != nil {
		return err
	}

	err = outstream.Close()
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

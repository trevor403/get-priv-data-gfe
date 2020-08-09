package main

type GitHubRepoContents struct {
	Name        string `json:"name"`
	Path        string `json:"path"`
	Sha         string `json:"sha"`
	Size        int    `json:"size"`
	URL         string `json:"url"`
	HTMLURL     string `json:"html_url"`
	GitURL      string `json:"git_url"`
	DownloadURL string `json:"download_url"`
	Type        string `json:"type"`
}

type WinGetPkg struct {
	ID           string `yaml:"Id"`
	Publisher    string `yaml:"Publisher"`
	Name         string `yaml:"Name"`
	Author       string `yaml:"Author"`
	Description  string `yaml:"Description"`
	AppMoniker   string `yaml:"AppMoniker"`
	Tags         string `yaml:"Tags"`
	Homepage     string `yaml:"Homepage"`
	License      string `yaml:"License"`
	LicenseURL   string `yaml:"LicenseUrl"`
	MinOSVersion string `yaml:"MinOSVersion"`
	Version      string `yaml:"Version"`
	Installers   []struct {
		Arch   string `yaml:"Arch"`
		URL    string `yaml:"Url"`
		Sha256 string `yaml:"Sha256"`
		Scope  string `yaml:"Scope"`
	} `yaml:"Installers"`
}

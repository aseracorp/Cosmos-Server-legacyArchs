package main

import (
    "encoding/json"
    "fmt"
    "io"
    "os"
    "net/http"
    "path/filepath"
    "bufio"
    "strings"
    "syscall"

    "github.com/azukaar/cosmos-server/src/utils"
)

// BuildArch is injected at build time via -ldflags "-X main.BuildArch=armv6|armv7|386|riscv64|ppc64le"
// It tells the update logic which architecture this binary was built for so it
// can pick the matching release asset.
var BuildArch = ""

// releaseAssetSuffixes maps the Go GOARCH/GOARM runtime variants to the suffix
// used in the published zip asset names (e.g. "...-armv6.zip", "...-386.zip").
func archAssetSuffix(arch string) string {
    switch arch {
    case "386":
        return "386"
    case "amd64":
        return "amd64"
    case "arm":
        return "arm"
    case "arm64":
        return "arm64"
    case "armv6":
        return "armv6"
    case "armv7":
        return "armv7"
    case "mips":
        return "mips"
    case "mipsle":
        return "mipsle"
    case "mips64":
        return "mips64"
    case "mips64le":
        return "mips64le"
    case "ppc64le":
        return "ppc64le"
    case "riscv64":
        return "riscv64"
    case "s390x":
        return "s390x"
    default:
        return arch
    }
}

type ReleaseAsset struct {
    Name        string `json:"name"`
    DownloadURL string `json:"browser_download_url"`
}

func (a *ReleaseAsset) UnmarshalJSON(data []byte) error {
    var raw struct {
        Name               string `json:"name"`
        BrowserDownloadURL string `json:"browser_download_url"`
        DownloadURL        string `json:"download_url"`
    }
    if err := json.Unmarshal(data, &raw); err != nil {
        return err
    }
    a.Name = raw.Name
    a.DownloadURL = raw.BrowserDownloadURL
    if a.DownloadURL == "" {
        a.DownloadURL = raw.DownloadURL
    }
    return nil
}

type Release struct {
    TagName string         `json:"tag_name"`
    PreRelease bool        `json:"prerelease"`
    Assets  []ReleaseAsset `json:"assets"`
}

type VersionInfo struct {
    Version string
    // Maps an asset suffix (e.g. "armv6", "386", "riscv64") to the corresponding
    // download URL or MD5 hash for that architecture.
    URLs map[string]string
    MD5s map[string]string
}

func parseMD5File(content string) string {
	// Read first line only (there should only be one anyway)
	scanner := bufio.NewScanner(strings.NewReader(content))
	if scanner.Scan() {
			// Split by whitespace and take first part (the hash)
			parts := strings.Fields(scanner.Text())
			if len(parts) > 0 {
					return parts[0]
			}
	}
	return ""
}

// detectAssetArch extracts the trailing architecture suffix (e.g. "-386",
// "-armv7") from a release asset name, or "" if none is found.
func detectAssetArch(name string) string {
    wanted := []string{
        "mips64le","mips64","mipsle","mips",
        "riscv64","ppc64le","s390x","armv7","armv6",
        "arm64","amd64","386","arm",
    }
    for _, a := range wanted {
        if strings.Contains(name, "-"+a) {
            return a
        }
    }
    return ""
}

func GetLatestVersion(includeBeta bool) (*VersionInfo, error) {
    // Fetch releases from GitHub API

    updateURL := "https://api.github.com/repos/aseracorp/Cosmos-Server-legacyArchs/releases"
    if utils.IsPro() {
        updateURL = "https://api.cosmos-cloud.io/proupdates"
    }

    resp, err := http.Get(updateURL)
    if err != nil {
        return nil, fmt.Errorf("failed to fetch releases: %v", err)
    }
    defer resp.Body.Close()

    body, err := io.ReadAll(resp.Body)
    if err != nil {
        return nil, fmt.Errorf("failed to read response body: %v", err)
    }

    var releases []Release
    if utils.IsPro() {
        var wrapper struct {
            Releases []Release `json:"releases"`
        }
        if err := json.Unmarshal(body, &wrapper); err != nil {
            return nil, fmt.Errorf("failed to parse JSON: %v", err)
        }
        releases = wrapper.Releases
    } else {
        if err := json.Unmarshal(body, &releases); err != nil {
            return nil, fmt.Errorf("failed to parse JSON: %v", err)
        }
    }

    // Find latest release that matches criteria
    var latestRelease *Release
    for _, release := range releases {
        if !includeBeta && release.PreRelease {
            continue
        }
        latestRelease = &release
        break
    }

    if latestRelease == nil {
        return nil, nil
    }

    // Initialize version info
    info := &VersionInfo{
        Version: latestRelease.TagName,
        URLs:    map[string]string{},
        MD5s:    map[string]string{},
    }

    // Parse assets to find URLs and MD5s
    for _, asset := range latestRelease.Assets {
        lower := strings.ToLower(asset.Name)

        // Skip irrelevant artifacts
        if strings.Contains(lower, "terraform") {
            continue
        }

        // A given asset is arch-specific if it carries an arch suffix.
        arch := detectAssetArch(lower)

        if strings.HasSuffix(lower, ".md5") {
            if arch == "" {
                continue
            }
            // Fetch the MD5 content for this architecture
            md5Resp, err := http.Get(asset.DownloadURL)
            if err != nil {
                continue
            }
            md5Content, err := io.ReadAll(md5Resp.Body)
            md5Resp.Body.Close()
            if err != nil {
                continue
            }
            info.MD5s[arch] = parseMD5File(strings.TrimSpace(string(md5Content)))
        } else if arch != "" {
            // Regular zip asset for this architecture
            info.URLs[arch] = asset.DownloadURL
        }
    }

    return info, nil
}

// ArchUpdateURL returns the download URL for the current runtime architecture.
func (v *VersionInfo) ArchUpdateURL() string {
    if v == nil || v.URLs == nil {
        return ""
    }
    return v.URLs[archAssetSuffix(BuildArch)]
}

// ArchMD5 returns the MD5 hash for the current runtime architecture.
func (v *VersionInfo) ArchMD5() string {
    if v == nil || v.MD5s == nil {
        return ""
    }
    return v.MD5s[archAssetSuffix(BuildArch)]
}

func cleanUpUpdateFiles() {
    execPath, err := os.Executable()
    if err != nil {
        return
    }

    currentFolder := filepath.Dir(execPath)

    dlPath := currentFolder + "/cosmos-update.zip"

    if _, err := os.Stat(dlPath); err == nil {
        os.Remove(dlPath)
    }

    // if cosmos-launcher.updated exists, rename it to cosmos-launcher
    updatedPath := currentFolder + "/cosmos-launcher.updated"
    if _, err := os.Stat(updatedPath); err == nil {
        // get old permissions
        var perms os.FileMode
        if info, err := os.Stat(currentFolder + "/cosmos-launcher"); err == nil {
            perms = info.Mode()
        } else {
            fmt.Println("Update: Failed to get old permissions:", err)
        }
        // get old owner
        var owner int
        if info, err := os.Stat(currentFolder + "/cosmos-launcher"); err == nil {
            owner = int(info.Sys().(*syscall.Stat_t).Uid)
        } else {
            fmt.Println("Update: Failed to get old owner:", err)
        }

        err := os.Rename(updatedPath, currentFolder + "/cosmos-launcher")
        if err != nil {
            fmt.Println("Update: Failed to rename cosmos-launcher.updated:", err)
        }

        // set old permissions
        if perms != 0 {
            err = os.Chmod(currentFolder + "/cosmos-launcher", perms)
            if err != nil {
                fmt.Println("Update: Failed to set old permissions:", err)
            }
        }
        // set old owner
        if owner != 0 {
            err = os.Chown(currentFolder + "/cosmos-launcher", owner, -1)
            if err != nil {
                fmt.Println("Update: Failed to set old owner:", err)
            }
        }
    }
}
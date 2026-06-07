# mega-cli
Commandline Mega.nz downloader, written in GO.
# Prerequisites
golang (Debian, Fedora based distros and Chocolatey), go (Arch, openSuse based distros and Homebrew), GoLang.Go (Winget)
# Build
git clone https://github.com/CHJ85/mega-cli.git && cd mega-cli && go mod init mega && go build -ldflags="-s -w" -o mega main.go
# Usage
mega "url1 url2 url3...", or "urls.txt" (one url per line), and optionally "-o /path/to/directory" (current directory by default).

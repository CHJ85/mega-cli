# mega-cli
Commandline Mega.nz downloader, written in GO.
# Prerequisites
golang
# Build
git clone https://github.com/CHJ85/mega-cli.git; cd mega-cli; go mod init mega; go build -ldflags="-s -w" -o mega main.go
# Usage
mega "url1 url2 url3...", or "urls.txt" (one url per line), and "-o /path/to/directory" (current directory by default).

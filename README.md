# mega-cli
Commandline Mega.nz downloader, written in GO.
# Prerequisites
golang
# Build
go mod init mega; go build -ldflags="-s -w" -o mega main.go
# Usage
mega url1 url2 url3 -o /path/to/directory (current directory by default).

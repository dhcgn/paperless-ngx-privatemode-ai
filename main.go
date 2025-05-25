package main

import "fmt"

// Set in build time
var (
	Version   string = "dev"
	BuildTime string = "unknown"
	Commit    string = "unknown"
)

func main() {
	fmt.Println("paperless-ngx-privatemode-ai")
	fmt.Printf("Version: %s, Commit: %s, Build Time: %s\n", Version, Commit, BuildTime)
	fmt.Println("Url: https://github.com/dhcgn/paperless-ngx-privatemode-ai")
	fmt.Println("⚠️⚠️⚠️\n⚠️⚠️⚠️ Use it at your own risk, no warranty of any kind is provided.\n⚠️⚠️⚠️")

}

// Package main provides the command-line interface for Scintirete.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/scintirete/scintirete/cmd/scintirete-cli/cli"
	pb "github.com/scintirete/scintirete/gen/go/scintirete/v1"
)

var (
	version = "dev"
	commit  = "unknown"

	// Command line flags
	host     = flag.String("h", "localhost", "Server host")
	port     = flag.Int("p", 9090, "Server port")
	password = flag.String("a", "", "Password for authentication")
	database = flag.String("d", "", "Database name")
	help     = flag.Bool("help", false, "Show help")
)

func main() {
	flag.Parse()

	if *help {
		showUsage()
		return
	}

	// Set version information
	cli.SetVersion(version, commit)

	// Create CLI instance
	cliInstance := cli.NewCLI(*password)

	// Connect to server with improved feedback
	serverAddr := fmt.Sprintf("%s:%d", *host, *port)
	fmt.Printf("正在连接到服务器 %s...", serverAddr)

	if err := cliInstance.Connect(*host, *port); err != nil {
		fmt.Printf(" ❌\n")
		fmt.Fprintf(os.Stderr, "连接失败: %v\n", err)
		fmt.Fprintf(os.Stderr, "请检查:\n")
		fmt.Fprintf(os.Stderr, "  1. 服务器是否正在运行\n")
		fmt.Fprintf(os.Stderr, "  2. 主机地址和端口是否正确 (%s)\n", serverAddr)
		fmt.Fprintf(os.Stderr, "  3. 网络连接是否正常\n")
		os.Exit(1)
	}
	defer cliInstance.Close()

	// Verify server is responding with health check
	fmt.Printf(" ✓\n正在验证服务器状态...")

	if err := verifyServerHealth(cliInstance, *password); err != nil {
		fmt.Printf(" ❌\n")
		fmt.Fprintf(os.Stderr, "服务器验证失败: %v\n", err)
		fmt.Fprintf(os.Stderr, "请检查:\n")
		fmt.Fprintf(os.Stderr, "  1. 服务器是否正常运行\n")
		fmt.Fprintf(os.Stderr, "  2. 认证密码是否正确\n")
		fmt.Fprintf(os.Stderr, "  3. 服务器是否可以处理请求\n")
		os.Exit(1)
	}

	fmt.Printf(" ✓\n")
	fmt.Printf("✅ 成功连接到 Scintirete 服务器 %s\n", serverAddr)

	// Set database if specified
	if *database != "" {
		fmt.Printf("设置默认数据库: %s\n", *database)
		cli.SetCurrentDatabase(*database)
		cliInstance.SetPrompt(fmt.Sprintf("scintirete[%s]> ", *database))
	}

	// Check if there are command line arguments to execute
	args := flag.Args()
	if len(args) > 0 {
		// Execute single command and exit
		fmt.Printf("执行命令: %s\n", args[0])
		if err := cliInstance.ExecuteCommand(args); err != nil {
			fmt.Fprintf(os.Stderr, "命令执行失败: %v\n", err)
			os.Exit(1)
		}
		return
	}

	// Start interactive mode
	fmt.Println()
	cliInstance.Interactive(version, commit)
}

// verifyServerHealth performs a health check on the server
func verifyServerHealth(cliInstance *cli.CLI, password string) error {
	client := cliInstance.GetClient()

	// Create a context with timeout for health check
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Use ListDatabases as a health check (same as ping command)
	_, err := client.ListDatabases(ctx, &pb.ListDatabasesRequest{
		Auth: &pb.AuthInfo{Password: password},
	})

	return err
}

// showUsage displays the usage information
func showUsage() {
	fmt.Printf("Scintirete CLI %s - Scintirete 向量数据库命令行工具\n\n", version)
	fmt.Println("用法:")
	fmt.Printf("  %s [选项] [命令]\n\n", os.Args[0])
	fmt.Println("选项:")
	fmt.Println("  -h <主机>        服务器主机地址 (默认: localhost)")
	fmt.Println("  -p <端口>        服务器端口 (默认: 9090)")
	fmt.Println("  -a <密码>        认证密码")
	fmt.Println("  -d <数据库>      默认使用的数据库")
	fmt.Println("  --help           显示帮助信息")
	fmt.Println()
	fmt.Println("示例:")
	fmt.Printf("  %s -h localhost -p 9090 -a mypassword\n", os.Args[0])
	fmt.Printf("  %s ping\n", os.Args[0])
	fmt.Printf("  %s -d mydb collection list\n", os.Args[0])
	fmt.Println()
	fmt.Println("交互模式:")
	fmt.Println("  不带参数运行进入交互模式")
	fmt.Println("  输入 'help' 查看可用命令")
}

package cli

import (
	"bufio"
	"fmt"
	"os"
	"os/user"
	"strings"

	"github.com/kardianos/service"
	"github.com/spf13/cobra"
)

var installUser string

var installCmd = &cobra.Command{
	Use:   "install",
	Short: "把butler注册为systemd服务",
	RunE: func(cmd *cobra.Command, args []string) error {
		username := installUser
		if username == "" {
			u, err := askRunUser()
			if err != nil {
				return err
			}
			username = u
		}
		_, err := user.Lookup(username)
		if err != nil {
			return err
		}
		s, err := newService(username)
		if err != nil {
			return err
		}
		err = service.Control(s, "install")
		if err != nil {
			return err
		}
		fmt.Printf("✓ 已注册服务，运行用户: %s\n", username)
		return nil
	},
}

// askRunUser 交互询问运行用户，回车接受默认（当前用户）。
func askRunUser() (string, error) {
	def := ""
	if u, err := user.Current(); err == nil {
		def = u.Username
	}
	fmt.Printf("以哪个用户运行服务? [%s]: ", def)

	line, err := bufio.NewReader(os.Stdin).ReadString('\n')
	if err != nil {
		return "", err
	}
	line = strings.TrimSpace(line)
	if line == "" {
		return def, nil
	}
	return line, nil
}

var uninstallCmd = &cobra.Command{
	Use:   "uninstall",
	Short: "把butler从systemd服务卸载",
	RunE: func(cmd *cobra.Command, args []string) error {
		return service.Control(svc, "uninstall")
	},
}

var stopCmd = &cobra.Command{
	Use:   "stop",
	Short: "停止butler服务",
	RunE: func(cmd *cobra.Command, args []string) error {
		return service.Control(svc, "stop")
	},
}

var startCmd = &cobra.Command{
	Use:   "start",
	Short: "开始butler服务",
	RunE: func(cmd *cobra.Command, args []string) error {
		return service.Control(svc, "start")
	},
}

func init() {
	installCmd.Flags().StringVar(&installUser, "user", "", "服务运行用户（不指定则交互询问）")
}

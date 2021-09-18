package cmd

import (
	"github.com/ferama/rospo/cmd/cmnflags"
	"github.com/ferama/rospo/pkg/conf"
	"github.com/ferama/rospo/pkg/sshc"
	"github.com/ferama/rospo/pkg/tun"

	"github.com/spf13/cobra"
)

func init() {
	tunCmd.AddCommand(tunForwardCmd)
}

var tunForwardCmd = &cobra.Command{
	Use:   "forward [user@][server]:port",
	Short: "Creates a forward ssh tunnel",
	Long:  `Creates a forward ssh tunnel`,
	Example: `
  # Forwards the local 8080 port to the remote 8080 
  $ rospo tun forward -l :8080 -r :8080 user@server:port
	`,
	Args: cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		local, _ := cmd.Flags().GetString("local")
		remote, _ := cmd.Flags().GetString("remote")

		sshcConf := cmnflags.GetSshClientConf(cmd, args)
		config := &conf.Config{
			SshClient: sshcConf,
			Tunnel: []*tun.TunnelConf{
				{
					Remote:  remote,
					Local:   local,
					Forward: true,
				},
			},
		}

		client := sshc.NewSshConnection(config.SshClient)
		go client.Start()
		tun.NewTunnel(client, config.Tunnel[0], false).Start()
	},
}

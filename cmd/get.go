package cmd

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	pb "github.com/cheggaaa/pb/v3"
	"github.com/ferama/rospo/cmd/cmnflags"
	"github.com/ferama/rospo/pkg/rio"
	"github.com/ferama/rospo/pkg/sshc"
	"github.com/pkg/sftp"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(getCmd)

	cmnflags.AddSshClientFlags(getCmd.Flags())
	getCmd.Flags().BoolP("recursive", "r", false, "if the copy should be recursive")
}

func getFile(client *sftp.Client, remote, local string) error {
	remotePath, err := client.RealPath(remote)
	if err != nil {
		return fmt.Errorf("invalid remote path: %s", remotePath)
	}
	remoteStat, err := client.Stat(remotePath)
	if err != nil {
		return fmt.Errorf("cannot stat remote path: %s", remotePath)
	}
	rFile, err := client.Open(remotePath)
	if err != nil {
		return fmt.Errorf("cannot open remote file for read: %s", err)
	}
	defer rFile.Close()

	lFile, err := os.Create(local)
	if err != nil {
		return fmt.Errorf("cannot open local file for write: %s", err)
	}
	defer lFile.Close()

	byteswrittench := make(chan int64)
	go func() {
		tmpl := `{{string . "target" | white}} {{with string . "prefix"}}{{.}} {{end}}{{counters . | blue }} {{bar . "|" "=" (cycle . "↖" "↗" "↘" "↙" ) "." "|" }} {{percent . | blue }} {{speed . | blue }} {{rtime . "ETA %s" | blue }}{{with string . "suffix"}} {{.}}{{end}}`
		pbar := pb.ProgressBarTemplate(tmpl).Start(0)
		pbar.Set(pb.Bytes, true)
		pbar.Set(pb.SIBytesPrefix, true)

		pbar.Set("target", filepath.Base(remotePath))
		pbar.SetTotal(remoteStat.Size())
		for w := range byteswrittench {
			pbar.Add64(w)
		}
		pbar.Finish()
	}()
	err = rio.CopyBuffer(lFile, rFile, byteswrittench)
	close(byteswrittench)
	if err != nil {
		return fmt.Errorf("error while writing local file: %s", err)
	}
	lFile.Chmod(remoteStat.Mode())
	return nil
}

func getFileRecursive(client *sftp.Client, remote, local string) error {
	remotePath, err := client.RealPath(remote)
	if err != nil {
		return fmt.Errorf("invalid remote path: %s", remotePath)
	}

	remoteStat, err := client.Stat(remotePath)
	if err != nil {
		return fmt.Errorf("cannot stat remote path: %s", remotePath)
	}
	if !remoteStat.IsDir() {
		return fmt.Errorf("remote path is not a directory: %s", remotePath)
	}

	localStat, err := os.Stat(local)
	if err != nil {
		return fmt.Errorf("cannot stat local path: %s", local)
	}
	if !localStat.IsDir() {
		return fmt.Errorf("local path is not a directory: %s", local)
	}

	dir := filepath.Dir(remotePath)
	walker := client.Walk(remotePath)
	for walker.Step() {
		if walker.Err() != nil {
			log.Println(walker.Err())
			continue
		}
		remotePath := walker.Path()
		stat := walker.Stat()
		part := strings.TrimPrefix(remotePath, dir)
		localPath := filepath.Join(local, part)
		if stat.IsDir() {
			err := os.Mkdir(localPath, stat.Mode())
			if err != nil {
				return fmt.Errorf("cannot create directory %s: %s", localPath, err)
			}
		} else {
			err := getFile(client, remotePath, localPath)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

var getCmd = &cobra.Command{
	Use:   "get [user@]host[:port] remote [local]",
	Short: "gets a file from remote",
	Long:  "gets a file from remote",
	Args:  cobra.MinimumNArgs(3),
	Run: func(cmd *cobra.Command, args []string) {
		remote := args[1]
		local := args[2]
		recursive, _ := cmd.Flags().GetBool("recursive")
		sshcConf := cmnflags.GetSshClientConf(cmd, args[0])
		sshcConf.Quiet = true
		conn := sshc.NewSshConnection(sshcConf)
		go conn.Start()
		conn.Connected.Wait()

		client, err := sftp.NewClient(conn.Client)
		if err != nil {
			log.Fatal(err)
		}
		defer client.Close()

		if recursive {
			err = getFileRecursive(client, remote, local)
		} else {
			err = getFile(client, remote, local)
		}
		if err != nil {
			log.Fatalln(err)
		}

	},
}

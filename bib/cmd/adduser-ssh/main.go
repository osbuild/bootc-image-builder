package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

var (
	// for tests
	rootdir = os.Getenv("OS_TOOLBOX_ADDUSER_ROOT")

	ghSSHKeyAPIFmt = "https://api.github.com/users/%s/keys"
)

func getSSHKeyGH(username string) (string, error) {
	resp, err := http.Get(fmt.Sprintf(ghSSHKeyAPIFmt, url.PathEscape(username)))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("cannot read ssh key data: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("unexpected http status when fetching ssh-key for %q: %v body:\n%s", username, resp.StatusCode, body)
	}

	var ghKeys []struct {
		ID  int    `json:"id"`
		Key string `json:"key"`
	}
	if err := json.Unmarshal(body, &ghKeys); err != nil {
		return "", fmt.Errorf("cannot unmarshal ssh keys for %q: %w", username, err)
	}

	buf := bytes.NewBuffer(nil)
	for _, ghkey := range ghKeys {
		fmt.Fprintf(buf, "# key for gh:%q (id: %v)\n", username, ghkey.ID)
		fmt.Fprintf(buf, "%s\n", ghkey.Key)
	}

	return buf.String(), nil
}

func getAuthorizedKeysContent(username, keySpec string) (string, error) {
	switch {
	case keySpec == "":
		return "", nil
	case strings.HasPrefix(keySpec, "gh:"):
		return getSSHKeyGH(strings.TrimPrefix(keySpec, "gh:"))
	default:
		return fmt.Sprintf("# key for %s from cmdline\n%s\n", username, keySpec), nil
	}
}

func run(cmd *cobra.Command, args []string) error {
	username := args[0]

	// deal with ssh keys
	keySpec, _ := cmd.Flags().GetString("ssh-key")
	authorizedKeysContent, err := getAuthorizedKeysContent(username, keySpec)
	if err != nil {
		return fmt.Errorf("cannot get ssh-key: %w", err)
	}
	if authorizedKeysContent != "" {
		// XXX: customize path, suport adduser options
		var homePath string
		if username == "root" {
			homePath = filepath.Join(rootdir, "/var/roothome/.ssh")
		} else {
			homePath = filepath.Join(rootdir, "/var/home/", username, ".ssh")
		}
		// XXX: customize perms
		if err := os.MkdirAll(homePath, 0o700); err != nil && !os.IsExist(err) {
			return err
		}
		authorizedKeysPath := filepath.Join(homePath, "authorized_keys")
		// XXX: make nicer, make part of e.g. getAuthorizedKeysContent()
		preamble := fmt.Sprintf("# created by adduser-ssh on %s\n", time.Now().Format(time.RFC822Z))
		content := preamble + authorizedKeysContent
		if err := os.WriteFile(authorizedKeysPath, []byte(content), 0600); err != nil {
			return err
		}
		fmt.Printf("created %q\n", authorizedKeysPath)
	}

	if skipUseradd, _ := cmd.Flags().GetBool("skip-useradd"); !skipUseradd {
		// now run adduser and pass options verbatim
		acmd := exec.Command("useradd", username)
		acmd.Args = append(acmd.Args, args[1:]...)
		acmd.Stdout = os.Stdout
		acmd.Stderr = os.Stderr
		if err := acmd.Run(); err != nil {
			return fmt.Errorf("cannot run %v: %w", acmd, err)
		}
	}

	return nil
}

func main() {
	var rootCmd = &cobra.Command{
		Use:           "adduser-ssh [user]",
		Short:         "Wrapper around adduser with sshkey adding",
		Args:          cobra.MinimumNArgs(1),
		RunE:          run,
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	rootCmd.Flags().String("ssh-key", "", "Specificy the ssh key to use (verbatim or gh:>user-id>)")
	rootCmd.Flags().Bool("skip-useradd", false, "Do not run useradd")

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %s\n", err)
		os.Exit(1)
	}
}

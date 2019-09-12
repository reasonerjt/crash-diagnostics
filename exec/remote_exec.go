package exec

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"gitlab.eng.vmware.com/vivienv/flare/ssh"

	"github.com/sirupsen/logrus"
	"gitlab.eng.vmware.com/vivienv/flare/script"
)

func exeRemotely(src *script.Script, machine *script.Machine, workdir string) error {
	asCmd, err := exeAs(src)
	if err != nil {
		return err
	}

	sshCmd, err := exeSSH(src)
	if err != nil {
		return err
	}

	user := asCmd.GetUserId()
	if sshCmd.GetUserId() != "" {
		user = sshCmd.GetUserId()
	}

	privKey := sshCmd.GetPrivateKeyPath()
	if privKey == "" {
		return fmt.Errorf("Missing private key file")
	}

	for _, action := range src.Actions {
		switch cmd := action.(type) {
		case *script.CopyCommand:
			if err := copyRemotely(user, privKey, machine, asCmd, cmd, workdir); err != nil {
				return err
			}
		case *script.CaptureCommand:
			// capture command output
			if err := captureRemotely(user, privKey, machine.Address(), cmd, workdir); err != nil {
				return err
			}
		default:
			logrus.Errorf("Unsupported command %T", cmd)
		}
	}

	return nil
}

func captureRemotely(user, privKey, hostAddr string, cmdCap *script.CaptureCommand, workdir string) error {
	sshc := ssh.New(user, privKey)
	if err := sshc.Dial(hostAddr); err != nil {
		return err
	}
	defer sshc.Hangup()

	cmdStr := cmdCap.GetCliString()
	logrus.Debugf("Capturing remote command command %v", cmdStr)
	cliCmd, cliArgs := cmdCap.GetParsedCli()

	cmdReader, err := sshc.SSHRun(cliCmd, cliArgs...)
	if err != nil {
		return err
	}

	fileName := fmt.Sprintf("%s.txt", sanitizeStr(cmdStr))
	filePath := filepath.Join(workdir, fileName)
	logrus.Debugf("Capturing output of [%s] -into-> %s", cmdStr, filePath)
	if err := writeFile(cmdReader, filePath); err != nil {
		return err
	}

	return nil
}

var (
	cliScpName = "scp"
	cliScpArgs = "-rpq"
)

// copyRemotely uses rsync and requires both rsync and ssh to be installed
func copyRemotely(user, privKey string, machine *script.Machine, asCmd *script.AsCommand, cmd *script.CopyCommand, dest string) error {
	if _, err := exec.LookPath(cliScpName); err != nil {
		return fmt.Errorf("remote copy: %s", err)
	}

	logrus.Debugf("Entering remote COPY command: %s", cmd.Args())

	host := machine.Host()
	port := machine.Port()

	asUid, asGid, err := asCmd.GetCredentials()
	if err != nil {
		return err
	}

	for _, path := range cmd.Args() {
		// if relPath, err := filepath.Rel(dest, path); err == nil && !strings.HasPrefix(relPath, "..") {
		// 	logrus.Errorf("%s path %s cannot be relative to %s", cmd.Name(), path, dest)
		// 	continue
		// }

		remotePath := fmt.Sprintf("%s@%s:%s", user, host, path)
		logrus.Debugf("Copying %s to %s", remotePath, dest)

		targetPath := filepath.Join(dest, path)
		targetDir := filepath.Dir(targetPath)
		if _, err := os.Stat(targetDir); err != nil {
			if os.IsNotExist(err) {
				if err := os.MkdirAll(targetDir, 0744); err != nil && !os.IsExist(err) {
					return err
				}
				logrus.Debugf("Created dir %s", targetDir)
			} else {
				return err
			}
		}

		args := []string{cliScpArgs, "-o StrictHostKeyChecking=no", "-P", port, remotePath, targetPath}
		_, err := CliRun(uint32(asUid), uint32(asGid), nil, cliScpName, args...)
		if err != nil {
			return err
		}
		logrus.Debug("Remote copy succeeded:", remotePath)
	}

	return nil
}

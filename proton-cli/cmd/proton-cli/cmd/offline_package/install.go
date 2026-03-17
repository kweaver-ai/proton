package offline_package

import (
	"archive/tar"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"k8s.io/utils/exec"
)

type installOptions struct {
	remain bool
}

func newInstallCommand() *cobra.Command {
	opts := &installOptions{}

	cmd := &cobra.Command{
		Use:  "install PACKAGE",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return install(exec.New(), args[0], opts)
		},
	}

	cmd.Flags().BoolVar(&opts.remain, "remain", opts.remain, "Remain working directory")
	return cmd
}

func install(executor exec.Interface, pkg string, opts *installOptions) error {
	// working directory
	wd := ".proton-offline-package"
	if err := os.MkdirAll(wd, 0o755); err != nil {
		return err
	}
	if !opts.remain {
		defer os.RemoveAll(wd)
	}

	r, err := os.Open(pkg)
	if err != nil {
		return err
	}
	defer r.Close()

	// extract tarball
	tr := tar.NewReader(r)
	fmt.Println("Extracting tarball")
	for {
		h, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return err
		}

		if h.FileInfo().IsDir() {
			if err := os.MkdirAll(filepath.Join(wd, h.Name), h.FileInfo().Mode()); err != nil {
				return err
			}
			continue
		}

		if err := extractFileFromTar(tr, h, wd); err != nil {
			return err
		}
	}

	// execute install script
	fmt.Println("Executing install script")
	cmd := executor.Command("bash", filepath.Join(wd, "install.sh"))
	cmd.SetStdin(os.Stdin)
	cmd.SetStdout(os.Stdout)
	cmd.SetStderr(os.Stderr)
	if err := cmd.Run(); err != nil {
		return err
	}

	return nil
}

func extractFileFromTar(tr *tar.Reader, h *tar.Header, dir string) error {
	p := filepath.Join(dir, h.Name)

	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}

	f, err := os.Create(p)
	if err != nil {
		return err
	}
	defer f.Close()

	if err := f.Chmod(h.FileInfo().Mode()); err != nil {
		return err
	}

	if _, err := io.Copy(f, tr); err != nil {
		return err
	}

	return nil
}

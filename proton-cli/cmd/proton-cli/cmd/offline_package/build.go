package offline_package

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"oras.land/oras-go/v2"
	"oras.land/oras-go/v2/content/oci"
	"oras.land/oras-go/v2/registry"
	"oras.land/oras-go/v2/registry/remote"
	"sigs.k8s.io/yaml"
)

type buildOptions struct {
	manifest string
}

func defaultBuildOptions() *buildOptions {
	return &buildOptions{
		manifest: "manifest.yaml",
	}
}

func (opts *buildOptions) AddFlag(s *pflag.FlagSet) {
	s.StringVar(&opts.manifest, "manifest", opts.manifest, "Path to the manifest file")
}

func newBuildCommand() *cobra.Command {
	opts := defaultBuildOptions()

	cmd := &cobra.Command{
		Use:   "build [flags]",
		Short: "Build a proton offline package",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			y, err := os.ReadFile(opts.manifest)
			if err != nil {
				return err
			}

			var m Manifest
			if err := yaml.Unmarshal(y, &m); err != nil {
				return err
			}

			return build(cmd.Context(), &m)
		},
	}

	opts.AddFlag(cmd.Flags())

	return cmd
}

func build(ctx context.Context, m *Manifest) error {
	// create temporary directory as workspace
	w, err := os.MkdirTemp("", "proton-cli-offline-package-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(w)
	log.Printf("working directory %q", w)

	var (
		binDir   = filepath.Join(w, "bin")
		chartDir = filepath.Join(w, "charts")
		imageDir = filepath.Join(w, "images")
		rpmDir   = filepath.Join(w, "rpms")
	)

	for _, p := range []string{
		binDir,
		chartDir,
		imageDir,
		rpmDir,
	} {
		if err := os.MkdirAll(p, 0755); err != nil {
			return err
		}
	}

	// create entrypoint script
	if err := os.WriteFile(filepath.Join(w, "install.sh"), installBytes, 0755); err != nil {
		return err
	}

	// pull binaries
	for _, a := range m.Spec.Binaries {
		if err := pull(ctx, &a, binDir); err != nil {
			return err
		}
	}

	// pull charts
	for _, a := range m.Spec.Charts {
		if err := pull(ctx, &a, chartDir); err != nil {
			return err
		}
	}

	// pull images
	for _, a := range m.Spec.Images {
		if err := pull(ctx, &a, imageDir); err != nil {
			return err
		}
	}

	// pull rpms
	for _, a := range m.Spec.RPMs {
		if err := pull(ctx, &a, rpmDir); err != nil {
			return err
		}
	}

	// package tarball
	f, err := os.Create("proton-cli-offline-package.tar")
	if err != nil {
		return err
	}
	defer f.Close()

	tw := tar.NewWriter(f)
	defer tw.Close()

	if err := tw.AddFS(os.DirFS(w)); err != nil {
		return err
	}

	return nil
}

func pull(ctx context.Context, a *Artifact, output string) error {
	switch {
	case a.HTTP != nil:
		return pullHTTP(ctx, filepath.Join(output, a.Name), a.HTTP)
	case a.OCI != nil:
		return pullOCI(ctx, output, a.Name, a.OCI)
	default:
		return fmt.Errorf("failed to find artifact source of %q", a.Name)
	}
}

func pullHTTP(ctx context.Context, path string, s *HTTPSource) error {
	log.Println("pull http", s.URL)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.URL, http.NoBody)
	if err != nil {
		return err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	var r io.Reader
	switch s.Format {
	case "":
		r = resp.Body
	case "tar+gzip":
		gr, err := gzip.NewReader(resp.Body)
		if err != nil {
			return err
		}
		defer gr.Close()

		tr := tar.NewReader(gr)
		for {
			h, err := tr.Next()
			if errors.Is(err, io.EOF) {
				return fmt.Errorf("%s not found", s.Path)
			}
			if err != nil {
				return err
			}
			if h.Name != s.Path {
				continue
			}
			r = tr
			break
		}
	default:
		return fmt.Errorf("invalid format %q", s.Format)
	}

	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	if _, err := io.Copy(f, r); err != nil {
		return err
	}

	return nil
}

func pullOCI(ctx context.Context, output, ref string, s *OCISource) error {
	log.Println("pull oci", s.Reference)
	// get oci artifact reference
	ar, err := registry.ParseReference(s.Reference)
	if err != nil {
		return err
	}

	r := &remote.Repository{
		Reference: ar,
		PlainHTTP: true, // TODO: from commandline flag
	}

	dst, err := oci.New(output)
	if err != nil {
		return err
	}

	if _, err := oras.Copy(ctx, r, ar.Reference, dst, ref, oras.DefaultCopyOptions); err != nil {
		return err
	}

	return nil
}

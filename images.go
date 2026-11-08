package moviedata

import (
	"context"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
)

// image is the file the image at The Movie Database's imgPath is kept in,
// under images beside the database, fetched the first time it is asked for.
func (c *Client) image(ctx context.Context, imgPath string) (string, error) {
	name := path.Base(imgPath)
	if name == "." || name == "/" || name == ".." {
		return "", fmt.Errorf("image path %q names no file", imgPath)
	}

	p := filepath.Join(c.dir, "images", name)
	if _, err := os.Stat(p); err == nil {
		return p, nil
	}

	cli, err := c.client()
	if err != nil {
		return "", err
	}

	img, err := cli.GetImage(ctx, imgPath)
	if err != nil {
		return "", fmt.Errorf("getting %s: %w", imgPath, notFound(err))
	}
	defer img.Close()

	return p, writeFile(p, img)
}

// writeFile writes through a temporary file, so a reader never sees half of one.
func writeFile(p string, r io.Reader) error {
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		return err
	}

	f, err := os.CreateTemp(filepath.Dir(p), ".tmp-*")
	if err != nil {
		return err
	}
	// Once renamed there is nothing left to remove; until then, a failure leaves no trace.
	defer func() { _ = os.Remove(f.Name()) }()

	if _, err := io.Copy(f, r); err != nil {
		_ = f.Close()

		return err
	}

	if err := f.Close(); err != nil {
		return err
	}

	return os.Rename(f.Name(), p)
}

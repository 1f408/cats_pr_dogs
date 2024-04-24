package conf

import (
	"cats_pr_dogs/lorca"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sync"

	"github.com/l4go/unifs"
	"github.com/naoina/toml"
)

type PrDogsConfig struct {
	Chrome ChromeConfig
}

type ChromeConfig struct {
	Path map[string][]string
	Args []string
}

func new_err(format string, v ...interface{}) error {
	return errors.New(fmt.Sprintf(format, v...))
}

func NewPrDogsConfigFS(fsys fs.FS, file string) (*PrDogsConfig, error) {
	cfg_f, err := unifs.Open(fsys, file)
	if err != nil {
		return nil, new_err("Config file open error: %s", err)
	}
	defer cfg_f.Close()
	text, err := io.ReadAll(cfg_f)
	if err != nil {
		return nil, new_err("Config read error: %s", err)
	}

	cfg := &PrDogsConfig{}
	if err := toml.Unmarshal(text, cfg); err != nil {
		return nil, new_err("Config file parse error: %s", err)
	}

	return cfg, nil
}

type WindowConfig struct {
	Width  int
	Height int
	mtx    sync.Mutex `toml:"-"`
}

const MinimunPixels = 300

func NewWindowConfigDefault() *WindowConfig {
	return &WindowConfig{
		Width: lorca.PageA4Width, Height: lorca.PageA4Height,
	}
}

func NewWindowConfig(os_file string) (*WindowConfig, error) {
	cfg := NewWindowConfigDefault()

	cfg.mtx.Lock()
	defer cfg.mtx.Unlock()

	cfg_f, err := os.Open(os_file)
	if err != nil {
		return cfg, nil
	}
	defer cfg_f.Close()

	text, err := io.ReadAll(cfg_f)
	if err != nil {
		return nil, new_err("Config read error: %s", err)
	}

	if err := toml.Unmarshal(text, cfg); err != nil {
		return nil, new_err("Config file parse error: %s", err)
	}

	if cfg.Width < MinimunPixels {
		cfg.Width = MinimunPixels
	}
	if cfg.Height < MinimunPixels {
		cfg.Height = MinimunPixels
	}

	return cfg, nil
}

func (wc *WindowConfig) GetSize() (int, int) {
	wc.mtx.Lock()
	defer wc.mtx.Unlock()

	return wc.Width, wc.Height
}

func (wc *WindowConfig) SetSize(width, height int) {
	wc.mtx.Lock()
	defer wc.mtx.Unlock()
	wc.Width = width
	wc.Height = height
}

func (wc *WindowConfig) Save(os_file string) error {
	wc.mtx.Lock()
	defer wc.mtx.Unlock()

	text, err := toml.Marshal(wc)
	if err != nil {
		return new_err("Config save error: %s", err)
	}

	tmpdir, err := os.MkdirTemp("", "cats_pr_dogs")
	if err != nil {
		return new_err("Config save error: %s", err)
	}
	defer os.RemoveAll(tmpdir)

	tmpfile := filepath.Join(tmpdir, "window.conf")
	if err := os.WriteFile(tmpfile, text, 0644); err != nil {
		return new_err("Config save error: %s", err)
	}

	if err := os.Rename(tmpfile, os_file); err != nil {
		return new_err("Config save error: %s", err)
	}

	return nil
}

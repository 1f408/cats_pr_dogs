package conf

import (
	"errors"
	"runtime"
	"os"

	"cats_pr_dogs/lorca"
)

var ErrNotFoundChrome = errors.New("not found chrome")
var ErrBadChromePath = errors.New("bad chrome path config")

func find_arch_chrome(ext_paths []string) (string, error) {
	for _, ep := range ext_paths {
		p := os.ExpandEnv(ep)
		if _, err := os.Stat(p); os.IsNotExist(err) {
			continue
		}
		return p, nil
	}

	return "", ErrNotFoundChrome
}

func find_chrome(path_map map[string][]string) (string, error) {
	typ_list := []string{
		runtime.GOOS + "/" + runtime.GOARCH,
		runtime.GOOS,
		"default",
	}

	for _, typ := range typ_list {
		if p, ok := path_map[typ]; ok {
			return find_arch_chrome(p)
		}
	}

	return "", ErrNotFoundChrome
}

func NewLorca(cfg *ChromeConfig) (*lorca.Lorca, error) {
    os_p, err := find_chrome(cfg.Path)
	if err != nil {
		return nil, err
	}
	
	lrc := &lorca.Lorca{
		ChromePath: os_p,
		ChromeArgs: cfg.Args,
	}

	return lrc, nil
}

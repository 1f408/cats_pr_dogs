package main

import (
	"embed"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"path"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"syscall"

	"github.com/l4go/mergefs"
	"github.com/l4go/osfs"
	"github.com/l4go/remapfs"
	"github.com/l4go/rpath"
	"github.com/l4go/task"
	"github.com/l4go/unifs"

	"github.com/1f408/cats_eeds/upath"
	"github.com/1f408/cats_eeds/view/mdview"

	"cats_pr_dogs/conf"
	"cats_pr_dogs/lorca"
)

var Debug bool = false

const PrDogsConfigUnipath = "/conf/etc/pr_dogs.conf"
const AppConfigUnipath = "/conf/etc/mdview.conf"
const AppName = "CatPrMd"
const StaticFileCacheControl = "max-age=86400, must-revalidate"

const ViewTopUrl = "/file/"
const FsDocumentRoot = "/file/"

var FsDocumentRootUpath = upath.MustNew(FsDocumentRoot)

var CustomConfigDir string = func() string {
	dir, err := os.UserConfigDir()
	if err != nil {
		return ""
	}

	return filepath.Join(dir, AppName)
}()
var WindowConfigFile string = filepath.Join(CustomConfigDir, "window.conf")

var PrDogsConfig *conf.PrDogsConfig
var Lorca *lorca.Lorca

var PreviewPath string
var HttpPath string

var IndexName string

var WebViewFS fs.FS
var WwwFS fs.FS

var ErrEmbedFile = errors.New("embed file")
var SkipWatchFileRe []*regexp.Regexp

//go:embed init.js
var InitJS string

//go:embed favicon.ico
var FaviconFS embed.FS

func init() {
	flag.CommandLine.SetOutput(os.Stderr)
	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(),
			"Usage: %s [options ...] <preview path>\n", os.Args[0])
		flag.PrintDefaults()
	}
	flag.BoolVar(&Debug, "d", Debug, "debug flag")
	flag.StringVar(&CustomConfigDir, "c", CustomConfigDir, "custom config directory path")

	flag.Parse()
	if flag.NArg() > 1 {
		flag.Usage()
		os.Exit(1)
	}

	if flag.NArg() == 0 {
		var err error
		PreviewPath, err = conf.StartDir()
		if err != nil {
			fmt.Fprintln(os.Stderr, "not found start dir", err)
			os.Exit(2)
		}
	} else {
		PreviewPath = flag.Arg(0)
		if _, err := os.Stat(PreviewPath); err != nil {
			fmt.Fprintln(os.Stderr, "file open error:", err)
			os.Exit(2)
		}
	}

	var conf_fs fs.FS = conf.DefaultConfFS
	if CustomConfigDir != "" {
		conf_fs = mergefs.New(os.DirFS(CustomConfigDir), conf.DefaultConfFS)
	}

	WebViewFS = remapfs.MustNew(remapfs.FSMap{
		"file": osfs.OsRootFS,
		"conf": conf_fs,
	})

	www_fs, err := fs.Sub(conf_fs, "www")
	if err != nil {
		fmt.Fprintln(os.Stderr, "fail config fs.FS:", err)
		os.Exit(2)
	}

	WwwFS = www_fs

	cfg, cerr := conf.NewPrDogsConfigFS(WebViewFS, PrDogsConfigUnipath)
	if cerr != nil {
		fmt.Fprintf(os.Stderr, "fail cats_pr_dogs config: %s: %s\n", PrDogsConfigUnipath, cerr.Error())
		os.Exit(2)
	}
	PrDogsConfig = cfg

	lrc, lerr := conf.NewLorca(&cfg.Chrome)
	if lerr != nil {
		fmt.Fprintln(os.Stderr, "fail Lorca:", lerr)
		os.Exit(2)
	}
	Lorca = lrc
}

func file_to_location(os_file string) (string, error) {
	fi, err := os.Stat(os_file)
	if err != nil {
		return "", err
	}
	is_dir := fi.IsDir()

	up, err := unifs.FromOSPath(os_file)
	if err != nil {
		return "", err
	}

	return rpath.Join(ViewTopUrl, rpath.SetType(up, is_dir)), nil
}

func is_ignore_watch(base string) bool {
	for _, re := range SkipWatchFileRe {
		if re.MatchString(base) {
			return true
		}
	}
	return false
}

func notify_onpageshow(un *UpdateNotify, raw_url string) {
	os_dir, err := url2osdir(raw_url)
	if err != nil {
		return
	}

	un.SetDir(os_dir)
}

func url2osdir(raw_url string) (string, error) {
	u, err := url.Parse(raw_url)
	if err != nil {
		return "", err
	}

	file, ok := strings.CutPrefix(u.Path, ViewTopUrl)
	if !ok {
		return "", ErrEmbedFile
	}
	file = "/" + file

	dir := rpath.Dir(file)
	if rpath.IsDir(file) {
		dir = file
	}
	dir = path.Clean(dir)

	os_dir, err := unifs.ToOSPath(dir)
	if err != nil {
		return "", err
	}

	return os_dir, nil
}

func create_ui(url_str string, debug bool, width, height int) (lorca.UI, error) {
	args := []string{}
	if runtime.GOOS == "linux" {
		args = append(args, "--class="+AppName)
	}
	if debug {
		args = append(args, "--auto-open-devtools-for-tabs")
	}
	args = append(args, "--no-referrers")
	args = append(args, "--remote-allow-origins=http://127.0.0.1")

	return Lorca.NewUI(url_str, width, height, args...)
}

type serveConfFile struct {
	cache_control string
	file_server   http.Handler
}

func (scf *serveConfFile) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if scf.cache_control != "" {
		w.Header().Set("Cache-Control", scf.cache_control)
	}

	scf.file_server.ServeHTTP(w, r)
}

func confFileServer(root http.FileSystem, cache_control string) http.Handler {
	return &serveConfFile{cache_control: cache_control, file_server: http.FileServer(root)}
}

func main() {
	cfg, err := mdview.NewMdViewConfigFS(WebViewFS, AppConfigUnipath)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	cfg.UrlTopPath = ViewTopUrl
	cfg.DocumentRoot = FsDocumentRootUpath
	cfg.DirectoryViewRoots = []upath.UPath{FsDocumentRootUpath}

	win_cfg, err := conf.NewWindowConfig(WindowConfigFile)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Warning: window config error:", err)
		win_cfg = conf.NewWindowConfigDefault()
	}

	cc := task.NewCancel()
	defer cc.Cancel()

	signal_chan := make(chan os.Signal, 1)
	signal.Notify(signal_chan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		select {
		case <-cc.RecvCancel():
		case <-signal_chan:
			cc.Cancel()
		}
	}()

	loc, err := file_to_location(PreviewPath)
	if err != nil {
		log.Printf("not found: %v.\n", err)
		return
	}

	var lcnf net.ListenConfig
	lstn, err := lcnf.Listen(cc.AsContext(), "tcp", "127.0.0.1:0")
	if err != nil {
		log.Fatal(err)
		return
	}
	defer lstn.Close()
	loc_url := fmt.Sprintf("http://%s%s", lstn.Addr(), loc)

	srv := &http.Server{}
	defer srv.Close()

	cfg.SocketType = "tcp"
	cfg.SocketPath = lstn.Addr().String()
	srv.Addr = lstn.Addr().String()
	IndexName = cfg.IndexName

	mdv, verr := mdview.NewMdView(cfg)
	if verr != nil {
		fmt.Fprintln(os.Stderr, verr)
		os.Exit(1)
	}
	SkipWatchFileRe = mdv.DirectoryViewHidden

	http.Handle("/favicon.ico", confFileServer(http.FS(FaviconFS), StaticFileCacheControl))
	http.Handle("/", confFileServer(http.FS(WwwFS), StaticFileCacheControl))
	http.Handle(ViewTopUrl, http.StripPrefix(ViewTopUrl, http.HandlerFunc(mdv.Handler)))

	win_width, win_height := win_cfg.GetSize()
	ui, err := create_ui(loc_url, Debug, win_width, win_height)
	if err != nil {
		log.Fatal(err)
	}
	defer ui.Close()

	ui.Init(InitJS)

	up_notify, err := NewUpdateNotify(func() { ui.Reload(false) }, is_ignore_watch)
	if err != nil {
		log.Fatal(err)
	}
	defer up_notify.Close()

	ui.SetOnPageshow(func(u string) { notify_onpageshow(up_notify, u) })

	ui.SetOnResize(func() {
		if bounds, err := ui.Bounds(); err == nil {
			win_cfg.SetSize(bounds.Width, bounds.Height)
		}
	})

	go func() {
		serr := srv.Serve(lstn)
		switch serr {
		case nil:
		case http.ErrServerClosed:
		default:
			log.Printf("HTTP server error: %v.\n", serr)
		}
	}()

	select {
	case <-ui.Done():
		cc.Cancel()
	case <-cc.RecvCancel():
	}

	os.MkdirAll(filepath.Dir(WindowConfigFile), 0755)
	win_cfg.Save(WindowConfigFile)
	log.Println("exiting...")
}

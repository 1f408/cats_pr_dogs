package lorca

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"reflect"
)

// UI interface allows talking to the HTML5 UI from Go.
type UI interface {
	Load(url string) error
	Bounds() (Bounds, error)
	SetBounds(Bounds) error
	Bind(name string, f interface{}) error
	Eval(js string) Value
	Init(js string) error
	Reload(super bool) error
	Done() <-chan struct{}
	Close() error

	SetOnResize(func())
	SetOnPageshow(func(url string))
	SetOnLoad(func())
}

type ui struct {
	chrome *chrome
	done   chan struct{}
	tmpDir string
}

// New returns a new HTML5 UI for the given URL, user profile directory, window
// size and other options passed to the browser engine. If URL is an empty
// string - a temporary directory is created and it will be removed on
// ui.Close(). You might want to use "--headless" custom CLI argument to test
// your UI code.
func (lrc *Lorca) NewUI(url string, width, height int, customArgs ...string) (UI, error) {
	if url == "" {
		url = "data:text/html,<html></html>"
	}

	tmpDir, err := os.MkdirTemp("", "lorca")
	if err != nil {
		return nil, err
	}

	args := make([]string, 0, len(lrc.ChromeArgs)+4+len(customArgs))
	args = append(args, lrc.ChromeArgs...)
	args = append(args, fmt.Sprintf("--app=%s", url))
	args = append(args, fmt.Sprintf("--user-data-dir=%s", tmpDir))
	args = append(args, fmt.Sprintf("--window-size=%d,%d", width, height))
	args = append(args, customArgs...)
	args = append(args, "--remote-debugging-port=0")

	chrome, err := newChromeWithArgs(lrc.ChromePath, args...)
	done := make(chan struct{})
	if err != nil {
		return nil, err
	}

	go func() {
		chrome.cmd.Wait()
		close(done)
	}()
	return &ui{chrome: chrome, done: done, tmpDir: tmpDir}, nil
}

func (u *ui) Done() <-chan struct{} {
	return u.done
}

func (u *ui) Close() error {
	// ignore err, as the chrome process might be already dead, when user close the window.
	u.chrome.kill()
	<-u.done
	if u.tmpDir != "" {
		if err := os.RemoveAll(u.tmpDir); err != nil {
			return err
		}
	}
	return nil
}

func (u *ui) Load(url string) error { return u.chrome.load(url) }

func (u *ui) Bind(name string, f interface{}) error {
	v := reflect.ValueOf(f)
	// f must be a function
	if v.Kind() != reflect.Func {
		return errors.New("only functions can be bound")
	}
	// f must return either value and error or just error
	if n := v.Type().NumOut(); n > 2 {
		return errors.New("function may only return a value or a value+error")
	}

	return u.chrome.bind(name, func(raw []json.RawMessage) (interface{}, error) {
		if len(raw) != v.Type().NumIn() {
			return nil, errors.New("function arguments mismatch")
		}
		args := []reflect.Value{}
		for i := range raw {
			arg := reflect.New(v.Type().In(i))
			if err := json.Unmarshal(raw[i], arg.Interface()); err != nil {
				return nil, err
			}
			args = append(args, arg.Elem())
		}
		errorType := reflect.TypeOf((*error)(nil)).Elem()
		res := v.Call(args)
		switch len(res) {
		case 0:
			// No results from the function, just return nil
			return nil, nil
		case 1:
			// One result may be a value, or an error
			if res[0].Type().Implements(errorType) {
				if res[0].Interface() != nil {
					return nil, res[0].Interface().(error)
				}
				return nil, nil
			}
			return res[0].Interface(), nil
		case 2:
			// Two results: first one is value, second is error
			if !res[1].Type().Implements(errorType) {
				return nil, errors.New("second return value must be an error")
			}
			if res[1].Interface() == nil {
				return res[0].Interface(), nil
			}
			return res[0].Interface(), res[1].Interface().(error)
		default:
			return nil, errors.New("unexpected number of return values")
		}
	})
}

func (u *ui) SetOnResize(onresize func()) {
	u.chrome.setOnResize(onresize)
}

func (u *ui) SetOnPageshow(onframe func(url string)) {
	u.chrome.setOnShowpage(onframe)
}

func (u *ui) SetOnLoad(onload func()) {
	u.chrome.setOnLoad(onload)
}

func (u *ui) Eval(js string) Value {
	v, err := u.chrome.eval(js)
	return value{err: err, raw: v}
}

func (u *ui) Reload(super bool) error {
	return u.chrome.reload(super, "") 
}

func (u *ui) Init(js string) error {
	return u.chrome.evalOnNew(js)
}

func (u *ui) SetBounds(b Bounds) error {
	return u.chrome.setBounds(b)
}

func (u *ui) Bounds() (Bounds, error) {
	return u.chrome.bounds()
}

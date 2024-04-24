package lorca

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"os/exec"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"golang.org/x/net/websocket"

	"github.com/l4go/timer"
)

type h = map[string]interface{}

// Result is a struct for the resulting value of the JS expression or an error.
type result struct {
	Value json.RawMessage
	Err   error
}

type bindingFunc func(args []json.RawMessage) (interface{}, error)

// Msg is a struct for incoming messages (results and async events)
type msg struct {
	ID     int             `json:"id"`
	Result json.RawMessage `json:"result"`
	Error  json.RawMessage `json:"error"`
	Method string          `json:"method"`
	Params json.RawMessage `json:"params"`
}

type chrome struct {
	sync.Mutex
	cmd        *exec.Cmd
	ws         *websocket.Conn
	id         int32
	target     string
	session    string
	window     int
	pending    map[int]chan result
	bindings   map[string]bindingFunc
	onload     func()
	onshowpage func(url string)
	onresize   func()
}

func newChromeWithArgs(chromeBinary string, args ...string) (*chrome, error) {
	// The first two IDs are used internally during the initialization
	c := &chrome{
		id:       2,
		pending:  map[int]chan result{},
		bindings: map[string]bindingFunc{},
	}

	// Start chrome process
	c.cmd = exec.Command(chromeBinary, args...)
	pipe, err := c.cmd.StderrPipe()
	if err != nil {
		return nil, err
	}
	if err := c.cmd.Start(); err != nil {
		return nil, err
	}

	// Wait for websocket address to be printed to stderr
	re := regexp.MustCompile(`^DevTools listening on (ws://.*?)\r?\n$`)
	m, err := readUntilMatch(pipe, re)
	if err != nil {
		c.kill()
		return nil, err
	}
	wsURL := m[1]

	// Open a websocket
	c.ws, err = websocket.Dial(wsURL, "", "http://127.0.0.1")
	if err != nil {
		c.kill()
		return nil, err
	}

	// Find target and initialize session
	c.target, err = c.findTarget()
	if err != nil {
		c.kill()
		return nil, err
	}

	c.session, err = c.startSession(c.target)
	if err != nil {
		c.kill()
		return nil, err
	}
	go c.readLoop()
	for method, args := range map[string]h{
		"Page.enable":          nil,
		"Target.setAutoAttach": {"autoAttach": true, "waitForDebuggerOnStart": false},
		"Network.enable":       nil,
		"Runtime.enable":       nil,
		"Security.enable":      nil,
		"Performance.enable":   nil,
		"Log.enable":           nil,
	} {
		if _, err := c.send(method, args); err != nil {
			c.kill()
			c.cmd.Wait()
			return nil, err
		}
	}

	if !contains(args, "--headless") {
		win, err := c.getWindowForTarget(c.target)
		if err != nil {
			c.kill()
			return nil, err
		}
		c.window = win.WindowID
	}

	return c, nil
}

func (c *chrome) findTarget() (string, error) {
	err := websocket.JSON.Send(c.ws, h{
		"id": 0, "method": "Target.setDiscoverTargets", "params": h{"discover": true},
	})
	if err != nil {
		return "", err
	}
	for {
		m := msg{}
		if err = websocket.JSON.Receive(c.ws, &m); err != nil {
			return "", err
		} else if m.Method == "Target.targetCreated" {
			target := struct {
				TargetInfo struct {
					Type string `json:"type"`
					ID   string `json:"targetId"`
				} `json:"targetInfo"`
			}{}
			if err := json.Unmarshal(m.Params, &target); err != nil {
				return "", err
			} else if target.TargetInfo.Type == "page" {
				return target.TargetInfo.ID, nil
			}
		}
	}
}

func (c *chrome) startSession(target string) (string, error) {
	err := websocket.JSON.Send(c.ws, h{
		"id": 1, "method": "Target.attachToTarget", "params": h{"targetId": target},
	})
	if err != nil {
		return "", err
	}
	for {
		m := msg{}
		if err = websocket.JSON.Receive(c.ws, &m); err != nil {
			return "", err
		} else if m.ID == 1 {
			if m.Error != nil {
				return "", errors.New("Target error: " + string(m.Error))
			}
			session := struct {
				ID string `json:"sessionId"`
			}{}
			if err := json.Unmarshal(m.Result, &session); err != nil {
				return "", err
			}
			return session.ID, nil
		}
	}
}

// WindowState defines the state of the Chrome window, possible values are
// "normal", "maximized", "minimized" and "fullscreen".
type WindowState string

const (
	// WindowStateNormal defines a normal state of the browser window
	WindowStateNormal WindowState = "normal"
	// WindowStateMaximized defines a maximized state of the browser window
	WindowStateMaximized WindowState = "maximized"
	// WindowStateMinimized defines a minimized state of the browser window
	WindowStateMinimized WindowState = "minimized"
	// WindowStateFullscreen defines a fullscreen state of the browser window
	WindowStateFullscreen WindowState = "fullscreen"
)

// Bounds defines settable window properties.
type Bounds struct {
	Left        int         `json:"left"`
	Top         int         `json:"top"`
	Width       int         `json:"width"`
	Height      int         `json:"height"`
	WindowState WindowState `json:"windowState"`
}

type windowTargetMessage struct {
	WindowID int    `json:"windowId"`
	Bounds   Bounds `json:"bounds"`
}

func (c *chrome) getWindowForTarget(target string) (windowTargetMessage, error) {
	var m windowTargetMessage
	msg, err := c.send("Browser.getWindowForTarget", h{"targetId": target})
	if err != nil {
		return m, err
	}
	err = json.Unmarshal(msg, &m)
	return m, err
}

type targetMessageTemplate struct {
	ID     int    `json:"id"`
	Method string `json:"method"`
	Params struct {
		Name    string `json:"name"`
		Payload string `json:"payload"`
		ID      int    `json:"executionContextId"`
		Args    []struct {
			Type  string      `json:"type"`
			Value interface{} `json:"value"`
		} `json:"args"`
	} `json:"params"`
	Error struct {
		Message string `json:"message"`
	} `json:"error"`
	Result json.RawMessage `json:"result"`
}

type targetMessage struct {
	targetMessageTemplate
	Result struct {
		Result struct {
			Type        string          `json:"type"`
			Subtype     string          `json:"subtype"`
			Description string          `json:"description"`
			Value       json.RawMessage `json:"value"`
			ObjectID    string          `json:"objectId"`
		} `json:"result"`
		Exception struct {
			Exception struct {
				Value json.RawMessage `json:"value"`
			} `json:"exception"`
		} `json:"exceptionDetails"`
	} `json:"result"`
}

type argMessage struct {
	Type  string          `json:"type"`
	Value json.RawMessage `json:"value"`
}

type consoleAPICalledMessage struct {
	Method string `json:"method"`
	Params struct {
		Type       string          `json:"type"`
		Args       []argMessage    `json:"args"`
		ID         int             `json:"executionContextId"`
		Timestamp  float64         `json:"timestamp"`
		StackTrace json.RawMessage `json:"stackTrace"`
		Context    json.RawMessage `json:"context"`
	} `json:"params"`
}

type pageFrameNavigatedParam struct {
	Method string `json:"method"`
	Params struct {
		Frame struct {
			ID       string `json:"id"`
			ParentID string `json:"parentId"`
			Url      string `json:"url"`
			MimeType string `json:"mimeType"`
		}
		Type string `json:"type"`
	} `json:"params"`
}

type pageLoadEventFired struct {
	Method string `json:"method"`
	Params struct {
		Timestamp float64 `json:"timestamp"`
	} `json:"params"`
}

func loggingConsole(m string) {
	cac_msg := consoleAPICalledMessage{}
	if json.Unmarshal([]byte(m), &cac_msg) == nil && cac_msg.Params.Type != "" {
		log.Println("Console." + cac_msg.Params.Type +
			"(" + args2string(cac_msg.Params.Args) + ")")
	}
}
func args2string(args []argMessage) string {
	strs := make([]string, len(args))
	for i, a := range args {
		strs[i] = string(a.Value)
	}
	return strings.Join(strs, ", ")
}

func (c *chrome) ping() error {
	param := h{}
	_, err := c.send("Target.getTargets", param)
	return err
}

func (c *chrome) pingLoop(done <-chan struct{}) {
	tm := timer.NewTimer()
	defer tm.Stop()

	for {
		tm.Start(15 * time.Second)

		select {
		case <-done:
			return
		case <-tm.Recv():
		}

		c.ping()
	}
}

func (c *chrome) readLoop() {
	last_onshowpage_url := ""
	last_load_timestamp := float64(0)

	done := make(chan struct{}, 1)
	go c.pingLoop(done)
	defer close(done)

	for {
		m := msg{}
		if err := websocket.JSON.Receive(c.ws, &m); err != nil {
			return
		}

		if m.Method == "Target.receivedMessageFromTarget" {
			params := struct {
				SessionID string `json:"sessionId"`
				Message   string `json:"message"`
			}{}
			json.Unmarshal(m.Params, &params)
			if params.SessionID != c.session {
				continue
			}
			res := targetMessage{}
			json.Unmarshal([]byte(params.Message), &res)

			if res.Method == "Runtime.exceptionThrown" {
				log.Println(params.Message)
			} else if res.ID == 0 && res.Method == "Runtime.consoleAPICalled" {
				loggingConsole(params.Message)
			} else if res.ID == 0 && res.Method == "Page.frameResized" {
				if c.onresize != nil {
					go c.onresize()
				}
			} else if res.ID == 0 && res.Method == "Page.frameNavigated" {
				pm := pageFrameNavigatedParam{}
				err := json.Unmarshal([]byte(params.Message), &pm)
				if err == nil && pm.Params.Frame.ParentID == "" {
					if pm.Params.Type == "BackForwardCacheRestore" {
						// Avoid chrome VM initialization problem.
						go c.reload(false, "")
					} else if c.onshowpage != nil {
						if last_onshowpage_url != pm.Params.Frame.Url {
							last_onshowpage_url = pm.Params.Frame.Url
							go c.onshowpage(pm.Params.Frame.Url)
						}
					}
				}
			} else if res.ID == 0 && res.Method == "Page.loadEventFired" {
				if c.onload != nil {
					pm := pageLoadEventFired{}
					err := json.Unmarshal([]byte(params.Message), &pm)
					if err == nil {
						if (pm.Params.Timestamp - last_load_timestamp) >= 0.1 {
							last_load_timestamp = pm.Params.Timestamp
							go c.onload()
						}
					}
				}
			} else if res.ID == 0 && res.Method == "Runtime.bindingCalled" {
				payload := struct {
					Name string            `json:"name"`
					Seq  int               `json:"seq"`
					Args []json.RawMessage `json:"args"`
				}{}
				if err := json.Unmarshal([]byte(res.Params.Payload), &payload); err != nil {
					log.Println("Invalid bindingCalled payload:", res.Params.Name, res.Params.Payload)
					continue
				}

				c.Lock()
				binding, ok := c.bindings[res.Params.Name]
				c.Unlock()

				if ok {
					jsString := func(v interface{}) string { b, _ := json.Marshal(v); return string(b) }
					go func() {
						result, error := "", `""`
						if r, err := binding(payload.Args); err != nil {
							error = jsString(err.Error())
						} else if b, err := json.Marshal(r); err != nil {
							error = jsString(err.Error())
						} else {
							result = string(b)
						}
						expr := fmt.Sprintf(`
							if (%[4]s) {
								window['%[1]s']['errors'].get(%[2]d)(%[4]s);
							} else {
								window['%[1]s']['callbacks'].get(%[2]d)(%[3]s);
							}
							window['%[1]s']['callbacks'].delete(%[2]d);
							window['%[1]s']['errors'].delete(%[2]d);
							`, payload.Name, payload.Seq, result, error)
						c.send("Runtime.evaluate", h{"expression": expr, "contextId": res.Params.ID})
					}()
				}
				continue
			}

			c.Lock()
			resc, ok := c.pending[res.ID]
			delete(c.pending, res.ID)
			c.Unlock()

			if !ok {
				continue
			}

			if res.Error.Message != "" {
				resc <- result{Err: errors.New(res.Error.Message)}
			} else if res.Result.Exception.Exception.Value != nil {
				resc <- result{Err: errors.New(string(res.Result.Exception.Exception.Value))}
			} else if res.Result.Result.Type == "object" && res.Result.Result.Subtype == "error" {
				resc <- result{Err: errors.New(res.Result.Result.Description)}
			} else if res.Result.Result.Type != "" {
				resc <- result{Value: res.Result.Result.Value}
			} else {
				res := targetMessageTemplate{}
				json.Unmarshal([]byte(params.Message), &res)
				resc <- result{Value: res.Result}
			}
		} else if m.Method == "Target.targetDestroyed" {
			params := struct {
				TargetID string `json:"targetId"`
			}{}
			json.Unmarshal(m.Params, &params)
			if params.TargetID == c.target {
				c.kill()
				return
			}
		}
	}
}

func (c *chrome) send(method string, params h) (json.RawMessage, error) {
	id := atomic.AddInt32(&c.id, 1)
	b, err := json.Marshal(h{"id": int(id), "method": method, "params": params})
	if err != nil {
		return nil, err
	}
	resc := make(chan result)
	c.Lock()
	c.pending[int(id)] = resc
	c.Unlock()

	if err := websocket.JSON.Send(c.ws, h{
		"id":     int(id),
		"method": "Target.sendMessageToTarget",
		"params": h{"message": string(b), "sessionId": c.session},
	}); err != nil {
		return nil, err
	}
	res := <-resc
	return res.Value, res.Err
}

func (c *chrome) load(url string) error {
	_, err := c.send("Page.navigate", h{"url": url})
	return err
}

func (c *chrome) eval(expr string) (json.RawMessage, error) {
	return c.send("Runtime.evaluate", h{"expression": expr, "awaitPromise": true, "returnByValue": true})
}

func (c *chrome) setOnResize(onresize func()) {
	c.onresize = onresize
}

func (c *chrome) setOnShowpage(onshowpage func(url string)) {
	c.onshowpage = onshowpage
}

func (c *chrome) setOnLoad(onload func()) {
	c.onload = onload
}

func (c *chrome) bind(name string, f bindingFunc) error {
	c.Lock()
	// check if binding already exists
	_, exists := c.bindings[name]

	c.bindings[name] = f
	c.Unlock()

	if exists {
		// Just replace callback and return, as the binding was already added to js
		// and adding it again would break it.
		return nil
	}

	if _, err := c.send("Runtime.addBinding", h{"name": name}); err != nil {
		return err
	}
	script := fmt.Sprintf(`(() => {
	const bindingName = '%s';
	let name = bindingName;
	const binding = window[bindingName];
	window[bindingName] = async (...args) => {
		const me = window[bindingName];
		let errors = me['errors'];
		let callbacks = me['callbacks'];
		if (!callbacks) {
			callbacks = new Map();
			me['callbacks'] = callbacks;
		}
		if (!errors) {
			errors = new Map();
			me['errors'] = errors;
		}
		const seq = (me['lastSeq'] || 0) + 1;
		me['lastSeq'] = seq;
		const promise = new Promise((resolve, reject) => {
			callbacks.set(seq, resolve);
			errors.set(seq, reject);
		});
		binding(JSON.stringify({name: bindingName, seq: seq, args: args}));
		return promise;
	}})();
	`, name)
	_, err := c.send("Page.addScriptToEvaluateOnNewDocument", h{"source": script})
	if err != nil {
		return err
	}
	_, err = c.eval(script)
	return err
}

func (c *chrome) evalOnNew(expr string) error {
	_, err := c.send("Page.addScriptToEvaluateOnNewDocument", h{"source": expr})
	return err
}

func (c *chrome) reload(super bool, expr string) error {
	param := h{"ignoreCache": super}
	if expr != "" {
		param["scriptToEvaluateOnLoad"] = expr
	}
	_, err := c.send("Page.reload", param)
	return err
}

func (c *chrome) setBounds(b Bounds) error {
	if b.WindowState == "" {
		b.WindowState = WindowStateNormal
	}
	param := h{"windowId": c.window, "bounds": b}
	if b.WindowState != WindowStateNormal {
		param["bounds"] = h{"windowState": b.WindowState}
	}
	_, err := c.send("Browser.setWindowBounds", param)
	return err
}

func (c *chrome) bounds() (Bounds, error) {
	result, err := c.send("Browser.getWindowBounds", h{"windowId": c.window})
	if err != nil {
		return Bounds{}, err
	}
	bounds := struct {
		Bounds Bounds `json:"bounds"`
	}{}
	err = json.Unmarshal(result, &bounds)
	return bounds.Bounds, err
}

func (c *chrome) kill() error {
	if c.ws != nil {
		if err := c.ws.Close(); err != nil {
			return err
		}
	}
	// TODO: cancel all pending requests
	if state := c.cmd.ProcessState; state == nil || !state.Exited() {
		return c.cmd.Process.Kill()
	}
	return nil
}

func readUntilMatch(r io.ReadCloser, re *regexp.Regexp) ([]string, error) {
	br := bufio.NewReader(r)
	for {
		if line, err := br.ReadString('\n'); err != nil {
			r.Close()
			return nil, err
		} else if m := re.FindStringSubmatch(line); m != nil {
			go io.Copy(io.Discard, br)
			return m, nil
		}
	}
}

func contains(arr []string, x string) bool {
	for _, n := range arr {
		if x == n {
			return true
		}
	}
	return false
}

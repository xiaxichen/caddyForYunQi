package file404

import (
	"github.com/caddyserver/caddy"
	"github.com/caddyserver/caddy/caddyhttp/httpserver"
	"os"
	"fmt"
	"path/filepath"
	"strconv"
)


func setup(c *caddy.Controller) error {

	handler, err := errorsParse(c)
	if err != nil {
		return err
	}
	handler.Log.Attach(c)

	if err != nil {
		return err
	}
	// 现在开始注册中间件
	httpserver.GetConfig(c).AddMiddleware(func(next httpserver.Handler) httpserver.Handler {
		handler.next = next
		return handler
	})

	return nil
}

func errorsParse(c *caddy.Controller) (*handler404default, error) {

	handler := &handler404default{
		ErrorPages: make(map[int]string),
		Log:        &httpserver.Logger{},
	} // 用来实现 HTTPHandler 的 next 的结构，用来构建 中间件。也可以加入一些自己的字段

	cfg := httpserver.GetConfig(c)

	optionalBlock := func() error {
		for c.NextBlock() {

			what := c.Val()
			where := c.RemainingArgs()

			if httpserver.IsLogRollerSubdirective(what) {
				var err error
				err = httpserver.ParseRoller(handler.Log.Roller, what, where...)
				if err != nil {
					return err
				}
			} else {
				if len(where) != 1 {
					return c.ArgErr()
				}
				where := where[0]

				// Error page; ensure it exists
				if !filepath.IsAbs(where) {
					where = filepath.Join(cfg.Root, where)
				}

				f, err := os.Open(where)
				if err != nil {
					fmt.Printf("[WARNING] Unable to open error page '%s': %v", where, err)
				}
				f.Close()

				if what == "*" {
					if handler.GenericErrorPage != "" {
						return c.Errf("Duplicate status code entry: %s", what)
					}
					handler.GenericErrorPage = where
				} else {
					whatInt, err := strconv.Atoi(what)
					if err != nil {
						return c.Err("Expecting a numeric status code or '*', got '" + what + "'")
					}

					if _, exists := handler.ErrorPages[whatInt]; exists {
						return c.Errf("Duplicate status code entry: %s", what)
					}

					handler.ErrorPages[whatInt] = where
				}
			}
		}
		return nil
	}

	for c.Next() {
		// weird hack to avoid having the handler values overwritten.
		if c.Val() == "}" {
			continue
		}

		args := c.RemainingArgs()

		if len(args) == 1 {
			switch args[0] {
			default:
				handler.Log.Output = args[0]
				handler.Log.Roller = httpserver.DefaultLogRoller()
			}
		}

		if len(args) > 1 {
			return handler, c.Errf("Only 1 Argument expected for errors directive")
		}

		// Configuration may be in a block
		err := optionalBlock()
		if err != nil {
			return handler, err
		}
	}

	return handler, nil
}


# gomax

`gomax` is a Go port of [PyMax](https://github.com/MaxApiTeam/PyMax), an unofficial API client library for the "MAX" messenger. It was generated with direct reference to the PyMax source (protocol framing, opcodes, request/response payload shapes and login/auth flows) and reimplements the same TCP/msgpack and WebSocket/JSON protocols, session persistence, SMS/QR authentication, message/chat/user APIs and event dispatching idiomatically in Go, using goroutines and channels in place of `asyncio`. This project is not affiliated with or endorsed by Max, MaxApiTeam, or the maintainers of PyMax.

## Install

```sh
go get github.com/SonChegg/PyMax
```

## Usage

```go
package main

import (
	"context"
	"fmt"
	"log"

	gomax "github.com/SonChegg/PyMax"
	"github.com/SonChegg/PyMax/types"
)

func main() {
	client := gomax.NewClient("+79990000000", gomax.Config{
		WorkDir: ".",
	})

	client.OnMessage(func(ctx context.Context, msg *types.Message, c *gomax.Client) error {
		if msg.ChatID == nil {
			return nil
		}
		fmt.Println("received:", msg.Text)
		_, err := c.API().Messages.SendMessage(ctx, *msg.ChatID, "Got it!", 0, nil, true, nil)
		return err
	})

	if err := client.Start(context.Background()); err != nil {
		log.Fatal(err)
	}
}
```

The first run prompts for the SMS code on stdin and persists the resulting session to `./session.db` (SQLite via `modernc.org/sqlite`, no cgo required); subsequent runs reuse it. `gomax.NewWebClient` provides the WebSocket/QR-login counterpart to `Client`.

## License

MIT, see [LICENSE](LICENSE) (carried over unmodified from the original PyMax project).

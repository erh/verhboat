package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/google/uuid"

	"go.viam.com/rdk/logging"

	"github.com/erh/verhboat/utils"
)

func main() {
	err := realMain()
	if err != nil {
		panic(err)
	}
}

func realMain() error {
	ctx := context.Background()
	logger := logging.NewLogger("onehelm")

	dir := flag.String("dir", "", "")
	port := flag.Int("port", 8888, "")
	name := flag.String("name", "Demo App", "")
	iconPath := flag.String("icon", "https://avatars.githubusercontent.com/u/71797972?s=96", "")

	flag.Parse()

	if *dir == "" {
		return fmt.Errorf("need a dir")
	}

	id := uuid.NewSHA1(uuid.MustParse("FF430B53-2287-401B-A4B3-0A6CD3A092E6"),
		[]byte(fmt.Sprintf("%s-%d-%s-%s", *dir, *port, *name, *iconPath)))

	fmt.Printf("id: %v\n", id)

	_, start, err := utils.PrepOnehelmServer(os.DirFS(*dir), logger, &utils.OnehelmAppConfig{
		Port:     *port,
		AppName:  *name,
		AppId:    id.String(),
		IconPath: *iconPath,
	})
	if err != nil {
		return err
	}

	closer := start(ctx)

	time.Sleep(10 * time.Second)
	closer(ctx)

	return nil
}

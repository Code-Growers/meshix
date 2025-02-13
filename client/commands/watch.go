package commands

import (
	"fmt"
	"strings"

	"github.com/fsnotify/fsnotify"
	"golang.org/x/net/context"
)

type WatchCommand struct {
	Cache string `long:"cache" description:"Cache to push artifacts to, if not specified nothing is pushed"`
}

const (
	lockSuffix   = ".lock"
	sourceSuffix = "-source"
	drvSuffix    = ".drv"
)

func (x *WatchCommand) Execute(args []string) error {
	store := "/nix/store/"
	ctx := context.Background()

	err := WatchStore(ctx, store, x.Cache)
	if err != nil {
		return err
	}

	return nil
}

var bufSize = 1000 // TODO move somewhere

// Watches the store
func WatchStore(ctx context.Context, storePath string, cacheUrl string) error {
	fmt.Printf("Starting watcher on nix store %s\n", storePath)

	// Create new watcher.
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}
	defer watcher.Close()

	// Add a path.
	err = watcher.Add(storePath)
	if err != nil {
		return err
	}

	builedChan := make(chan string, bufSize)
	go func() {
		uploadCtx := context.Background()
		for {
			select {
			case <-ctx.Done():
				return
			case path := <-builedChan:
				err := uploadPath(uploadCtx, path, cacheUrl)
				if err != nil {
					fmt.Printf("Upload error: %v\n", err)
				}
			}
		}
	}()

outer:
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case event, ok := <-watcher.Events:
			if !ok {
				break outer
			}
			if event.Has(fsnotify.Remove) {
				fileName := event.Name
				if !strings.HasSuffix(fileName, lockSuffix) {
					continue
				}

				path := strings.TrimSuffix(fileName, lockSuffix)
				if strings.HasSuffix(path, sourceSuffix) || strings.HasSuffix(path, drvSuffix) {
					continue
				}
				builedChan <- path
			}
		case err, ok := <-watcher.Errors:
			if !ok {
				break outer
			}
			fmt.Printf("error: %v\n", err)
		}
	}

	err = watcher.Close()
	if err != nil {
		return err
	}

	return nil
}

func uploadPath(ctx context.Context, path string, cacheUrl string) error {
	fmt.Printf("Uploading path: %s\n", path)
	_, err := runNixCmd(ctx, "nix", "copy", "--to", cacheUrl, path)
	if err != nil {
		return fmt.Errorf("Failed to push to binary cache: %w", err)
	}
	fmt.Printf("Done uploading path: %s\n", path)
	return nil
}

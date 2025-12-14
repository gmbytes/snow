package sources

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"path"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/mogud/snow/core/configuration"
)

var _ configuration.IConfigurationSource = (*FileConfigurationSource)(nil)

type FileConfigurationSource struct {
	Path           string
	Optional       bool
	ReloadOnChange bool
}

func (ss *FileConfigurationSource) BuildConfigurationProvider(_ configuration.IConfigurationBuilder) configuration.IConfigurationProvider {
	return NewFileConfigurationProvider(ss)
}

var _ configuration.IConfigurationProvider = (*FileConfigurationProvider)(nil)

type FileConfigurationProvider struct {
	*configuration.Provider

	path           string
	optional       bool
	reloadOnChange bool
	loaded         bool
	loadLock       sync.Mutex

	watcher *fsnotify.Watcher
	ctx     context.Context
	cancel  context.CancelFunc

	OnLoad func(bytes []byte)
}

func NewFileConfigurationProvider(source *FileConfigurationSource) *FileConfigurationProvider {
	ctx, cancel := context.WithCancel(context.Background())
	return &FileConfigurationProvider{
		Provider:       configuration.NewProvider(),
		path:           source.Path,
		optional:       source.Optional,
		reloadOnChange: source.ReloadOnChange,
		ctx:            ctx,
		cancel:         cancel,
		OnLoad:         func(bytes []byte) {},
	}
}

func (ss *FileConfigurationProvider) Load() {
	ss.loadLock.Lock()
	defer ss.loadLock.Unlock()

	if ss.loaded {
		if ss.reloadOnChange {
			return
		}

		ss.loadFile()
		ss.OnReload()
		return
	}
	ss.loadFile()
	ss.loaded = true

	if ss.reloadOnChange {
		watcher, err := fsnotify.NewWatcher()
		if err != nil {
			log.Printf("failed to create file watcher: %v", err)
			return
		}
		ss.watcher = watcher

		go func() {
			defer watcher.Close()
			var uniqueRead int32
			for {
				select {
				case <-ss.ctx.Done():
					return
				case event, ok := <-watcher.Events:
					if !ok {
						return
					}
					if event.Has(fsnotify.Write | fsnotify.Create) {
						if pathEquals(event.Name, ss.path) && atomic.CompareAndSwapInt32(&uniqueRead, 0, 1) {
							log.Printf("file watcher received Write Or Create: %v", event.Name)
							go func() {
								<-time.After(500 * time.Millisecond)
								atomic.StoreInt32(&uniqueRead, 0)
								ss.loadLock.Lock()
								ss.loadFile()
								ss.loadLock.Unlock()
								ss.OnReload()
							}()
						}
					} else if event.Has(fsnotify.Remove) {
						if pathEquals(event.Name, ss.path) {
							log.Printf("file watcher received Rename or Remove: %v", event.Name)
							ss.loadLock.Lock()
							ss.Replace(configuration.NewCaseInsensitiveStringMap[string]())
							ss.loadLock.Unlock()
							ss.OnReload()
						}
					}
				case err, ok := <-watcher.Errors:
					if err != nil {
						log.Println("file watcher error:", err)
					}
					if !ok {
						return
					}
				}
			}
		}()

		err = watcher.Add(ss.path)
		if err != nil {
			parentDir := path.Dir(ss.path)

			log.Printf("file watcher cannot watch file(%v), try parent(%v)...", ss.path, parentDir)

			err = os.MkdirAll(parentDir, os.ModeDir)
			if err != nil {
				log.Printf("create watcher path(%v) failed: %v", parentDir, err.Error())
				watcher.Close()
				ss.watcher = nil
				return
			}
			err = watcher.Add(parentDir)
			if err != nil {
				log.Printf("cannot watch path(%v): %v", parentDir, err.Error())
				watcher.Close()
				ss.watcher = nil
				return
			}
		}
	}
}

// Close 关闭文件监听器，释放资源
func (ss *FileConfigurationProvider) Close() error {
	ss.loadLock.Lock()
	defer ss.loadLock.Unlock()

	if ss.cancel != nil {
		ss.cancel()
	}
	if ss.watcher != nil {
		err := ss.watcher.Close()
		ss.watcher = nil
		return err
	}
	return nil
}

func (ss *FileConfigurationProvider) loadFile() {
	if !ss.optional {
		if _, err := os.Stat(ss.path); errors.Is(err, os.ErrNotExist) {
			panic(fmt.Sprintf("file not found: %v", ss.path))
		}
	}

	data, err := os.ReadFile(ss.path)
	if err != nil {
		if !ss.optional {
			log.Printf("read file error: %v", err.Error())
		}
		return
	}

	ss.OnLoad(data)
}

func pathEquals(path1, path2 string) bool {
	p1, p2 := strings.Replace(path1, "\\", "/", -1), strings.Replace(path2, "\\", "/", -1)
	return p1 == p2
}

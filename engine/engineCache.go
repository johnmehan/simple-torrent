package engine

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/anacrolix/torrent/metainfo"
)

const (
	cacheSavedPrefix = "_CLDAUTOSAVED_"
)

func (e *Engine) newMagnetCacheFile(magnetURI, infohash string) {
	// create .info file with hash as filename
	if w, err := os.Stat(e.cacheDir); err == nil && w.IsDir() {
		cacheInfoPath := filepath.Join(e.cacheDir,
			fmt.Sprintf("%s%s.info", cacheSavedPrefix, infohash))
		if _, err := os.Stat(cacheInfoPath); os.IsNotExist(err) {
			cf, err := os.Create(cacheInfoPath)
			defer cf.Close()
			if err == nil {
				cf.WriteString(magnetURI)
				log.Println("created magnet cache info file", infohash)
			}
		}
	}
}

func (e *Engine) newTorrentCacheFile(meta *metainfo.MetaInfo) {
	// create .torrent file
	infohash := meta.HashInfoBytes().HexString()
	if w, err := os.Stat(e.cacheDir); err == nil && w.IsDir() {
		cacheFilePath := filepath.Join(e.cacheDir,
			fmt.Sprintf("%s%s.torrent", cacheSavedPrefix, infohash))
		// only create the cache file if not exists
		// avoid recreating cache files during boot import
		if _, err := os.Stat(cacheFilePath); os.IsNotExist(err) {
			cf, err := os.Create(cacheFilePath)
			defer cf.Close()
			if err == nil {
				meta.Write(cf)
				log.Println("created torrent cache file", infohash)
			} else {
				log.Println("failed to create torrent file ", err)
			}
		}
	}
}

func (e *Engine) removeMagnetCache(infohash string) {
	// remove both magnet and torrent cache if exists.
	cacheInfoPath := filepath.Join(e.cacheDir,
		fmt.Sprintf("%s%s.info", cacheSavedPrefix, infohash))
	if err := os.Remove(cacheInfoPath); err == nil {
		log.Printf("removed magnet info file %s", infohash)
	}
}

func (e *Engine) removeTorrentCache(infohash string) {
	cacheFilePath := filepath.Join(e.cacheDir,
		fmt.Sprintf("%s%s.torrent", cacheSavedPrefix, infohash))
	if err := os.Remove(cacheFilePath); err == nil {
		log.Printf("removed torrent file %s", infohash)
	} else {
		log.Printf("fail to removed torrent file %s, %s", infohash, err)
	}
}

func (e *Engine) RestoreTask(fn string) error {

	if strings.HasSuffix(fn, ".torrent") {
		if err := e.NewTorrentByFilePath(fn); err == nil {
			if strings.HasPrefix(filepath.Base(fn), cacheSavedPrefix) {
				log.Printf("[RestoreTask] Restored Torrent: %s \n", fn)
			} else {
				log.Printf("Task: added %s, file removed\n", fn)
				os.Remove(fn)
			}
		} else {
			log.Printf("RestoreTask: fail to add %s, ERR:%s\n", fn, err)
			return err
		}
	}
	if strings.HasSuffix(fn, ".info") && strings.HasPrefix(fn, cacheSavedPrefix) && len(fn) == 59 {
		mag, err := ioutil.ReadFile(fn)
		if err != nil {
			log.Printf("Task: fail to read %s\n", fn)
			return err
		}
		if err := e.NewMagnet(string(mag)); err == nil {
			log.Printf("[RestoreMagnet] Restored: %s \n", fn)
		} else {
			log.Printf("RestoreTask: fail to add %s, ERR:%s\n", fn, err)
			return err
		}
	}

	return nil
}

func (e *Engine) RestoreCacheDir() {

	files, err := ioutil.ReadDir(e.cacheDir)
	if err != nil {
		log.Println("RestoreCacheDir failed read cachedir ", err)
		return
	}

	// sort by modtime
	sort.Slice(files, func(i, j int) bool {
		return files[i].ModTime().Before(files[j].ModTime())
	})

	for _, i := range files {
		if i.IsDir() {
			continue
		}
		e.RestoreTask(path.Join(e.cacheDir, i.Name()))
	}
}

func (e *Engine) NextWaitTask() error {
	if elm := e.waitList.Pop(); elm != nil {
		var res string
		te := elm.(taskElem)
		switch te.tp {
		case taskTorrent:
			res = fmt.Sprintf("%s%s.torrent", cacheSavedPrefix, te.ih)
		case taskMagnet:
			res = fmt.Sprintf("%s%s.info", cacheSavedPrefix, te.ih)
		}

		fn := path.Join(e.cacheDir, res)
		if _, err := os.Stat(fn); err != nil {
			log.Println("nextWaitTask RestoreTask:", fn, err)
			return err
		}
		return e.RestoreTask(fn)
	}

	log.Println("nextWaitTask: wait list empty")
	return ErrWaitListEmpty
}

func (e *Engine) pushWaitTask(ih string, tp taskType) {
	e.waitList.Push(taskElem{ih: ih, tp: tp})
}

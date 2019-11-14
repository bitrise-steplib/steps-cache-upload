// Cache descriptor file related models and functions.
package main

import (
	"crypto/md5"
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/bitrise-io/go-utils/fileutil"
	"github.com/bitrise-io/go-utils/log"
	"github.com/bitrise-io/go-utils/pathutil"
)

// ChangeIndicator ...
type ChangeIndicator string

const (
	// MD5 ...
	MD5 = ChangeIndicator("file-content-hash")
	// MODTIME ...
	MODTIME = ChangeIndicator("file-mod-time")
)

// result stores how the keys are different in two cache descriptor.
type result struct {
	removedIgnored []string
	removed        []string
	changed        []string
	matching       []string
	addedIgnored   []string
	added          []string
}

// hasChanges reports whether a new cache needs to be generated or not.
func (r result) hasChanges() bool {
	return len(r.removed) > 0 || len(r.changed) > 0 || len(r.added) > 0
}

// compare compares two cache descriptor file and return the differences.
func compare(prevPathToIndicator map[string]string, currentPathToIndicator map[string]string) (r result) {
	for oldPth, oldIndicator := range prevPathToIndicator {
		newIndicator, ok := currentPathToIndicator[oldPth]
		switch {
		case !ok && oldIndicator == "-":
			r.removedIgnored = append(r.removedIgnored, oldPth)
		case !ok:
			r.removed = append(r.removed, oldPth)
		case oldIndicator != newIndicator:
			r.changed = append(r.changed, oldPth)
		default:
			r.matching = append(r.matching, oldPth)
		}

		delete(currentPathToIndicator, oldPth)
	}

	for newPth, newIndicator := range currentPathToIndicator {
		if newIndicator == "-" {
			r.addedIgnored = append(r.addedIgnored, newPth)
		} else {
			r.added = append(r.added, newPth)
		}
	}

	return
}

// cacheDescriptor creates a cache descriptor for a given change_indicator_path - cache_path (single-multiple) mapping.
func cacheDescriptor(pathToIndicatorFile map[string]string, method ChangeIndicator) (map[string]string, error) {
	indicatorPathToIndicator := map[string]string{}
	pathToIndicator := map[string]string{}

	for path, indicatorPath := range pathToIndicatorFile {
		if len(indicatorPath) == 0 {
			// this file's changes does not fluctuates existing cache invalidation
			pathToIndicator[path] = "-"
			continue
		}

		indicatorValue, ok := indicatorPathToIndicator[indicatorPath]
		if ok {
			pathToIndicator[path] = indicatorValue
			continue
		}

		var indicator string
		var err error
		if method == MD5 {
			indicator, err = fileContentHash(indicatorPath)
		} else {
			indicator, err = fileModtime(indicatorPath)
		}
		if err != nil {
			return nil, err
		}

		pathToIndicator[path] = indicator
	}
	return pathToIndicator, nil
}

// fileContentHash returns file's md5 content hash.
func fileContentHash(pth string) (string, error) {
	f, err := os.Open(pth)
	if err != nil {
		return "", err
	}

	defer func() {
		if err := f.Close(); err != nil {
			log.Errorf("Failed to close file (%s), error: %+v", pth, err)
		}
	}()

	// #nosec G401 Ignore gosec warning: Use of weak cryptographic primitive
	h := md5.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}

	return fmt.Sprintf("%x", h.Sum(nil)), nil
}

// fileModtime returns a file's modtime as a Unix timestamp representation.
func fileModtime(pth string) (string, error) {
	fi, err := os.Stat(pth)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%d", fi.ModTime().Unix()), nil
}

// readCacheDescriptor reads cache descriptor from pth is exists.
func readCacheDescriptor(pth string) (map[string]string, error) {
	if exists, err := pathutil.IsPathExists(pth); err != nil {
		return nil, err
	} else if !exists {
		return nil, nil
	}

	fileBytes, err := fileutil.ReadBytesFromFile(pth)
	if err != nil {
		return nil, err
	}

	var previousFilePathMap map[string]string
	err = json.Unmarshal(fileBytes, &previousFilePathMap)
	if err != nil {
		return nil, err
	}

	return previousFilePathMap, nil
}

// convertDescriptorToIndicatorMap converts the descriptor to an indicator map (from path - indicator to
// indicator - paths slice.
func convertDescriptorToIndicatorMap(indicatorByPth map[string]string) map[string][]string {
	var indicatorMap = map[string][]string{}
	for pth, indicator := range indicatorByPth {
		indicatorMap[indicator] = append(indicatorMap[indicator], pth)
	}
	return indicatorMap
}

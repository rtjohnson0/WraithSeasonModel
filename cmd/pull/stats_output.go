package main

import (
	"bufio"
	"io"
	"os"
	"path/filepath"
)

// statsDownload stages the entire refresh beside the current file. A failed
// request never truncates or partially appends the last usable stats file.
type statsDownload struct {
	*output
	target, temp string
	finished     bool
}

func beginStatsDownload(directory, name string, appendMode bool) (*statsDownload, error) {
	if e := os.MkdirAll(directory, 0755); e != nil {
		return nil, e
	}
	f, e := os.CreateTemp(directory, "."+name+"-*.download")
	if e != nil {
		return nil, e
	}
	d := &statsDownload{output: &output{f: f, w: bufio.NewWriterSize(f, 1<<20)}, target: filepath.Join(directory, name), temp: f.Name()}
	if appendMode {
		old, e := os.Open(d.target)
		if e != nil {
			d.Abort()
			return nil, e
		}
		_, e = io.Copy(d.w, old)
		old.Close()
		if e != nil {
			d.Abort()
			return nil, e
		}
	}
	return d, nil
}
func (d *statsDownload) Abort() {
	if d.finished {
		return
	}
	_ = d.f.Close()
	_ = os.Remove(d.temp)
	d.finished = true
}
func (d *statsDownload) Commit() error {
	if e := d.output.Close(); e != nil {
		d.Abort()
		return e
	}
	if e := os.Rename(d.temp, d.target); e != nil {
		d.Abort()
		return e
	}
	d.finished = true
	return nil
}

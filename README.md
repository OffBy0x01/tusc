# OffBy0x01's Opinionated fork of go-tus

adds checksum extension, custom fingerprints and various critical bug fixes

A pure Go client for the [tus resumable upload protocol](http://tus.io/)

## Example

```go
package main

import (
    "os"
	"crypto/sha1"
    "github.com/offby0x01/tusc"
)

func main() {
	// open file to upload
    f, err := os.Open("my-file.txt")
    if err != nil {
        panic(err)
    }
    defer f.Close()

	// [optional] create hash.Hash + name of alg 
	hasher := sha1.New()
	// [...cont] configure client config
	config := &tusc.Config{
		ChunkSizeBytes: 5 * 1024 * 1024,
		ChecksumAlg:    "sha1", // must match name of hasher
		ChecksumFunc:   hasher,
	}

    // create the tus client.
    client, _ := tusc.NewClient("https://tus.example.org/files", config)
    
	// define a fingerprint for the file - ideally a short hash, but can be anything
	fingerprint := "anything"
	
    // create an upload from a file + fingerprint.
    upload, _ := tusc.NewUploadFromFile(f, &fingerprint)

    // create the uploader.
    uploadMgr, _ := client.CreateUpload(upload)

    // start the uploading process.
	uploadMgr.Upload()
}
```

## Features

> This is not a full protocol client implementation.

Checksum, Termination and Concatenation extensions are not implemented yet.

This client allows to resume an upload if a Store is used.

## Built in Store

Store is used to map an upload's fingerprint with the corresponding upload URL.

| Name | Backend | Dependencies |
|:----:|:-------:|:------------:|
| MemoryStore  | In-Memory | None |
| LeveldbStore | LevelDB   | [goleveldb](https://github.com/syndtr/goleveldb) |

## Future Work

- [ ] SQLite store
- [ ] Redis store
- [x] Memcached store
- [ ] Checksum extension
- [ ] Termination extension
- [x] Concatenation extension
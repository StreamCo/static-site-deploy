package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"io"
	"log"
	"mime"
	"net/http"
	"net/http/httputil"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
)

type output interface {
	PutReader(key string, r io.ReadSeeker, contentType string) error
}

func deploy(localFolder, path string, remote output) error {
	key, err := filepath.Rel(localFolder, path)
	if err != nil {
		return err
	}
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	if err := remote.PutReader(key, f, mime.TypeByExtension(filepath.Ext(key))+"; charset=utf-8"); err != nil {
		return err
	}
	return nil
}

func main() {
	if len(os.Args) <= 1 {
		log.Fatal("Usage: static-site-deploy <localFolder>")
	}
	localFolder := os.Args[1]
	var remote output
	if bucket := os.Getenv("S3_BUCKET"); bucket != "" {
		sess := session.New()
		s3Client := s3.New(sess, nil)
		remote = &S3Output{s3Client, bucket, ""}
	} else if netstorageHost := os.Getenv("NETSTORAGE_HOST"); netstorageHost != "" {
		remote = &NetstorageOutput{
			Host:              netstorageHost,
			Folder:            os.Getenv("NETSTORAGE_FOLDER"),
			NetstorageKeyName: os.Getenv("NETSTORAGE_UPLOAD_KEY_NAME"),
			NetstorageSecret:  os.Getenv("NETSTORAGE_UPLOAD_SECRET"),
		}
	} else {
		log.Fatal("Either a netstorage or s3 remote should be configured in the env.")
	}
	var (
		htmlFiles  []string
		otherFiles []string
	)
	if err := filepath.Walk(localFolder, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		if strings.HasSuffix(path, ".html") {
			htmlFiles = append(htmlFiles, path)
		} else {
			otherFiles = append(otherFiles, path)
		}
		return nil
	}); err != nil {
		log.Fatal(err)
	}

	for _, path := range otherFiles {
		if err := deploy(localFolder, path, remote); err != nil {
			log.Fatal(err)
		}
	}
	for _, path := range htmlFiles {
		if err := deploy(localFolder, path, remote); err != nil {
			log.Fatal(err)
		}
	}
}

// A NetstorageOutput uploads to Akamai Netstorage using HTTP:
// https://control.akamai.com/dl/customers/NS/NS_http_api_FS.pdf
// (login required)
type NetstorageOutput struct {
	Host              string
	Folder            string
	Prefix            string
	BaseURL           string
	NetstorageKeyName string
	NetstorageSecret  string
}

func (o *NetstorageOutput) SetPrefix(key string) {
	o.Prefix = key
}

func (o *NetstorageOutput) URLFor(p string) string {
	return fmt.Sprintf("%s/%s.json", o.BaseURL, path.Join(o.Prefix, p))
}

func (o *NetstorageOutput) auth(r *http.Request, id string, filename string, unixTime int64) {
	action := "version=1&action=upload"
	r.Header.Set("X-Akamai-ACS-Action", action)
	authData := fmt.Sprintf("5, 0.0.0.0, 0.0.0.0, %d, %s, %s", unixTime, id, o.NetstorageKeyName)
	r.Header.Set("X-Akamai-ACS-Auth-Data", authData)
	hash := hmac.New(sha256.New, []byte(o.NetstorageSecret))
	fmt.Fprintf(hash, "%s/%s\nx-akamai-acs-action:%s\n", authData, filename, action)
	r.Header.Set("X-Akamai-ACS-Auth-Sign", base64.StdEncoding.EncodeToString(hash.Sum(nil)))
}

func (o *NetstorageOutput) PutReader(key string, r io.ReadSeeker, contentType string) error {
	filename := path.Join(o.Folder, o.Prefix, key)
	req, err := http.NewRequest("PUT", fmt.Sprintf("http://%s/%s", o.Host, filename), r)
	if err != nil {
		return err
	}
	o.auth(req, filename, filename, time.Now().Unix())
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		dump, _ := httputil.DumpResponse(resp, true)
		return fmt.Errorf("unexpected response code %d when uploading %s. Here's a dump of the response:\n%s", resp.StatusCode, filename, string(dump))
	}
	log.Printf("output: put %s", filename)
	return nil
}

func (o *NetstorageOutput) Delete(key string) error {
	filename := path.Join(o.Folder, o.Prefix, key)
	req, err := http.NewRequest("DELETE", fmt.Sprintf("http://%s/%s", o.Host, filename), nil)
	if err != nil {
		return err
	}
	o.auth(req, filename, filename, time.Now().Unix())
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		dump, _ := httputil.DumpResponse(resp, true)
		return fmt.Errorf("unexpected response code %d when uploading %s. Here's a dump of the response:\n%s", resp.StatusCode, filename, string(dump))
	}
	log.Printf("output: delete %s", filename)
	return nil
}

// An s3Output implements Output to a provided s3 bucket with the provided
// prefix.
type S3Output struct {
	Client *s3.S3
	Bucket string
	Prefix string
}

func (o *S3Output) SetPrefix(key string) {
	o.Prefix = key
}

func (o *S3Output) URLFor(p string) string {
	return fmt.Sprintf("http://%s/%s.json", o.Bucket, path.Join(o.Prefix, p))
}

func (o *S3Output) PutReader(key string, r io.ReadSeeker, contentType string) error {
	filename := path.Join(o.Prefix, key)
	if _, err := o.Client.PutObject(&s3.PutObjectInput{
		Bucket:      aws.String(o.Bucket),
		Key:         aws.String(filename),
		Body:        r,
		ContentType: aws.String(contentType),
		ACL:         aws.String("public-read"),
	}); err != nil {
		return err
	}
	log.Printf("output: put %s", filename)
	return nil
}

func (o *S3Output) Delete(key string) error {
	filename := path.Join(o.Prefix, key)
	if _, err := o.Client.DeleteObject(&s3.DeleteObjectInput{
		Bucket: aws.String(o.Bucket),
		Key:    aws.String(filename),
	}); err != nil {
		return err
	}
	log.Printf("output: delete %s", filename)
	return nil
}

package main

import (
	"bufio"
	"crypto/md5"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
)

var (
	user     string
	secret   string
	endpoint string
	region   string
	bucket   string
	id       string
	ip       string
	key      string
	tmpdir   string
	dryrun   bool
	ignore   bool
)

func checkErr(err error) {
	if err != nil {
		if awsErr, ok := err.(awserr.Error); ok {
			log.Println("Error:", awsErr.Code(), awsErr.Message(), awsErr.Error())
		} else {
			log.Println(err)
		}
		os.Exit(1)
	}
}

func GetS3Client(user, secret, endpoint, region string) (*s3.S3, error) {
	s3Config := &aws.Config{
		Credentials: credentials.NewStaticCredentials(user, secret, ""),
		Endpoint:    aws.String(endpoint),
		Region:      aws.String(region),
	}

	newSession, err := session.NewSession(s3Config)
	if err != nil {
		return nil, fmt.Errorf("Failed to create S3 session")
	}

	// Create S3 Client
	return s3.New(newSession), nil
}

func main() {
	flag.StringVar(&user, "user", "", "user")
	flag.StringVar(&secret, "secret", "", "secret")
	flag.StringVar(&endpoint, "endpoint", "", "endpoint")
	flag.StringVar(&region, "region", "us-west-2", "region")
	flag.StringVar(&bucket, "bucket", "", "bucket")
	flag.StringVar(&key, "key", "", "key")
	flag.StringVar(&id, "id", "", "id")
	flag.StringVar(&ip, "ip", "", "ip")
	flag.StringVar(&tmpdir, "tmpdir", "", "tmpdir")
	flag.BoolVar(&dryrun, "dryrun", true, "dryrun")
	flag.BoolVar(&ignore, "ignore", false, "ignore")
	flag.Parse()

	s3client, err := GetS3Client(user, secret, endpoint, region)
	checkErr(err)

	if id != "" {
		if name := getNameById(id, ip); name == "" {
			log.Fatal("failed to get name for id ", id)
		} else {
			key = name
		}
	}

	log.Println("object name :", key)

	resp, err := s3client.GetObject(
		&s3.GetObjectInput{
			Bucket: aws.String(bucket),
			Key:    aws.String(key),
		})
	checkErr(err)

	size := *resp.ContentLength
	if size == 0 {
		log.Println("  object size is 0. Skip check")
		return
	}
	defer resp.Body.Close()

	etag := getEtag(resp.ETag)

	var ctype string
	if resp.ContentType != nil {
		ctype = *resp.ContentType
	}

	base := tmpdir + "/" + user + "/" + bucket
	if _, err := os.Stat(base); os.IsNotExist(err) {
		os.MkdirAll(base, os.ModePerm)
	}

	mkey := strings.Replace(key, "/", "_", -1)
	localpath := base + "/" + mkey + "_orig"
	origmd5, err := writeToPath(localpath, resp.Body)
	checkErr(err)
	resp.Body.Close()
	log.Println("Step 1: Read Object Done")

	if origmd5 != etag {
		if !ignore {
			log.Fatal("etag doesn't match md5 ", etag, " vs ", origmd5)
		}
		log.Println("Step 2: etag doesn't match md5 ", etag, " vs ", origmd5)
	} else {
		log.Println("Step 2: Etag Matches MD5")
	}

	if dryrun {
		log.Println("Dryrun mode, skip remaining steps")
		return
	}

	_, err = s3client.DeleteObject(
		&s3.DeleteObjectInput{
			Bucket: aws.String(bucket),
			Key:    aws.String(key),
		})
	if !ignore {
		checkErr(err)
	}
	log.Println("Step 3: Delete Object Done")

	ifile, err := os.OpenFile(localpath, os.O_RDONLY, 0444)
	checkErr(err)
	defer ifile.Close()
	_, err = s3client.PutObject(
		&s3.PutObjectInput{
			Bucket:      aws.String(bucket),
			Key:         aws.String(key),
			ContentType: aws.String(ctype),
			Body:        ifile,
		})
	checkErr(err)
	ifile.Close()
	log.Println("Step 4: Upload Object Done")

	newresp, err := s3client.GetObject(
		&s3.GetObjectInput{
			Bucket: aws.String(bucket),
			Key:    aws.String(key),
		})
	checkErr(err)

	newsize := *newresp.ContentLength

	if newsize != size {
		log.Fatal("  object size doesn't match with original one ", newsize, " vs ", size)
	}
	defer newresp.Body.Close()

	newetag := getEtag(newresp.ETag)

	newlocalpath := base + "/" + mkey + "_RewriteReadback"
	newmd5, err := writeToPath(newlocalpath, newresp.Body)
	checkErr(err)
	if newmd5 != newetag {
		log.Fatal("new etag doesn't match new md5 ", newetag, " vs ", newmd5)
	}
	if etag != newetag {
		log.Fatal("etag doesn't match new etag ", etag, " vs ", newetag)
	}
	log.Println("Step 5: Etag Matches MD5, and new Etag Matches previous Etag as well")
}

func writeToPath(path string, r io.Reader) (string, error) {
	ofile, err := os.Create(path)
	if err != nil {
		return "", err
	}
	defer ofile.Close()

	h := md5.New()
	w := io.MultiWriter(ofile, h)
	if _, err = io.Copy(w, r); err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", h.Sum(nil)), nil
}

func getEtag(e *string) string {
	if e == nil {
		log.Fatal("no etag found")
	}
	if len(*e) < 2 {
		log.Fatal("invalid etag ", *e)
	}
	return (*e)[1 : len(*e)-1]
}

func getNameById(id, ip string) string {
	resp, err := http.Get(fmt.Sprintf("http://%s:9101/diagnostic/OB/0/DumpAllKeys/OBJECT_TABLE_KEY?type=UPDATE&objectId=%s&useStyle=raw&showvalue=gpb", ip, id))
	checkErr(err)
	reader := bufio.NewReader(resp.Body)
	var last, queryurl string
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			break
		}
		if strings.HasPrefix(line, "schemaType OBJECT_TABLE_KEY objectId") {
			queryurl = strings.TrimRight(last, "\r\n")
			break
		}
		last = line
	}
	resp.Body.Close()

	if queryurl == "" {
		return ""
	}

	qresp, err := http.Get(queryurl)
	checkErr(err)
	defer qresp.Body.Close()
	body, err := ioutil.ReadAll(qresp.Body)
	checkErr(err)
	msg := string(body)
	if idx := strings.Index(msg, "key: \"object-name\""); idx == -1 {
		return ""
	} else {
		msg = msg[idx+len("key: \"object-name\""):]
	}
	if parts := strings.SplitN(msg, "\"", 3); len(parts) > 2 {
		return parts[1]
	}
	return ""
}

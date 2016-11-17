Deploy a static site to s3 or Akamai netstorage, controlled by env variables.

# Prerequisites

- [Go 1.4](https://golang.org/doc/install)

# Setup

    go get github.com/streamco/static-site-deploy

To publish to an S3 bucket, set `AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY`, and `S3_BUCKET`.

If you want your objects to be publicly readable, set `S3_ACL` to `public-read`
(the default setting just uses the bucket ACL).

IAM credentials are also supported.

To publish to a Netstorage endpoint, set the following, e.g.:

    NETSTORAGE_HOST=mystoragegrp-nsu.akamaihd.net
    NETSTORAGE_FOLDER=330000/dev
    NETSTORAGE_UPLOAD_KEY_NAME=myAccount
    NETSTORAGE_UPLOAD_SECRET=23948abbed4

# Running

If the folder you want to deploy is `dist/`, set the required env variables, and run:

    static-site-deploy dist

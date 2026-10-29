package config

import (
	"os"
)

type StorageConfig struct {
	AccessKeyID     string
	SecretAccessKey string
	EndpointURL     string
	Region          string
	Bucket          string
}

var Storage StorageConfig

func InitStorage() StorageConfig {
	Storage = StorageConfig{
		AccessKeyID:     os.Getenv("AWS_ACCESS_KEY_ID"),
		SecretAccessKey: os.Getenv("AWS_SECRET_ACCESS_KEY"),
		EndpointURL:     os.Getenv("AWS_ENDPOINT_URL_S3"),
		Region:          os.Getenv("AWS_REGION"),
		Bucket:          os.Getenv("AWS_BUCKET_NAME"),
	}
	if Storage.Bucket == "" {
		Storage.Bucket = "assets"
	}
	return Storage
}

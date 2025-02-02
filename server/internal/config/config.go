package config

import (
	"errors"
	"net/url"
	"os"

	"github.com/alecthomas/kong"
	"github.com/nix-community/go-nix/pkg/narinfo/signature"
	"gopkg.in/yaml.v3"
)

type cli struct {
	ConfigPath         string `kong:"name='config',help='Path to config file',default='./configuration/config.yaml'"`
	ListenAddr         string `kong:"name='listen',help='address and port to listen on',default='0.0.0.0:8088'"`
	SecretKey          string `kong:"name='secret-key',help='Binary cache secret key',env='SECRET_KEY'"`
	SecretKeyFilePath  string `kong:"name='secret-key-file-path',help='Path to binary cache secret key',env='SECRET_KEY_FILE_PATH'"`
	MinioUrl           string `kong:"name='s3-url',help='s3 URL',default='http://localhost:9001',env='S3_URL'"`
	MinioAcccessKey    string `kong:"name='s3-access-key',help='s3 access key',env='S3_ACCESS_KEY'"`
	MinioAcccessSecret string `kong:"name='s3-access-secret',help='s3 access secret',env='S3_ACCESS_SECRET'"`
}

type Config struct {
	ListenAddr     string
	SecretKey      string // Don't use for binary cache. Used just for config loading. Use BinaryCacheCfg
	SecretKeyPath  string // Don't use for binary cache. Used just for config loading. Use BinaryCacheCfg
	MinioCfg       MinioCfg
	BinaryCacheCfg BinaryCacheCfg
}

type BinaryCacheCfg struct {
	PrivateKey signature.SecretKey
	PublicKey  signature.PublicKey
}

type MinioCfg struct {
	Url           url.URL
	AcccessKey    string
	AcccessSecret string
}

func LoadConfiguration(args []string) (Config, error) {
	var cli cli
	// TODO king is overkill
	parser, err := kong.New(&cli)
	if err != nil {
		return Config{}, err
	}
	_, err = parser.Parse(args[1:])
	if err != nil {
		return Config{}, err
	}

	minioUrl, err := url.Parse(cli.MinioUrl)
	if err != nil {
		return Config{}, err
	}

	cfgFile, err := os.Open(cli.ConfigPath)
	if err != nil {
		return Config{}, err
	}

	var cfg Config
	err = yaml.NewDecoder(cfgFile).Decode(&cfg)
	if err != nil {
		return Config{}, err
	}

	defaultedConfig := Config{
		ListenAddr:    defaultLeft(cli.ListenAddr, cfg.ListenAddr),
		SecretKey:     defaultLeft(cli.SecretKey, cfg.SecretKey),
		SecretKeyPath: defaultLeft(cli.SecretKeyFilePath, cfg.SecretKeyPath),
		MinioCfg: MinioCfg{
			Url:           defaultLeft(*minioUrl, cfg.MinioCfg.Url),
			AcccessKey:    defaultLeft(cli.MinioAcccessKey, cfg.MinioCfg.AcccessKey),
			AcccessSecret: defaultLeft(cli.MinioAcccessSecret, cfg.MinioCfg.AcccessSecret),
		},
	}

	err = resolveSecretKey(&defaultedConfig)
	if err != nil {
		return Config{}, err
	}

	return defaultedConfig, nil
}

func defaultLeft[T comparable](left, right T) T {
	var zero T
	if left == zero {
		return right
	}

	return left
}

func resolveSecretKey(cfg *Config) error {
	if cfg.SecretKey == "" && cfg.SecretKeyPath == "" {
		return errors.New("One of secretKey or secretKeyPath has to be set")
	}

	secretKey := cfg.SecretKey
	if cfg.SecretKeyPath != "" {
		secret, err := os.ReadFile(cfg.SecretKeyPath)
		if err != nil {
			return err
		}
		secretKey = string(secret)
	}

	priv, err := signature.LoadSecretKey(secretKey)
	if err != nil {
		return err
	}

	cfg.BinaryCacheCfg = BinaryCacheCfg{
		PrivateKey: priv,
		PublicKey:  priv.ToPublicKey(),
	}

	return nil
}

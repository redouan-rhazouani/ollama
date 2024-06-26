package server

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

type ModelPath struct {
	ProtocolScheme string
	Registry       string
	Namespace      string
	Repository     string
	Tag            string
}

const (
	DefaultRegistry       = "registry.ollama.ai"
	DefaultNamespace      = "library"
	DefaultTag            = "latest"
	DefaultProtocolScheme = "https"
)

var (
	ErrInvalidImageFormat  = errors.New("invalid image format")
	ErrInvalidProtocol     = errors.New("invalid protocol scheme")
	ErrInsecureProtocol    = errors.New("insecure protocol http")
	ErrInvalidDigestFormat = errors.New("invalid digest format")
)

// blobDigestRegEx only accept actual sha256 digests
var blobDigestRegEx = regexp.MustCompile("^sha256[:-][0-9a-fA-F]{64}$")

func ParseModelPath(name string) ModelPath {
	mp := ModelPath{
		ProtocolScheme: DefaultProtocolScheme,
		Registry:       DefaultRegistry,
		Namespace:      DefaultNamespace,
		Repository:     "",
		Tag:            DefaultTag,
	}

	before, after, found := strings.Cut(name, "://")
	if found {
		mp.ProtocolScheme = before
		name = after
	}

	name = strings.ReplaceAll(name, string(os.PathSeparator), "/")
	parts := strings.Split(name, "/")
	switch len(parts) {
	case 3:
		mp.Registry = parts[0]
		mp.Namespace = parts[1]
		mp.Repository = parts[2]
	case 2:
		mp.Namespace = parts[0]
		mp.Repository = parts[1]
	case 1:
		mp.Repository = parts[0]
	}

	if repo, tag, found := strings.Cut(mp.Repository, ":"); found {
		mp.Repository = repo
		mp.Tag = tag
	}

	return mp
}

var errModelPathInvalid = errors.New("invalid model path")

func (mp ModelPath) Validate() error {
	if mp.Repository == "" {
		return fmt.Errorf("%w: model repository name is required", errModelPathInvalid)
	}

	if strings.Contains(mp.Tag, ":") {
		return fmt.Errorf("%w: ':' (colon) is not allowed in tag names", errModelPathInvalid)
	}

	return nil
}

func (mp ModelPath) GetNamespaceRepository() string {
	return fmt.Sprintf("%s/%s", mp.Namespace, mp.Repository)
}

func (mp ModelPath) GetFullTagname() string {
	return fmt.Sprintf("%s/%s/%s:%s", mp.Registry, mp.Namespace, mp.Repository, mp.Tag)
}

func (mp ModelPath) GetShortTagname() string {
	if mp.Registry == DefaultRegistry {
		if mp.Namespace == DefaultNamespace {
			return fmt.Sprintf("%s:%s", mp.Repository, mp.Tag)
		}
		return fmt.Sprintf("%s/%s:%s", mp.Namespace, mp.Repository, mp.Tag)
	}
	return fmt.Sprintf("%s/%s/%s:%s", mp.Registry, mp.Namespace, mp.Repository, mp.Tag)
}

// modelsDir returns the value of the OLLAMA_MODELS environment variable or the user's home directory if OLLAMA_MODELS is not set.
// The models directory is where Ollama stores its model files and manifests.
func modelsDir() (string, error) {
	if models, exists := os.LookupEnv("OLLAMA_MODELS"); exists {
		return models, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".ollama", "models"), nil
}

// GetManifestPath returns the path to the manifest file for the given model path, it is up to the caller to create the directory if it does not exist.
func (mp ModelPath) GetManifestPath() (string, error) {
	dir, err := modelsDir()
	if err != nil {
		return "", err
	}

	return filepath.Join(dir, "manifests", mp.Registry, mp.Namespace, mp.Repository, mp.Tag), nil
}

func (mp ModelPath) BaseURL() *url.URL {
	return &url.URL{
		Scheme: mp.ProtocolScheme,
		Host:   mp.Registry,
	}
}

func GetManifestPath() (string, error) {
	dir, err := modelsDir()
	if err != nil {
		return "", err
	}

	path := filepath.Join(dir, "manifests")
	if err := os.MkdirAll(path, 0o755); err != nil {
		return "", err
	}

	return path, nil
}

// GetBlobsPath returns the path to a file in the model directory given its SHA256 digest
// It returns ErrInvalidDigestFormat if the digest is not valid.
func GetBlobsPath(digest string) (path string, err error) {
	dir, err := modelsDir()
	if err != nil {
		return "", err
	}
	dir = filepath.Join(dir, "blobs")
	if digest != "" {
		if !blobDigestRegEx.MatchString(digest) {
			return "", ErrInvalidDigestFormat
		}
		digest = strings.ReplaceAll(digest, ":", "-")
		path = filepath.Join(dir, digest)
	} else {
		path = dir
	}

	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}

	return path, nil
}

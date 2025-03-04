package arbiter

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials/stscreds"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/packrat386/s3fs"
)

// BackendList is a collection of backends for arbiter to look in for terraform
// state, where a backend consists of a name and a filesystem to browse for
// terraform state files.
type BackendList interface {
	// AddState adds a named state FS
	AddState(name string, state fs.FS)

	// GetState returns the state FS with the given name. If none exists, it returns nil
	GetState(name string) fs.FS

	// Names returns the list of the names of backends
	Names() []string
}

type backendList struct {
	backends []*backend
}

type backend struct {
	name  string
	state fs.FS
}

// NewBackendList returns an empty BackendList. It returns Names() in the order added
func NewBackendList() BackendList {
	return &backendList{
		backends: []*backend{},
	}
}

func (b *backendList) AddState(name string, state fs.FS) {
	b.backends = append(
		b.backends,
		&backend{
			name:  name,
			state: state,
		},
	)
}

func (b *backendList) GetState(name string) fs.FS {
	for _, backend := range b.backends {
		if backend.name == name {
			return backend.state
		}
	}

	return nil
}

func (b *backendList) Names() []string {
	names := []string{}
	for _, backend := range b.backends {
		names = append(names, backend.name)
	}

	return names
}

type backendConfig struct {
	Name           string          `json:"name"`
	Type           string          `json:"type"`
	ConnectionInfo json.RawMessage `json:"connection_info"`
}

// BackendListFromJSON is a convenience method to initialize a BackendList from
// a JSON source. See README.md for the expected JSON format.
func BackendListFromJSON(reader io.Reader) (BackendList, error) {
	var configs []backendConfig
	err := json.NewDecoder(reader).Decode(&configs)
	if err != nil {
		return nil, fmt.Errorf("could not decode JSON: %w", err)
	}

	backends := NewBackendList()

	for _, conf := range configs {
		state, err := initBackendState(conf)
		if err != nil {
			return nil, fmt.Errorf("could not init backend state for backend '%s': %w", conf.Name, err)
		}

		backends.AddState(conf.Name, state)
	}

	return backends, nil
}

func initBackendState(conf backendConfig) (fs.FS, error) {
	switch conf.Type {
	case "s3":
		var info s3ConnectionInfo
		err := json.Unmarshal(conf.ConnectionInfo, &info)
		if err != nil {
			return nil, fmt.Errorf("could not parse connection info: %w", err)
		}

		return initS3State(info)
	default:
		return nil, fmt.Errorf("backend type '%s' not yet implemented: ", conf.Type)
	}

}

type s3ConnectionInfo struct {
	BucketName string `json:"bucket_name"`
	RoleARN    string `json:"role_arn"`
}

func initS3State(info s3ConnectionInfo) (fs.FS, error) {
	cfg, err := config.LoadDefaultConfig(context.Background())
	if err != nil {
		return nil, fmt.Errorf("could not init aws config: %w", err)
	}

	if info.RoleARN != "" {
		stssvc := sts.NewFromConfig(cfg)
		creds := stscreds.NewAssumeRoleProvider(stssvc, info.RoleARN)
		cfg.Credentials = aws.NewCredentialsCache(creds)
	}

	client := s3.NewFromConfig(cfg)

	return s3fs.NewS3FSV2(client, info.BucketName), nil
}

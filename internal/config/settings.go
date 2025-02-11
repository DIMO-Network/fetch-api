// Package config holds application configuration settings.
package config

import (
	"github.com/DIMO-Network/clickhouse-infra/pkg/connect/config"
	"github.com/ethereum/go-ethereum/common"
)

// Settings contains the application config.
type Settings struct {
	Port                      int             `yaml:"PORT"`
	MonPort                   int             `yaml:"MON_PORT"`
	GRPCPort                  int             `yaml:"GRPC_PORT"`
	TokenExchangeJWTKeySetURL string          `yaml:"TOKEN_EXCHANGE_JWK_KEY_SET_URL"`
	TokenExchangeIssuer       string          `yaml:"TOKEN_EXCHANGE_ISSUER_URL"`
	VehicleNFTAddress         common.Address  `yaml:"VEHICLE_NFT_ADDRESS"`
	ChainID                   string          `yaml:"CHAIN_ID"`
	CloudEventBucket          string          `yaml:"CLOUDEVENT_BUCKET"`
	EphemeralBucket           string          `yaml:"EPHEMERAL_BUCKET"`
	VCBucket                  string          `yaml:"VC_BUCKET"`
	S3AWSRegion               string          `yaml:"S3_AWS_REGION"`
	S3AWSAccessKeyID          string          `yaml:"S3_AWS_ACCESS_KEY_ID"`
	S3AWSSecretAccessKey      string          `yaml:"S3_AWS_SECRET_ACCESS_KEY"`
	Clickhouse                config.Settings `yaml:",inline"`
}
